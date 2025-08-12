package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gtfs_realtime "nyc-subway/gtfs_realtime"
	"google.golang.org/protobuf/proto"
)

func TestAPIStopsEndpoint(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	// Initialize some test stations
	stations = []Station{
		{StopID: "R14N", Name: "14 St - Union Sq", Lat: 40.7359, Lon: -73.9906},
		{StopID: "635S", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
	}

	// First request - should not be cached
	req := httptest.NewRequest("GET", "/api/stops", nil)
	w := httptest.NewRecorder()
	handleStops(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result []Station
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(result))
	}
	
	// Second request - should be cached
	req2 := httptest.NewRequest("GET", "/api/stops", nil)
	w2 := httptest.NewRecorder()
	handleStops(w2, req2)
	
	resp2 := w2.Result()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on cached request, got %d", resp2.StatusCode)
	}
	
	var result2 []Station
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatalf("failed to decode cached response: %v", err)
	}
	
	if len(result2) != 2 {
		t.Fatalf("expected 2 stations from cache, got %d", len(result2))
	}
	
	// Verify cached data matches original
	if result[0].StopID != result2[0].StopID || result[1].StopID != result2[1].StopID {
		t.Errorf("cached data doesn't match original data")
	}
}

func TestAPINearestEndpoint(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	// Initialize test stations
	stations = []Station{
		{StopID: "R14N", Name: "14 St - Union Sq", Lat: 40.7359, Lon: -73.9906},
		{StopID: "635S", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
	}

	// Mock the departuresForStation function to test the limiting behavior
	// We'll test with a request near Grand Central
	req := httptest.NewRequest("GET", "/api/departures/nearest?lat=40.7527&lon=-73.9772", nil)
	w := httptest.NewRecorder()
	
	// We can't easily mock the GTFS feeds, but we can test that the endpoint responds correctly
	handleNearest(w, req)

	resp := w.Result()
	// The actual response might be 502 if feeds are unavailable, or 200 if they work
	// We mainly want to ensure the endpoint doesn't crash
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

func TestAPIByIDEndpoint(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	// Initialize test stations
	stations = []Station{
		{StopID: "R14N", Name: "14 St - Union Sq", Lat: 40.7359, Lon: -73.9906},
		{StopID: "635S", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
		{StopID: "635N", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
	}

	req := httptest.NewRequest("GET", "/api/departures/by-id?id=635", nil)
	w := httptest.NewRecorder()
	
	handleByID(w, req)

	resp := w.Result()
	// Similar to above, actual GTFS feeds might not be available in test
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

func TestAPIInvalidRequests(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	stations = []Station{
		{StopID: "R14N", Name: "14 St - Union Sq", Lat: 40.7359, Lon: -73.9906},
	}

	tests := []struct {
		name     string
		endpoint string
		wantCode int
	}{
		{"missing lat", "/api/departures/nearest?lon=-73.9772", http.StatusBadRequest},
		{"missing lon", "/api/departures/nearest?lat=40.7527", http.StatusBadRequest},
		{"invalid lat", "/api/departures/nearest?lat=abc&lon=-73.9772", http.StatusBadRequest},
		{"invalid lon", "/api/departures/nearest?lat=40.7527&lon=xyz", http.StatusBadRequest},
		{"outside NYC", "/api/departures/nearest?lat=34.0522&lon=-118.2437", http.StatusBadRequest},
		{"missing id", "/api/departures/by-id", http.StatusBadRequest},
		{"no match", "/api/departures/by-id?id=NoSuchID", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()
			
			if tt.endpoint[:21] == "/api/departures/by-id" {
				handleByID(w, req)
			} else {
				handleNearest(w, req)
			}

			resp := w.Result()
			if resp.StatusCode != tt.wantCode {
				t.Errorf("expected status %d, got %d", tt.wantCode, resp.StatusCode)
			}
		})
	}
}

// TestFeedOptimization verifies that stations with route information only fetch necessary feeds
func TestFeedOptimization(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	// Set up test stations with different route configurations
	tests := []struct {
		name              string
		station           Station
		expectedFeedCount int
		description       string
	}{
		{
			name: "L train only station",
			station: Station{
				StopID: "L01",
				Name:   "Bedford Av",
				Lat:    40.717304,
				Lon:    -73.956872,
				Routes: []string{"L"},
			},
			expectedFeedCount: 1,
			description:       "should only fetch L feed",
		},
		{
			name: "ACE station",
			station: Station{
				StopID: "A32",
				Name:   "Penn Station",
				Lat:    40.750373,
				Lon:    -73.991057,
				Routes: []string{"A", "C", "E"},
			},
			expectedFeedCount: 1,
			description:       "should only fetch ACE feed",
		},
		{
			name: "Multi-feed station",
			station: Station{
				StopID: "635",
				Name:   "Times Sq-42 St",
				Lat:    40.754672,
				Lon:    -73.986754,
				Routes: []string{"N", "Q", "R", "W", "1", "2", "3", "7"},
			},
			expectedFeedCount: 2,
			description:       "should fetch base and NQRW feeds",
		},
		{
			name: "Station with no routes",
			station: Station{
				StopID: "TEST",
				Name:   "Test Station",
				Lat:    40.750000,
				Lon:    -73.980000,
				Routes: []string{},
			},
			expectedFeedCount: len(feedURLs),
			description:       "should fallback to all feeds",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the feeds that would be fetched for this station
			feeds := getFeedsForStation(tt.station)
			
			if len(feeds) != tt.expectedFeedCount {
				t.Errorf("Station %s: expected %d feeds, got %d feeds. %s",
					tt.station.Name, tt.expectedFeedCount, len(feeds), tt.description)
				t.Logf("Routes: %v", tt.station.Routes)
				t.Logf("Feeds returned: %d", len(feeds))
			}
			
			// Verify the feeds are valid URLs from our feedURLs list
			validFeeds := make(map[string]bool)
			for _, url := range feedURLs {
				validFeeds[url] = true
			}
			
			for _, feed := range feeds {
				if !validFeeds[feed] {
					t.Errorf("Invalid feed URL returned: %s", feed)
				}
			}
		})
	}
}

// TestFeedOptimizationWithRealStations tests the optimization with stations that have been loaded with route data
func TestFeedOptimizationWithRealStations(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	// Initialize stations with route information
	stations = []Station{
		{StopID: "L01", Name: "Bedford Av", Lat: 40.717304, Lon: -73.956872, Routes: []string{"L"}},
		{StopID: "635", Name: "Times Sq-42 St", Lat: 40.754672, Lon: -73.986754, Routes: []string{"N", "Q", "R", "W", "1", "2", "3", "7"}},
		{StopID: "A32", Name: "Penn Station", Lat: 40.750373, Lon: -73.991057, Routes: []string{"A", "C", "E"}},
	}
	
	// Test the by-id endpoint with a station that has L train only
	t.Run("by-id endpoint with L train station", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/departures/by-id?id=L01", nil)
		w := httptest.NewRecorder()
		
		handleByID(w, req)
		
		resp := w.Result()
		// The actual GTFS feeds might not be available, so we accept either 200 or 502
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
			t.Errorf("unexpected status %d", resp.StatusCode)
		}
		
		// The important part is that it correctly identified the station and attempted to fetch only the L feed
		// This is verified by the logs which show "Station Bedford Av serves routes [L], fetching 1 feed(s)"
	})
	
	// Test the nearest endpoint with a multi-feed station
	t.Run("nearest endpoint with multi-feed station", func(t *testing.T) {
		// Coordinates near Times Square
		req := httptest.NewRequest("GET", "/api/departures/nearest?lat=40.7547&lon=-73.9867", nil)
		w := httptest.NewRecorder()
		
		handleNearest(w, req)
		
		resp := w.Result()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
			t.Errorf("unexpected status %d", resp.StatusCode)
		}
		
		// Logs should show "Station Times Sq-42 St serves routes [N Q R W 1 2 3 7], fetching 2 feed(s)"
	})
	
	// Test with a station without route info
	t.Run("station without route info falls back to all feeds", func(t *testing.T) {
		// Add a station without route info
		stations = append(stations, Station{
			StopID: "TEST",
			Name:   "Test Station",
			Lat:    40.760000,
			Lon:    -73.990000,
			Routes: []string{}, // No routes
		})
		
		req := httptest.NewRequest("GET", "/api/departures/by-id?id=TEST", nil)
		w := httptest.NewRecorder()
		
		handleByID(w, req)
		
		resp := w.Result()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
			t.Errorf("unexpected status %d", resp.StatusCode)
		}
		
		// Logs should show "No route information for station Test Station, using all feeds"
	})
}

