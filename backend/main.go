// Minimal NYC Subway departures backend with extra logging
// - Endpoints:
//   GET /api/stops
//   GET /api/departures/nearest?lat=<lat>&lon=<lon>
//   GET /api/departures/by-name?name=<stop name>
//
// Build/run:
//   go mod init nyc-subway
//   go get github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs
//   go get google.golang.org/protobuf/proto
//   go run backend/main.go
//
// Data sources used at runtime (no API keys):
// - Real-time GTFS-RT feeds (9 endpoints): https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs[-suffix]
//   e.g., .../nyct%2Fgtfs, -ace, -bdfm, -g, -jz, -l, -nqrw, -7, -si
// - Stations list (with GTFS Stop ID, lat/lon): https://data.ny.gov/api/views/39hk-dx4f/rows.csv?accessType=DOWNLOAD
// - Walking time: OSRM demo: https://router.project-osrm.org/route/v1/foot/{lon1},{lat1};{lon2},{lat2}?overview=false
//
// NOTES:
// - This is intentionally minimal. It downloads station metadata on startup.
// - It fetches every GTFS-RT feed on each request (simple but not optimized).
// - It returns an error when the requested coordinate is clearly outside the NYC area.

package main

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bluele/gcache"
	gtfs_realtime "nyc-subway/gtfs_realtime"
	"google.golang.org/protobuf/proto"
)

type Station struct {
	StopID string   `json:"gtfs_stop_id"`
	Name   string   `json:"stop_name"`
	Lat    float64  `json:"lat"`
	Lon    float64  `json:"lon"`
	Routes []string `json:"routes,omitempty"` // Routes serving this station (e.g., ["N", "W"])
}

type NearestResponse struct {
	Station    Station     `json:"station"`
	Walking    *WalkResult `json:"walking,omitempty"`
	Departures []Departure `json:"departures"`
}

type Departure struct {
	RouteID    string `json:"route_id"`
	StopID     string `json:"stop_id"`
	Direction  string `json:"direction"` // last letter of stop_id (N/S/E/W) if present
	UnixTime   int64  `json:"unix_time"`
	ETASeconds int64  `json:"eta_seconds"`
	TripID     string `json:"trip_id,omitempty"`
	HeadSign   string `json:"headsign,omitempty"`
}

type WalkResult struct {
	Seconds  float64 `json:"seconds"`
	Distance float64 `json:"meters"`
}

type Trip struct {
	RouteID     string
	TripID      string
	ServiceID   string
	TripHeadsign string
	DirectionID string
}


var (
	stations   []Station
	trips      []Trip
	httpClient = &http.Client{Timeout: 12 * time.Second}
	walkCache  gcache.Cache
	// NYC area bounding box (coarse)
	minLat, maxLat = 40.3, 41.1
	minLon, maxLon = -74.5, -73.3

	// Feeds: base + ACE, BDFM, G, JZ, L, NQRW, 7, SI
	feedURLs = []string{
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace",
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-bdfm",
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-g",
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-jz",
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-l",
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-nqrw",
		"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-si",
	}

	// Mapping of routes to their feed URLs
	routeToFeed = map[string]string{
		// Base feed (numbered lines + Grand Central Shuttle)
		"1": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"2": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"3": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"4": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"5": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"6": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"7": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
		"GS": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs", // Grand Central Shuttle
		// ACE feed
		"A": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace",
		"C": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace",
		"E": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace",
		"H": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace", // Rockaway Park Shuttle
		"FS": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace", // Franklin Avenue Shuttle
		// BDFM feed
		"B": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-bdfm",
		"D": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-bdfm",
		"F": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-bdfm",
		"M": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-bdfm",
		// G feed
		"G": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-g",
		// JZ feed
		"J": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-jz",
		"Z": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-jz",
		// L feed
		"L": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-l",
		// NQRW feed
		"N": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-nqrw",
		"Q": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-nqrw",
		"R": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-nqrw",
		"W": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-nqrw",
		// Staten Island Railway
		"SI": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-si",
		"SIR": "https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-si",
	}

	// Default stations CSV from NY Open Data (no token needed)
	stationsCSV = "https://data.ny.gov/api/views/39hk-dx4f/rows.csv?accessType=DOWNLOAD"
	// MTA Stations.csv with route information
	mtaStationsCSV = "http://web.mta.info/developers/data/nyct/subway/Stations.csv"
	gtfsZipURL = "http://web.mta.info/developers/data/nyct/subway/google_transit.zip"
)

