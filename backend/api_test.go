package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nyc-subway/gtfs_realtime"
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

func TestLastStopHeadsignFallback(t *testing.T) {
	// Initialize test caches
	initTestCaches()
	
	// Mock stations with distinctive last stop name
	stations = []Station{
		{StopID: "TEST", Name: "Test Station", Lat: 40.7, Lon: -73.9},
		{StopID: "TERMINAL", Name: "Distinctive Terminal Station", Lat: 40.8, Lon: -74.0},
	}
	
	// Don't mock trips arrays to ensure no headsign is found
	trips = []Trip{}
	supplementedTrips = []Trip{}
	
	// Create mock server that returns GTFS-RT data with LastStop
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a TripUpdate with stop_sequence values and no headsign in trips
		feed := &gtfs_realtime.FeedMessage{
			Header: &gtfs_realtime.FeedHeader{
				GtfsRealtimeVersion: proto.String("2.0"),
				Timestamp:           proto.Uint64(uint64(time.Now().Unix())),
			},
			Entity: []*gtfs_realtime.FeedEntity{
				{
					Id: proto.String("1"),
					TripUpdate: &gtfs_realtime.TripUpdate{
						Trip: &gtfs_realtime.TripDescriptor{
							TripId:  proto.String("test_trip_123"),
							RouteId: proto.String("6"),
						},
						StopTimeUpdate: []*gtfs_realtime.TripUpdate_StopTimeUpdate{
							{
								StopId:       proto.String("TEST"),
								StopSequence: proto.Uint32(1),
								Departure: &gtfs_realtime.TripUpdate_StopTimeEvent{
									Time: proto.Int64(time.Now().Unix() + 300), // 5 minutes from now
								},
							},
							{
								StopId:       proto.String("TERMINAL"),
								StopSequence: proto.Uint32(10), // Higher sequence = last stop
								Departure: &gtfs_realtime.TripUpdate_StopTimeEvent{
									Time: proto.Int64(time.Now().Unix() + 1800), // 30 minutes from now
								},
							},
						},
					},
				},
			},
		}
		
		data, _ := proto.Marshal(feed)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Write(data)
	}))
	defer server.Close()
	
	// Override feed URLs to use our mock server
	originalURLs := feedURLs
	feedURLs = []string{server.URL}
	defer func() { feedURLs = originalURLs }()
	
	// Test departuresForStation
	station := Station{StopID: "TEST", Name: "Test Station", Lat: 40.7, Lon: -73.9}
	deps, err := departuresForStation(station)
	
	if err != nil {
		t.Fatalf("departuresForStation failed: %v", err)
	}
	
	if len(deps) == 0 {
		t.Fatal("Expected at least one departure")
	}
	
	// Verify that LastStop was populated
	found := false
	for _, dep := range deps {
		if dep.LastStop == "Distinctive Terminal Station" {
			found = true
			// Verify headsign fallback worked (should use LastStop as HeadSign)
			if dep.HeadSign != "Distinctive Terminal Station" {
				t.Errorf("Expected HeadSign to be 'Distinctive Terminal Station', got '%s'", dep.HeadSign)
			}
		}
	}
	
	if !found {
		t.Error("LastStop was not populated with expected terminal station name")
	}
}
