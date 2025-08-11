package main

import (
	"context"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gtfs_realtime "nyc-subway/gtfs_realtime"
	"google.golang.org/protobuf/proto"
)

func TestHaversine(t *testing.T) {
	// Times Square to Grand Central ~1.1km
	tsLat, tsLon := 40.7580, -73.9855
	gcLat, gcLon := 40.7527, -73.9772
	d := haversine(tsLat, tsLon, gcLat, gcLon)
	if d < 900 || d > 1500 {
		t.Fatalf("unexpected distance %.0f m", d)
	}
}

func TestNearestStation(t *testing.T) {
	// Inject a tiny station list
	stations = []Station{
		{StopID: "R14N", Name: "14 St - Union Sq", Lat: 40.7359, Lon: -73.9906},
		{StopID: "635S", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
		{StopID: "A32N", Name: "Times Sq - 42 St", Lat: 40.7553, Lon: -73.9877},
	}
	// Point near Grand Central
	s := nearestStation(40.7528, -73.9775)
	if s.Name != "Grand Central - 42 St" {
		t.Fatalf("nearest wrong: got %s", s.Name)
	}
}

func TestOutsideNYC(t *testing.T) {
	if !outsideNYC(34.0522, -118.2437) {
		t.Fatal("LA should be outside NYC")
	}
	if outsideNYC(40.76, -73.98) {
		t.Fatal("Midtown should be inside NYC")
	}
}

func TestLimitDeparturesByRouteAndDirection(t *testing.T) {
	deps := []Departure{
		// Route 6, North direction - 4 departures
		{RouteID: "6", Direction: "N", UnixTime: 100, ETASeconds: 60},
		{RouteID: "6", Direction: "N", UnixTime: 200, ETASeconds: 120},
		{RouteID: "6", Direction: "N", UnixTime: 300, ETASeconds: 180},
		{RouteID: "6", Direction: "N", UnixTime: 400, ETASeconds: 240},
		// Route 6, South direction - 3 departures
		{RouteID: "6", Direction: "S", UnixTime: 150, ETASeconds: 90},
		{RouteID: "6", Direction: "S", UnixTime: 250, ETASeconds: 150},
		{RouteID: "6", Direction: "S", UnixTime: 350, ETASeconds: 210},
		// Route Q, North direction - 1 departure
		{RouteID: "Q", Direction: "N", UnixTime: 175, ETASeconds: 105},
		// Route Q, no direction - 3 departures
		{RouteID: "Q", Direction: "", UnixTime: 125, ETASeconds: 75},
		{RouteID: "Q", Direction: "", UnixTime: 225, ETASeconds: 135},
		{RouteID: "Q", Direction: "", UnixTime: 325, ETASeconds: 195},
	}

	limited := limitDeparturesByRouteAndDirection(deps)

	// Check total count: should be 2*2 (route 6) + 1 (Q North) + 2 (Q no direction) = 7
	if len(limited) != 7 {
		t.Fatalf("expected 7 departures, got %d", len(limited))
	}

	// Count by route and direction
	counts := make(map[string]int)
	for _, d := range limited {
		key := d.RouteID + "_" + d.Direction
		counts[key]++
	}

	// Verify counts
	if counts["6_N"] != 2 {
		t.Errorf("expected 2 departures for route 6 North, got %d", counts["6_N"])
	}
	if counts["6_S"] != 2 {
		t.Errorf("expected 2 departures for route 6 South, got %d", counts["6_S"])
	}
	if counts["Q_N"] != 1 {
		t.Errorf("expected 1 departure for route Q North, got %d", counts["Q_N"])
	}
	if counts["Q_"] != 2 {
		t.Errorf("expected 2 departures for route Q (no direction), got %d", counts["Q_"])
	}

	// Verify we kept the earliest departures
	for _, d := range limited {
		if d.RouteID == "6" && d.Direction == "N" {
			if d.UnixTime != 100 && d.UnixTime != 200 {
				t.Errorf("route 6 North should have times 100 and 200, got %d", d.UnixTime)
			}
		}
	}
}

// Test to verify the departure limiting logic works end-to-end
func TestDepartureGroupingIntegration(t *testing.T) {
	// Create test departures with multiple routes and directions
	now := time.Now().Unix()
	deps := []Departure{
		// Route 6 - multiple departures in each direction
		{RouteID: "6", StopID: "635N", Direction: "N", UnixTime: now + 60, ETASeconds: 60},
		{RouteID: "6", StopID: "635N", Direction: "N", UnixTime: now + 180, ETASeconds: 180},
		{RouteID: "6", StopID: "635N", Direction: "N", UnixTime: now + 300, ETASeconds: 300},
		{RouteID: "6", StopID: "635N", Direction: "N", UnixTime: now + 420, ETASeconds: 420},
		
		{RouteID: "6", StopID: "635S", Direction: "S", UnixTime: now + 120, ETASeconds: 120},
		{RouteID: "6", StopID: "635S", Direction: "S", UnixTime: now + 240, ETASeconds: 240},
		{RouteID: "6", StopID: "635S", Direction: "S", UnixTime: now + 360, ETASeconds: 360},
		
		// Route Q - mixed directions
		{RouteID: "Q", StopID: "Q05N", Direction: "N", UnixTime: now + 90, ETASeconds: 90},
		{RouteID: "Q", StopID: "Q05N", Direction: "N", UnixTime: now + 210, ETASeconds: 210},
		{RouteID: "Q", StopID: "Q05N", Direction: "N", UnixTime: now + 330, ETASeconds: 330},
		
		{RouteID: "Q", StopID: "Q05S", Direction: "S", UnixTime: now + 150, ETASeconds: 150},
		
		// Route 7 - no direction specified
		{RouteID: "7", StopID: "701", Direction: "", UnixTime: now + 100, ETASeconds: 100},
		{RouteID: "7", StopID: "701", Direction: "", UnixTime: now + 200, ETASeconds: 200},
		{RouteID: "7", StopID: "701", Direction: "", UnixTime: now + 300, ETASeconds: 300},
	}

	// Apply the limiting function
	limited := limitDeparturesByRouteAndDirection(deps)

	// Verify we have the right number of departures
	expectedTotal := 2 + 2 + 2 + 1 + 2 // 6N + 6S + QN + QS + 7
	if len(limited) != expectedTotal {
		t.Fatalf("expected %d departures after limiting, got %d", expectedTotal, len(limited))
	}

	// Count departures by route and direction
	counts := make(map[string]int)
	for _, d := range limited {
		key := d.RouteID + "_" + d.Direction
		counts[key]++
	}

	// Verify each group has at most 2 departures
	for key, count := range counts {
		if count > 2 {
			t.Errorf("route/direction %s has %d departures, expected max 2", key, count)
		}
	}

	// Verify specific counts
	expectedCounts := map[string]int{
		"6_N": 2,
		"6_S": 2,
		"Q_N": 2,
		"Q_S": 1,
		"7_":  2,
	}
	
	for key, expected := range expectedCounts {
		if counts[key] != expected {
			t.Errorf("route/direction %s: expected %d departures, got %d", key, expected, counts[key])
		}
	}

	// Verify we kept the earliest departures (they should be sorted)
	for i := 1; i < len(limited); i++ {
		prev := limited[i-1]
		curr := limited[i]
		if prev.RouteID == curr.RouteID && prev.Direction == curr.Direction {
			if prev.UnixTime > curr.UnixTime {
				t.Errorf("departures not in chronological order for route %s direction %s", 
					curr.RouteID, curr.Direction)
			}
		}
	}
}

// Test normalizeHeader function
func TestNormalizeHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"GTFS Stop ID", "gtfsstopid"},
		{"Stop_Name", "stopname"},
		{"GTFS-Latitude", "gtfslatitude"},
		{"GTFS/Longitude", "gtfslongitude"},
		{"  Station.Name  ", "stationname"},
		{"mixedCASE_with-symbols/dots.", "mixedcasewithsymbolsdots"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeHeader(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeHeader(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test loadStations function
func TestLoadStations(t *testing.T) {
	// Create a test CSV server
	csvData := `"GTFS Stop ID","Stop Name","GTFS Latitude","GTFS Longitude"
"123N","Test Station 1","40.7580","-73.9855"
"456S","Test Station 2","40.7527","-73.9772"
"","Invalid Station","",""
"789E","Station with bad coords","invalid","invalid"`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvData))
	}))
	defer server.Close()

	// Clear existing stations
	originalStations := stations
	defer func() { stations = originalStations }()

	// Test successful load
	err := loadStations(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("loadStations failed: %v", err)
	}

	// Verify loaded stations
	if len(stations) != 2 {
		t.Errorf("expected 2 valid stations, got %d", len(stations))
	}

	// Verify station data
	expectedStations := []Station{
		{StopID: "123N", Name: "Test Station 1", Lat: 40.7580, Lon: -73.9855},
		{StopID: "456S", Name: "Test Station 2", Lat: 40.7527, Lon: -73.9772},
	}

	for i, expected := range expectedStations {
		if i >= len(stations) {
			break
		}
		if stations[i].StopID != expected.StopID {
			t.Errorf("station[%d].StopID = %s, want %s", i, stations[i].StopID, expected.StopID)
		}
	}
}