func main() {
	// Enable line numbers in logging with microsecond granularity
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	
	// Initialize walking time cache: 24h TTL, max 10,000 entries with LRU eviction
	walkCache = gcache.New(10000).
		LRU().
		Expiration(24 * time.Hour).
		Build()
	
	if v := os.Getenv("STATIONS_CSV"); v != "" {
		stationsCSV = v
	}
	if err := loadStations(context.Background(), stationsCSV); err != nil {
		log.Panic(err)
	}

	// Log full list of stations as requested
	log.Printf("Loaded %d stations", len(stations))

	if err := loadTrips(context.Background(), gtfsZipURL); err != nil {
		log.Printf("Warning: failed to load GTFS trips data: %v", err)
	} else {
		log.Printf("Loaded %d trips", len(trips))
	}


	mux := http.NewServeMux()
	mux.HandleFunc("/api/stops", withCORS(handleStops))
	mux.HandleFunc("/api/departures/nearest", withCORS(handleNearest))
	mux.HandleFunc("/api/departures/by-name", withCORS(handleByName))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Panic(err)
	}
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h(w, r)
	}
}


func handleStops(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("Request received: %s %s", r.Method, r.URL.String())
	writeJSON(w, stations)
	log.Printf("Request completed in %.2f ms", float64(time.Since(start).Microseconds())/1000.0)
}

func handleNearest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("Request received: %s %s", r.Method, r.URL.String())
	lat, lon, err := parseLatLon(r)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	if outsideNYC(lat, lon) {
		httpError(w, http.StatusBadRequest, "location outside NYC area")
		return
	}

	nearest := nearestStation(lat, lon)
	log.Printf("Nearest station to (%.6f, %.6f) is %s [%s] at (%.6f, %.6f)",
		lat, lon, nearest.Name, nearest.StopID, nearest.Lat, nearest.Lon)

	deps, err := departuresForStation(nearest)
	if err != nil {
		httpError(w, http.StatusBadGateway, err.Error())
		return
	}

	walk, werr := walkingTime(lat, lon, nearest.Lat, nearest.Lon) // best-effort
	if werr != nil {
		log.Printf("walkingTime error: %v", werr)
	}
	resp := NearestResponse{Station: nearest, Walking: walk, Departures: deps}
	writeJSON(w, resp)
	log.Printf("Request completed in %.2f ms", float64(time.Since(start).Microseconds())/1000.0)
}

func handleByName(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("Request received: %s %s", r.Method, r.URL.String())
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		httpError(w, http.StatusBadRequest, "missing name")
		return
	}
	var matched []Station
	lname := strings.ToLower(name)
	for _, s := range stations {
		if strings.Contains(strings.ToLower(s.Name), lname) {
			matched = append(matched, s)
		}
	}
	if len(matched) == 0 {
		httpError(w, http.StatusNotFound, "no station matched by name")
		return
	}
	log.Printf("handleByName matched %d station records for name %q", len(matched), name)
	deps, err := departuresForStation(matched[0])
	if err != nil {
		httpError(w, http.StatusBadGateway, err.Error())
		return
	}
	resp := NearestResponse{Station: matched[0], Departures: deps}
	writeJSON(w, resp)
	log.Printf("Request completed in %.2f ms", float64(time.Since(start).Microseconds())/1000.0)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func outsideNYC(lat, lon float64) bool {
	return lat < minLat || lat > maxLat || lon < minLon || lon > maxLon
}