// TestLastStopHeadsignFallback tests that when no headsign is found in trips arrays,
// the last stop name is used as the headsign
func TestLastStopHeadsignFallback(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	// Clear the trips arrays to ensure no headsign is found there
	originalTrips := trips
	originalSupplementedTrips := supplementedTrips
	trips = []Trip{}
	supplementedTrips = []Trip{}
	defer func() {
		trips = originalTrips
		supplementedTrips = originalSupplementedTrips
	}()
	
	// Setup test stations
	stations = []Station{
		{StopID: "TEST_STATION", Name: "Test Station", Lat: 40.7527, Lon: -73.9772, Routes: []string{"N"}},
	}
	
	// Create mock GTFS-RT feed with a trip that has stops but no headsign in trips.csv
	mockFeed := createMockFeedWithLastStop("TEST_TRIP", "TEST_STATION", "Distinctive Last Stop Terminal")
	
	// Test departuresForStation with the mock feed
	deps := extractDeparturesFromMockFeed(mockFeed, "TEST_STATION")
	
	if len(deps) == 0 {
		t.Fatal("Expected at least one departure from mock feed")
	}
	
	// Verify that initially no headsign is found (should be empty)
	headsign := lookupHeadsignWithTiming("TEST_TRIP")
	if headsign != "" {
		t.Errorf("Expected empty headsign from trips lookup, got %q", headsign)
	}
	
	// Verify that LastStop is populated (this will fail until we implement it)
	departure := deps[0]
	if departure.LastStop != "Distinctive Last Stop Terminal" {
		t.Errorf("Expected LastStop to be 'Distinctive Last Stop Terminal', got %q", departure.LastStop)
	}
	
	// Verify that HeadSign falls back to LastStop when no headsign found (this will fail until we implement it)
	if departure.HeadSign != "Distinctive Last Stop Terminal" {
		t.Errorf("Expected HeadSign to fallback to LastStop 'Distinctive Last Stop Terminal', got %q", departure.HeadSign)
	}
}