// Test loadStations error cases
func TestLoadStationsErrors(t *testing.T) {
	ctx := context.Background()

	// Test network error
	err := loadStations(ctx, "http://invalid-url-that-does-not-exist.local")
	if err == nil {
		t.Error("expected error for invalid URL")
	}

	// Test malformed CSV
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not,a,valid,csv\nwith mismatched columns"))
	}))
	defer server.Close()

	err = loadStations(ctx, server.URL)
	if err == nil {
		t.Error("expected error for malformed CSV")
	}

	// Test missing required columns
	serverMissingCols := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Wrong Column 1,Wrong Column 2\nvalue1,value2"))
	}))
	defer serverMissingCols.Close()

	err = loadStations(ctx, serverMissingCols.URL)
	if err == nil || !strings.Contains(err.Error(), "missing column") {
		t.Error("expected missing column error")
	}
}

// Test withCORS middleware
func TestWithCORS(t *testing.T) {
	handler := withCORS(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	// Check CORS header is set
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS header not set correctly")
	}

	// Check response body
	if w.Body.String() != "test response" {
		t.Error("handler not called correctly")
	}
}

// Test serveIndex function
func TestServeIndex(t *testing.T) {
	// This would need a test HTML file to exist
	// For now, test that it doesn't crash
	// req := httptest.NewRequest("GET", "/", nil)
	// w := httptest.NewRecorder()
	
	// Should handle missing file gracefully - commenting out since serveIndex is not defined
	// serveIndex(w, req)
	// We expect a 404 since frontend/index.html doesn't exist in test environment
	// if w.Code != http.StatusNotFound {
	//	t.Errorf("expected 404 for missing file, got %d", w.Code)
	// }
	
	// Placeholder test since serveIndex is not currently implemented
	t.Skip("serveIndex function not currently implemented")
}