func nearestStation(lat, lon float64) Station {
	best := Station{}
	bestD := math.MaxFloat64
	for _, s := range stations {
		d := haversine(lat, lon, s.Lat, s.Lon)
		if d < bestD {
			bestD = d
			best = s
		}
	}
	return best
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0
	φ1 := lat1 * math.Pi / 180.0
	φ2 := lat2 * math.Pi / 180.0
	dφ := (lat2 - lat1) * math.Pi / 180.0
	dλ := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// quantizeCoord rounds coordinates to 4 decimal places (~11m precision) for cache key generation
func quantizeCoord(coord float64) float64 {
	return math.Round(coord*10000) / 10000
}

// makeCacheKey creates a cache key for walking time results
func makeCacheKey(fromLat, fromLon, toLat, toLon float64) string {
	// Quantize user coordinates (from), keep station coordinates (to) precise
	qFromLat := quantizeCoord(fromLat)
	qFromLon := quantizeCoord(fromLon)
	return fmt.Sprintf("%.4f,%.4f,%.6f,%.6f", qFromLat, qFromLon, toLat, toLon)
}

func walkingTime(fromLat, fromLon, toLat, toLon float64) (*WalkResult, error) {
	// Check cache first
	cacheKey := makeCacheKey(fromLat, fromLon, toLat, toLon)
	if cached, err := walkCache.Get(cacheKey); err == nil {
		if result, ok := cached.(*WalkResult); ok {
			log.Printf("walkingTime cache hit for key %s", cacheKey)
			return result, nil
		}
	}
	
	url := fmt.Sprintf(
		"https://router.project-osrm.org/route/v1/foot/%f,%f;%f,%f?overview=false",
		fromLon, fromLat, toLon, toLat,
	)
	log.Printf("walkingTime request: %s", url)
	req, _ := http.NewRequest("GET", url, nil)
	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("walkingTime HTTP error after %s: %v", time.Since(start), err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("walkingTime non-200 status=%d body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("osrm status %d", resp.StatusCode)
	}
	type route struct {
		Duration float64 `json:"duration"`
		Distance float64 `json:"distance"`
	}
	var obj struct {
		Routes []route `json:"routes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		log.Printf("walkingTime decode error: %v", err)
		return nil, err
	}
	if len(obj.Routes) == 0 {
		log.Printf("walkingTime response had zero routes")
		return nil, errors.New("no route")
	}
	
	result := &WalkResult{Seconds: obj.Routes[0].Duration, Distance: obj.Routes[0].Distance}
	
	// Store in cache
	walkCache.Set(cacheKey, result)
	log.Printf("walkingTime OK: duration=%.1fs distance=%.1fm (elapsed %s) [cached: %s]", 
		obj.Routes[0].Duration, obj.Routes[0].Distance, time.Since(start), cacheKey)
	return result, nil
}

func departuresForStation(s Station) ([]Departure, error) {
	// Build sets for exact stop IDs and their "base" IDs (without trailing direction letter).
	stopExact := map[string]struct{}{}
	stopBase := map[string]struct{}{}
	stopExact[s.StopID] = struct{}{}
	stopBase[baseStopID(s.StopID)] = struct{}{}

	now := time.Now().Unix()
	deps := make([]Departure, 0, 64)

	// Determine which feeds to fetch based on station's routes
	feeds := getFeedsForStation(s)
	log.Printf("Station %s serves routes %v, fetching %d feed(s)", s.Name, s.Routes, len(feeds))

	for _, u := range feeds {
		feed, err := fetchGTFS(u)
		if err != nil {
			log.Printf("fetchGTFS error for %s: %v", u, err)
			continue
		}
		for _, ent := range feed.GetEntity() {
			tu := ent.GetTripUpdate()
			if tu == nil {
				continue
			}
			routeID := ""
			tripID := ""
			if td := tu.GetTrip(); td != nil {
				routeID = td.GetRouteId()
				tripID = td.GetTripId()
			}

			// IMPORTANT: translate and append within the same loop that iterates stop time updates.
			for _, stu := range tu.GetStopTimeUpdate() {
				stopID := stu.GetStopId()

				// Match against exact stop ID OR base stop ID (handles N/S/E/W suffix in GTFS-RT).
				if _, ok := stopExact[stopID]; !ok {
					if _, ok2 := stopBase[baseStopID(stopID)]; !ok2 {
						continue
					}
				}

				var t int64
				if dep := stu.GetDeparture(); dep != nil {
					t = dep.GetTime()
				}
				if t == 0 {
					if arr := stu.GetArrival(); arr != nil {
						t = arr.GetTime()
					}
				}
				if t == 0 || t < now {
					continue
				}


				dir := ""
				if n := len(stopID); n > 0 {
					last := stopID[n-1]
					if last == 'N' || last == 'S' || last == 'E' || last == 'W' {
						dir = string(last)
					}
				}
				etaSec := t - now

				deps = append(deps, Departure{
					RouteID:    routeID,
					StopID:     stopID,
					Direction:  dir,
					UnixTime:   t,
					ETASeconds: etaSec,
					TripID:     tripID,
					HeadSign:   "",
				})
			}
		}
	}

	sort.Slice(deps, func(i, j int) bool { return deps[i].UnixTime < deps[j].UnixTime })
	
	// Limit to 2 departures per route and direction
	deps = limitDeparturesByRouteAndDirection(deps)
	
	// Fill in headsigns for the filtered departures
	for i := range deps {
		deps[i].HeadSign = lookupHeadsign(deps[i].TripID)
	}
	
	log.Printf("departuresForStation produced %d departures (after filtering)", len(deps))
	return deps, nil
}

// getFeedsForStation returns the feed URLs needed for a station based on its routes
func getFeedsForStation(s Station) []string {
	// If no route information, fall back to fetching all feeds
	if len(s.Routes) == 0 {
		log.Printf("No route information for station %s, using all feeds", s.Name)
		return feedURLs
	}
	
	// Use a map to deduplicate feed URLs
	feedSet := make(map[string]struct{})
	
	for _, route := range s.Routes {
		if feedURL, ok := routeToFeed[route]; ok {
			feedSet[feedURL] = struct{}{}
		} else {
			// Handle special cases and variants
			// Express variants (e.g., 6X -> 6)
			if len(route) > 1 && route[len(route)-1] == 'X' {
				baseRoute := route[:len(route)-1]
				if feedURL, ok := routeToFeed[baseRoute]; ok {
					feedSet[feedURL] = struct{}{}
					continue
				}
			}
			// S could be any shuttle, check common ones
			if route == "S" {
				// Grand Central Shuttle is in base feed
				feedSet["https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs"] = struct{}{}
				// Franklin Avenue Shuttle is in ACE feed
				feedSet["https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace"] = struct{}{}
			} else {
				log.Printf("Unknown route %s for station %s", route, s.Name)
			}
		}
	}
	
	// Convert set to slice
	var feeds []string
	for feed := range feedSet {
		feeds = append(feeds, feed)
	}
	
	// If no feeds matched, fall back to all feeds
	if len(feeds) == 0 {
		log.Printf("No feeds matched for station %s routes %v, using all feeds", s.Name, s.Routes)
		return feedURLs
	}
	
	return feeds
}

// limitDeparturesByRouteAndDirection limits departures to at most 2 per route+direction combination
func limitDeparturesByRouteAndDirection(deps []Departure) []Departure {
	// Group departures by route+direction
	counts := make(map[string]int)
	result := []Departure{}
	
	for _, dep := range deps {
		key := dep.RouteID + "_" + dep.Direction
		if counts[key] < 2 {
			result = append(result, dep)
			counts[key]++
		}
	}
	
	return result
}




func fetchGTFS(url string) (*gtfs_realtime.FeedMessage, error) {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var feed gtfs_realtime.FeedMessage
	if err := proto.Unmarshal(b, &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

func loadStations(ctx context.Context, csvURL string) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", csvURL, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download stations: %w", err)
	}
	defer resp.Body.Close()
	r := csv.NewReader(resp.Body)
	r.FieldsPerRecord = -1

	// NOTE: column keys use "gtfs", not "gtsf".
	need := []string{"gtfsstopid", "stopname", "gtfslatitude", "gtfslongitude"}
	idx, err := parseCSVHeaders(r, need, "stations")
	if err != nil {
		return err
	}

	var out []Station
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read stations row: %w", err)
		}
		stopID := row[idx["gtfsstopid"]]
		name := row[idx["stopname"]]
		lat, _ := strconv.ParseFloat(row[idx["gtfslatitude"]], 64)
		lon, _ := strconv.ParseFloat(row[idx["gtfslongitude"]], 64)
		if stopID == "" || lat == 0 || lon == 0 {
			continue
		}
		out = append(out, Station{StopID: stopID, Name: name, Lat: lat, Lon: lon})
	}
	stations = out
	
	// Load route mappings from MTA Stations.csv
	if err := loadRouteMapping(ctx); err != nil {
		log.Printf("Warning: failed to load route mappings: %v", err)
		// Continue without route optimization if loading fails
	}
	
	return nil
}

// loadRouteMapping loads the MTA Stations.csv to extract route information for each stop
func loadRouteMapping(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", mtaStationsCSV, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download MTA stations: %w", err)
	}
	defer resp.Body.Close()
	r := csv.NewReader(resp.Body)
	r.FieldsPerRecord = -1

	// MTA Stations.csv uses different column names
	need := []string{"gtfsstopid", "daytimeroutes"}
	idx, err := parseCSVHeaders(r, need, "mta-stations")
	if err != nil {
		return err
	}
	
	// Create a map for quick lookup
	routeMap := make(map[string][]string)
	
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read MTA stations row: %w", err)
		}
		
		stopID := row[idx["gtfsstopid"]]
		routesStr := row[idx["daytimeroutes"]]
		
		if stopID == "" || routesStr == "" {
			continue
		}
		
		// Parse routes (e.g., "N W" or "A C E")
		routes := strings.Fields(routesStr)
		routeMap[stopID] = routes
	}
	
	// Update stations with route information
	for i := range stations {
		if routes, ok := routeMap[stations[i].StopID]; ok {
			stations[i].Routes = routes
		}
	}
	
	log.Printf("Loaded route mappings for %d stops", len(routeMap))
	return nil
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "/", "", ".", "")
	return replacer.Replace(s)
}

func loadTrips(ctx context.Context, zipURL string) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", zipURL, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download GTFS zip: %w", err)
	}
	defer resp.Body.Close()

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read GTFS zip: %w", err)
	}

	zipReader, err := zip.NewReader(strings.NewReader(string(zipData)), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("open GTFS zip: %w", err)
	}

	var tripsFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == "trips.txt" {
			tripsFile = f
			break
		}
	}
	if tripsFile == nil {
		return fmt.Errorf("trips.txt not found in GTFS zip")
	}

	rc, err := tripsFile.Open()
	if err != nil {
		return fmt.Errorf("open trips.txt: %w", err)
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	r.FieldsPerRecord = -1

	need := []string{"route_id", "trip_id", "service_id", "trip_headsign", "direction_id"}
	idx, err := parseCSVHeaders(r, need, "trips")
	if err != nil {
		return err
	}

	var out []Trip
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read trips row: %w", err)
		}

		trip := Trip{
			RouteID:      row[idx["route_id"]],
			TripID:       row[idx["trip_id"]],
			ServiceID:    row[idx["service_id"]],
			TripHeadsign: row[idx["trip_headsign"]],
			DirectionID:  row[idx["direction_id"]],
		}
		out = append(out, trip)
	}

	trips = out
	log.Printf("Loaded %d trips from GTFS data", len(trips))
	return nil
}

func parseLatLon(r *http.Request) (float64, float64, error) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	if latStr == "" || lonStr == "" {
		return 0, 0, fmt.Errorf("missing lat or lon")
	}
	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("invalid lat or lon")
	}
	return lat, lon, nil
}

func baseStopID(id string) string {
	if id == "" {
		return id
	}
	last := id[len(id)-1]
	if (last >= 'A' && last <= 'Z') || (last >= 'a' && last <= 'z') {
		return id[:len(id)-1]
	}
	return id
}

func parseCSVHeaders(r *csv.Reader, needed []string, source string) (map[string]int, error) {
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read %s header: %w", source, err)
	}
	log.Printf("%s csv header (raw): %q", source, headers)
	
	idx := map[string]int{}
	for i, h := range headers {
		var key string
		if source == "trips" {
			key = strings.ToLower(strings.TrimSpace(h))
		} else {
			key = normalizeHeader(h)
		}
		idx[key] = i
	}
	
	var normKeys []string
	for k := range idx {
		normKeys = append(normKeys, k)
	}
	sort.Strings(normKeys)
	log.Printf("%s csv header (normalized): %s", source, strings.Join(normKeys, ", "))
	
	for _, k := range needed {
		if _, ok := idx[k]; !ok {
			return nil, fmt.Errorf("%s csv missing column '%s'", source, k)
		}
	}
	return idx, nil
}

func lookupHeadsign(tripID string) string {
	if tripID == "" || len(trips) == 0 {
		return ""
	}

	// Get current day of week
	now := time.Now()
	dayOfWeek := now.Weekday()
	var service string
	switch dayOfWeek {
	case time.Sunday:
		service = "Sunday"
	case time.Saturday:
		service = "Saturday"
	default:
		service = "Weekday"
	}

	// Find matching trips where tripID from GTFS-RT is a substring of trip_id from trips.txt
	var matches []Trip
	for _, trip := range trips {
		if strings.Contains(trip.TripID, tripID) {
			matches = append(matches, trip)
		}
	}

	if len(matches) == 0 {
		return ""
	}

	// If multiple matches, prefer the one matching today's service
	for _, match := range matches {
		if match.ServiceID == service {
			return match.TripHeadsign
		}
	}

	// If no service match, return first match
	return matches[0].TripHeadsign
}