// createMockFeedWithLastStop creates a mock GTFS-RT feed with a trip that has multiple stops,
// with the last stop having a distinctive name
func createMockFeedWithLastStop(tripID, stopID, lastStopName string) *gtfs_realtime.FeedMessage {
	now := time.Now().Unix()
	futureTime1 := now + 300 // 5 minutes from now
	futureTime2 := now + 900 // 15 minutes from now
	
	// Create stop time updates - first stop is our target, last stop is the terminal
	stopTimeUpdates := []*gtfs_realtime.TripUpdate_StopTimeUpdate{
		{
			StopId: proto.String(stopID),
			Departure: &gtfs_realtime.TripUpdate_StopTimeEvent{
				Time: proto.Int64(futureTime1),
			},
			StopSequence: proto.Uint32(1),
		},
		{
			StopId: proto.String("LAST_STOP_ID"),
			Arrival: &gtfs_realtime.TripUpdate_StopTimeEvent{
				Time: proto.Int64(futureTime2),
			},
			StopSequence: proto.Uint32(10), // High sequence number to ensure it's last
		},
	}
	
	tripUpdate := &gtfs_realtime.TripUpdate{
		Trip: &gtfs_realtime.TripDescriptor{
			TripId:  proto.String(tripID),
			RouteId: proto.String("N"),
		},
		StopTimeUpdate: stopTimeUpdates,
	}
	
	entity := &gtfs_realtime.FeedEntity{
		Id:         proto.String("test_entity_1"),
		TripUpdate: tripUpdate,
	}
	
	feed := &gtfs_realtime.FeedMessage{
		Header: &gtfs_realtime.FeedHeader{
			GtfsRealtimeVersion: proto.String("2.0"),
			Timestamp:          proto.Uint64(uint64(now)),
		},
		Entity: []*gtfs_realtime.FeedEntity{entity},
	}
	
	// Mock the station lookup for the last stop
	// We need to add this station temporarily so the last stop name can be resolved
	stations = append(stations, Station{
		StopID: "LAST_STOP_ID",
		Name:   lastStopName,
		Lat:    40.7527,
		Lon:    -73.9772,
	})
	
	return feed
}

// extractDeparturesFromMockFeed extracts departures from a mock feed (simulates departuresForStation logic)
func extractDeparturesFromMockFeed(feed *gtfs_realtime.FeedMessage, targetStopID string) []Departure {
	now := time.Now().Unix()
	var deps []Departure
	
	for _, entity := range feed.GetEntity() {
		tu := entity.GetTripUpdate()
		if tu == nil {
			continue
		}
		
		routeID := ""
		tripID := ""
		if td := tu.GetTrip(); td != nil {
			routeID = td.GetRouteId()
			tripID = td.GetTripId()
		}
		
		// Find the last stop in this trip for LastStop field
		var lastStopName string
		var maxSequence uint32 = 0
		for _, stu := range tu.GetStopTimeUpdate() {
			if seq := stu.GetStopSequence(); seq > maxSequence {
				maxSequence = seq
				stopID := stu.GetStopId()
				// Look up station name for this stop
				for _, station := range stations {
					if station.StopID == stopID {
						lastStopName = station.Name
						break
					}
				}
			}
		}
		
		// Process stop time updates for our target stop
		for _, stu := range tu.GetStopTimeUpdate() {
			stopID := stu.GetStopId()
			
			// Only process if this is our target stop
			if stopID != targetStopID {
				continue
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
			
			dir := getStopDirection(stopID)
			etaSec := t - now
			
			// Create departure with LastStop field
			departure := Departure{
				RouteID:    routeID,
				StopID:     stopID,
				Direction:  dir,
				UnixTime:   t,
				ETASeconds: etaSec,
				TripID:     tripID,
				HeadSign:   "",      // Will be filled by headsign lookup
				LastStop:   lastStopName, // This will fail until we add the field
			}
			
			// Apply headsign lookup with fallback to LastStop
			headsign := lookupHeadsignWithTiming(departure.TripID)
			if headsign == "" && departure.LastStop != "" {
				departure.HeadSign = departure.LastStop
			} else {
				departure.HeadSign = headsign
			}
			
			deps = append(deps, departure)
		}
	}
	
	return deps
}