// Test fetchGTFS error cases
func TestFetchGTFSErrors(t *testing.T) {
	// Test network error
	_, err := fetchGTFS("http://invalid-url-that-does-not-exist.local")
	if err == nil {
		t.Error("expected error for invalid URL")
	}

	// Test invalid protobuf response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not a valid protobuf"))
	}))
	defer server.Close()

	_, err = fetchGTFS(server.URL)
	if err == nil {
		t.Error("expected error for invalid protobuf")
	}
}

// Test departuresForStops with arrival-only times
func TestDeparturesForStopsArrivalOnly(t *testing.T) {
	// Mock GTFS feed with arrival times only (no departure times)
	version := "2.0"
	timestamp := uint64(time.Now().Unix())
	incrementality := gtfs_realtime.FeedHeader_FULL_DATASET
	
	mockFeed := &gtfs_realtime.FeedMessage{
		Header: &gtfs_realtime.FeedHeader{
			GtfsRealtimeVersion: &version,
			Timestamp:           &timestamp,
			Incrementality:      &incrementality,
		},
		Entity: []*gtfs_realtime.FeedEntity{
			{
				Id: proto.String("entity1"),
				TripUpdate: &gtfs_realtime.TripUpdate{
					Trip: &gtfs_realtime.TripDescriptor{
						RouteId: proto.String("6"),
						TripId:  proto.String("trip1"),
					},
					StopTimeUpdate: []*gtfs_realtime.TripUpdate_StopTimeUpdate{
						{
							StopId: proto.String("635N"),
							Arrival: &gtfs_realtime.TripUpdate_StopTimeEvent{
								Time: proto.Int64(time.Now().Unix() + 300),
							},
							// No Departure time
						},
					},
				},
			},
		},
	}

	// Mock the fetchGTFS function temporarily
	data, _ := proto.Marshal(mockFeed)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer server.Close()

	// Temporarily replace feedURLs
	originalURLs := feedURLs
	feedURLs = []string{server.URL}
	defer func() { feedURLs = originalURLs }()

	station := Station{StopID: "635N", Name: "Test", Lat: 40.75, Lon: -73.98}
	deps, err := departuresForStation(station)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deps) != 1 {
		t.Errorf("expected 1 departure with arrival-only time, got %d", len(deps))
	}
}

