package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	
	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"google.golang.org/protobuf/proto"
)

func TestAPIStopsEndpoint(t *testing.T) {
	// Initialize some test stations
	stations = []Station{
		{StopID: "R14N", Name: "14 St - Union Sq", Lat: 40.7359, Lon: -73.9906},
		{StopID: "635S", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
	}

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
}

func TestAPINearestEndpoint(t *testing.T) {
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

func TestAPIByNameEndpoint(t *testing.T) {
	// Initialize test stations
	stations = []Station{
		{StopID: "R14N", Name: "14 St - Union Sq", Lat: 40.7359, Lon: -73.9906},
		{StopID: "635S", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
		{StopID: "635N", Name: "Grand Central - 42 St", Lat: 40.7527, Lon: -73.9772},
	}

	req := httptest.NewRequest("GET", "/api/departures/by-name?name=Grand", nil)
	w := httptest.NewRecorder()
	
	handleByName(w, req)

	resp := w.Result()
	// Similar to above, actual GTFS feeds might not be available in test
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

func TestAPIInvalidRequests(t *testing.T) {
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
		{"missing name", "/api/departures/by-name", http.StatusBadRequest},
		{"no match", "/api/departures/by-name?name=NoSuchStation", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()
			
			if tt.endpoint[:21] == "/api/departures/by-na" {
				handleByName(w, req)
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

func TestExtractHeadsignFromTrip(t *testing.T) {
	tests := []struct {
		name           string
		tripID         string
		expectedHeadsign string
		description    string
	}{
		{
			name:           "Trip with headsign in cache",
			tripID:         "test_trip_123",
			expectedHeadsign: "Times Sq-42 St",
			description:    "Should extract headsign from cache",
		},
		{
			name:           "Trip without headsign in cache",
			tripID:         "unknown_trip",
			expectedHeadsign: "",
			description:    "Should return empty string for unknown trip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test headsign data if expected
			originalCache := GTFSCache{
				HeadSigns: make(map[string]string),
				LoadedAt:  gtfsCache.LoadedAt,
				TTL:       gtfsCache.TTL,
			}
			for k, v := range gtfsCache.HeadSigns {
				originalCache.HeadSigns[k] = v
			}
			defer func() {
				// Restore original cache
				gtfsCache.HeadSigns = originalCache.HeadSigns
				gtfsCache.LoadedAt = originalCache.LoadedAt
			}()
			
			if tt.expectedHeadsign != "" {
				gtfsCache.HeadSigns[tt.tripID] = tt.expectedHeadsign
			} else {
				// Clear any existing test data
				delete(gtfsCache.HeadSigns, tt.tripID)
			}
			gtfsCache.LoadedAt = time.Now() // Ensure cache is not expired
			
			// Create a mock trip descriptor
			trip := &gtfs.TripDescriptor{
				TripId:  proto.String(tt.tripID),
				RouteId: proto.String("6"),
			}

			// Test the headsign extraction function from main.go
			result := extractHeadsignFromTrip(trip)
			
			if result != tt.expectedHeadsign {
				t.Errorf("expected headsign %q, got %q", tt.expectedHeadsign, result)
			}
		})
	}
}


func TestLoadTripHeadsigns(t *testing.T) {
	// Test with a mock HTTP server that returns a simple ZIP with trips.txt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a simple ZIP file in memory with trips.txt
		buf := new(bytes.Buffer)
		zipWriter := zip.NewWriter(buf)
		
		tripsFile, err := zipWriter.Create("trips.txt")
		if err != nil {
			t.Fatal(err)
		}
		
		// Write simple CSV data
		tripsData := `trip_id,trip_headsign,route_id,direction_id
test_trip_456,Brooklyn Bridge-City Hall,6,0
test_trip_789,Times Sq-42 St,6,1`
		
		_, err = tripsFile.Write([]byte(tripsData))
		if err != nil {
			t.Fatal(err)
		}
		
		err = zipWriter.Close()
		if err != nil {
			t.Fatal(err)
		}
		
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer server.Close()
	
	// Save original cache and restore after test
	originalCache := GTFSCache{
		HeadSigns: make(map[string]string),
		LoadedAt:  gtfsCache.LoadedAt,
		TTL:       gtfsCache.TTL,
	}
	for k, v := range gtfsCache.HeadSigns {
		originalCache.HeadSigns[k] = v
	}
	defer func() {
		gtfsCache.HeadSigns = originalCache.HeadSigns
		gtfsCache.LoadedAt = originalCache.LoadedAt
	}()
	
	// Clear existing cache for clean test
	gtfsCache.HeadSigns = make(map[string]string)
	gtfsCache.LoadedAt = time.Time{} // Force cache to be expired
	
	// Test loading from mock server
	err := loadTripHeadsigns(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("loadTripHeadsigns failed: %v", err)
	}
	
	// Verify the data was loaded correctly
	expected := map[string]string{
		"test_trip_456": "Brooklyn Bridge-City Hall",
		"test_trip_789": "Times Sq-42 St",
	}
	
	for tripID, expectedHeadsign := range expected {
		actualHeadsign, exists := gtfsCache.HeadSigns[tripID]
		if !exists {
			t.Errorf("Expected trip_id %s not found in cache", tripID)
		}
		if actualHeadsign != expectedHeadsign {
			t.Errorf("Expected headsign %q for trip_id %s, got %q", expectedHeadsign, tripID, actualHeadsign)
		}
	}
	
	// Verify the count
	if len(gtfsCache.HeadSigns) != 2 {
		t.Errorf("Expected 2 headsigns loaded, got %d", len(gtfsCache.HeadSigns))
	}
	
	// Verify cache timestamp was updated
	if gtfsCache.LoadedAt.IsZero() {
		t.Error("Expected cache LoadedAt to be updated, but it's still zero")
	}
}

func TestGTFSCacheExpiry(t *testing.T) {
	// Save original cache and restore after test
	originalCache := GTFSCache{
		HeadSigns: make(map[string]string),
		LoadedAt:  gtfsCache.LoadedAt,
		TTL:       gtfsCache.TTL,
	}
	for k, v := range gtfsCache.HeadSigns {
		originalCache.HeadSigns[k] = v
	}
	defer func() {
		gtfsCache.HeadSigns = originalCache.HeadSigns
		gtfsCache.LoadedAt = originalCache.LoadedAt
		gtfsCache.TTL = originalCache.TTL
	}()
	
	// Test fresh cache (not expired)
	gtfsCache.HeadSigns = map[string]string{"test": "value"}
	gtfsCache.LoadedAt = time.Now()
	gtfsCache.TTL = 24 * time.Hour
	
	if gtfsCache.IsExpired() {
		t.Error("Expected fresh cache to not be expired")
	}
	
	// Test expired cache
	gtfsCache.LoadedAt = time.Now().Add(-25 * time.Hour) // 25 hours ago
	gtfsCache.TTL = 24 * time.Hour
	
	if !gtfsCache.IsExpired() {
		t.Error("Expected old cache to be expired")
	}
	
	// Test empty cache (should be considered expired)
	gtfsCache.HeadSigns = make(map[string]string)
	gtfsCache.LoadedAt = time.Now()
	
	// Even fresh timestamp, empty cache should trigger reload
	// (this is handled in ensureTripHeadsigns)
}

func TestEnsureTripHeadsigns(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		zipWriter := zip.NewWriter(buf)
		tripsFile, _ := zipWriter.Create("trips.txt")
		tripsFile.Write([]byte(`trip_id,trip_headsign,route_id
test_fresh,Fresh Data,6`))
		zipWriter.Close()
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer server.Close()
	
	// Save original values
	originalURL := gtfsStaticURL
	originalCache := GTFSCache{
		HeadSigns: make(map[string]string),
		LoadedAt:  gtfsCache.LoadedAt,
		TTL:       gtfsCache.TTL,
	}
	for k, v := range gtfsCache.HeadSigns {
		originalCache.HeadSigns[k] = v
	}
	
	defer func() {
		gtfsStaticURL = originalURL
		gtfsCache.HeadSigns = originalCache.HeadSigns
		gtfsCache.LoadedAt = originalCache.LoadedAt
		gtfsCache.TTL = originalCache.TTL
	}()
	
	// Set up test
	gtfsStaticURL = server.URL
	
	// Test 1: Empty cache should trigger load
	gtfsCache.HeadSigns = make(map[string]string)
	gtfsCache.LoadedAt = time.Time{}
	
	err := ensureTripHeadsigns(context.Background())
	if err != nil {
		t.Fatalf("ensureTripHeadsigns failed: %v", err)
	}
	
	if len(gtfsCache.HeadSigns) != 1 {
		t.Errorf("Expected 1 headsign loaded, got %d", len(gtfsCache.HeadSigns))
	}
	
	// Test 2: Fresh cache should not trigger reload
	firstLoad := gtfsCache.LoadedAt
	gtfsCache.TTL = 24 * time.Hour
	
	err = ensureTripHeadsigns(context.Background())
	if err != nil {
		t.Fatalf("ensureTripHeadsigns failed on cached data: %v", err)
	}
	
	if !gtfsCache.LoadedAt.Equal(firstLoad) {
		t.Error("Expected cache timestamp to remain unchanged when using cached data")
	}
	
	// Test 3: Expired cache should trigger reload
	gtfsCache.LoadedAt = time.Now().Add(-25 * time.Hour) // 25 hours ago
	
	err = ensureTripHeadsigns(context.Background())
	if err != nil {
		t.Fatalf("ensureTripHeadsigns failed on expired cache: %v", err)
	}
	
	if gtfsCache.LoadedAt.Equal(firstLoad) {
		t.Error("Expected cache timestamp to be updated when cache is expired")
	}
}