// Test parseLatLon helper function
func TestParseLatLon(t *testing.T) {
	tests := []struct {
		name        string
		lat         string
		lon         string
		expectError bool
		expectedLat float64
		expectedLon float64
	}{
		{
			name:        "valid coordinates",
			lat:         "40.7580",
			lon:         "-73.9855",
			expectError: false,
			expectedLat: 40.7580,
			expectedLon: -73.9855,
		},
		{
			name:        "missing lat",
			lat:         "",
			lon:         "-73.9855",
			expectError: true,
		},
		{
			name:        "missing lon",
			lat:         "40.7580",
			lon:         "",
			expectError: true,
		},
		{
			name:        "invalid lat",
			lat:         "invalid",
			lon:         "-73.9855",
			expectError: true,
		},
		{
			name:        "invalid lon",
			lat:         "40.7580",
			lon:         "invalid",
			expectError: true,
		},
		{
			name:        "both missing",
			lat:         "",
			lon:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?lat="+tt.lat+"&lon="+tt.lon, nil)
			lat, lon, err := parseLatLon(req)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if lat != tt.expectedLat {
					t.Errorf("expected lat %.4f, got %.4f", tt.expectedLat, lat)
				}
				if lon != tt.expectedLon {
					t.Errorf("expected lon %.4f, got %.4f", tt.expectedLon, lon)
				}
			}
		})
	}
}

// Test baseStopID helper function
func TestBaseStopID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"123N", "123"},
		{"456S", "456"},
		{"789E", "789"},
		{"101W", "101"},
		{"635", "635"},
		{"", ""},
		{"123n", "123"},
		{"456s", "456"},
		{"A12N", "A12"},
		{"R14S", "R14"},
		{"123X", "123"},
		{"4567", "4567"},
		{"A", ""},
		{"1", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := baseStopID(tt.input)
			if result != tt.expected {
				t.Errorf("baseStopID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test parseCSVHeaders helper function
func TestParseCSVHeaders(t *testing.T) {
	t.Run("valid headers for stations", func(t *testing.T) {
		csvData := `"GTFS Stop ID","Stop Name","GTFS Latitude","GTFS Longitude"
"123N","Test Station","40.7580","-73.9855"`
		reader := csv.NewReader(strings.NewReader(csvData))
		reader.FieldsPerRecord = -1
		
		needed := []string{"gtfsstopid", "stopname", "gtfslatitude", "gtfslongitude"}
		idx, err := parseCSVHeaders(reader, needed, "stations")
		
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		
		// Verify all needed columns are mapped
		for _, col := range needed {
			if _, ok := idx[col]; !ok {
				t.Errorf("column %s not found in index", col)
			}
		}
		
		// Verify correct mappings
		if idx["gtfsstopid"] != 0 {
			t.Errorf("expected gtfsstopid at index 0, got %d", idx["gtfsstopid"])
		}
		if idx["stopname"] != 1 {
			t.Errorf("expected stopname at index 1, got %d", idx["stopname"])
		}
	})
	
	t.Run("valid headers for trips", func(t *testing.T) {
		csvData := `route_id,trip_id,service_id,trip_headsign,direction_id
6,trip1,Weekday,Manhattan,0`
		reader := csv.NewReader(strings.NewReader(csvData))
		reader.FieldsPerRecord = -1
		
		// For trips, the function now preserves underscores (just toLowerCase + trim)
		needed := []string{"route_id", "trip_id", "service_id", "trip_headsign", "direction_id"}
		idx, err := parseCSVHeaders(reader, needed, "trips")
		
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		
		// Verify all needed columns are mapped
		for _, col := range needed {
			if _, ok := idx[col]; !ok {
				t.Errorf("column %s not found in index", col)
			}
		}
	})
	
	t.Run("missing required column", func(t *testing.T) {
		csvData := `"Wrong Column 1","Wrong Column 2"
"value1","value2"`
		reader := csv.NewReader(strings.NewReader(csvData))
		reader.FieldsPerRecord = -1
		
		needed := []string{"gtfsstopid", "stopname", "gtfslatitude", "gtfslongitude"}
		_, err := parseCSVHeaders(reader, needed, "stations")
		
		if err == nil {
			t.Error("expected error for missing required column")
		}
		if !strings.Contains(err.Error(), "missing column") {
			t.Errorf("expected 'missing column' error, got: %v", err)
		}
	})
	
	t.Run("read error", func(t *testing.T) {
		// Create a reader that will fail on first read
		reader := csv.NewReader(strings.NewReader(""))
		reader.FieldsPerRecord = -1
		
		needed := []string{"gtfsstopid"}
		_, err := parseCSVHeaders(reader, needed, "stations")
		
		if err == nil {
			t.Error("expected error for empty CSV")
		}
	})
}

// Test getFeedsForStation function
func TestGetFeedsForStation(t *testing.T) {
	tests := []struct {
		name          string
		station       Station
		expectedFeeds int
		expectedURLs  []string
	}{
		{
			name: "Station with N and W routes",
			station: Station{
				StopID: "R01",
				Name:   "Astoria-Ditmars Blvd",
				Routes: []string{"N", "W"},
			},
			expectedFeeds: 1,
			expectedURLs:  []string{"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-nqrw"},
		},
		{
			name: "Station with routes from multiple feeds",
			station: Station{
				StopID: "635",
				Name:   "Times Sq-42 St",
				Routes: []string{"N", "Q", "R", "W", "1", "2", "3", "7"},
			},
			expectedFeeds: 2,
			expectedURLs: []string{
				"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs",
				"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-nqrw",
			},
		},
		{
			name: "Station with A, C, E routes",
			station: Station{
				StopID: "A32",
				Name:   "Penn Station",
				Routes: []string{"A", "C", "E"},
			},
			expectedFeeds: 1,
			expectedURLs:  []string{"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-ace"},
		},
		{
			name: "Station with L train only",
			station: Station{
				StopID: "L01",
				Name:   "8 Av",
				Routes: []string{"L"},
			},
			expectedFeeds: 1,
			expectedURLs:  []string{"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs-l"},
		},
		{
			name: "Station with S (shuttle)",
			station: Station{
				StopID: "S01",
				Name:   "Franklin Av",
				Routes: []string{"S"},
			},
			expectedFeeds: 2, // Both base and ACE feeds for shuttles
		},
		{
			name: "Station with no route info",
			station: Station{
				StopID: "TEST",
				Name:   "Test Station",
				Routes: []string{},
			},
			expectedFeeds: len(feedURLs), // Should return all feeds
		},
		{
			name: "Station with express variant",
			station: Station{
				StopID: "601",
				Name:   "Pelham Bay Park",
				Routes: []string{"6", "6X"},
			},
			expectedFeeds: 1,
			expectedURLs:  []string{"https://api-endpoint.mta.info/Dataservice/mtagtfsfeeds/nyct%2Fgtfs"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feeds := getFeedsForStation(tt.station)
			
			if len(feeds) != tt.expectedFeeds {
				t.Errorf("expected %d feeds, got %d", tt.expectedFeeds, len(feeds))
			}
			
			if len(tt.expectedURLs) > 0 {
				// Check that all expected URLs are present
				feedSet := make(map[string]bool)
				for _, url := range feeds {
					feedSet[url] = true
				}
				
				for _, expectedURL := range tt.expectedURLs {
					if !feedSet[expectedURL] {
						t.Errorf("expected feed URL %s not found", expectedURL)
					}
				}
			}
		})
	}
}

// Test loadRouteMapping with mock CSV data
func TestLoadRouteMapping(t *testing.T) {
	// Save original stations
	originalStations := stations
	defer func() { stations = originalStations }()
	
	// Create test stations
	stations = []Station{
		{StopID: "R01", Name: "Astoria-Ditmars Blvd", Lat: 40.775036, Lon: -73.912034},
		{StopID: "635", Name: "Times Sq-42 St", Lat: 40.754672, Lon: -73.986754},
		{StopID: "A32", Name: "Penn Station", Lat: 40.750373, Lon: -73.991057},
	}
	
	// Create a test server with mock CSV data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csv := `Station ID,Complex ID,GTFS Stop ID,Division,Line,Stop Name,Borough,Daytime Routes,Structure
R01,1,R01,BMT,Astoria,Astoria-Ditmars Blvd,Q,N W,Elevated
635,611,635,IRT,42 St,Times Sq-42 St,M,N Q R W 1 2 3 7,Subway
A32,614,A32,IND,8 Av,Penn Station,M,A C E,Subway`
		w.Write([]byte(csv))
	}))
	defer server.Close()
	
	// Save original URL and replace with test server
	originalURL := mtaStationsCSV
	mtaStationsCSV = server.URL
	defer func() { mtaStationsCSV = originalURL }()
	
	// Load route mappings
	err := loadRouteMapping(context.Background())
	if err != nil {
		t.Fatalf("loadRouteMapping failed: %v", err)
	}
	
	// Check that routes were loaded correctly
	tests := []struct {
		stopID        string
		expectedRoutes []string
	}{
		{"R01", []string{"N", "W"}},
		{"635", []string{"N", "Q", "R", "W", "1", "2", "3", "7"}},
		{"A32", []string{"A", "C", "E"}},
	}
	
	for _, tt := range tests {
		var found *Station
		for i := range stations {
			if stations[i].StopID == tt.stopID {
				found = &stations[i]
				break
			}
		}
		
		if found == nil {
			t.Errorf("station %s not found", tt.stopID)
			continue
		}
		
		if len(found.Routes) != len(tt.expectedRoutes) {
			t.Errorf("station %s: expected %d routes, got %d", tt.stopID, len(tt.expectedRoutes), len(found.Routes))
			continue
		}
		
		// Check each route
		routeSet := make(map[string]bool)
		for _, r := range found.Routes {
			routeSet[r] = true
		}
		
		for _, expectedRoute := range tt.expectedRoutes {
			if !routeSet[expectedRoute] {
				t.Errorf("station %s: expected route %s not found", tt.stopID, expectedRoute)
			}
		}
	}
}

// Test that route-to-feed mapping is comprehensive
func TestRouteToFeedMapping(t *testing.T) {
	// All known NYC subway routes
	allRoutes := []string{
		"1", "2", "3", "4", "5", "6", "7",
		"A", "B", "C", "D", "E", "F", "G",
		"J", "L", "M", "N", "Q", "R", "W", "Z",
		"GS", "FS", "H", "SI", "SIR",
	}
	
	for _, route := range allRoutes {
		if _, ok := routeToFeed[route]; !ok {
			t.Errorf("route %s not found in routeToFeed mapping", route)
		}
	}
	
	// Check that all mapped feeds are valid URLs
	validFeeds := make(map[string]bool)
	for _, url := range feedURLs {
		validFeeds[url] = true
	}
	
	for route, feedURL := range routeToFeed {
		if !validFeeds[feedURL] {
			t.Errorf("route %s maps to invalid feed URL: %s", route, feedURL)
		}
	}
}
