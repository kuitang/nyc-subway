package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
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
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	// Should handle missing file gracefully
	serveIndex(w, req)
	// We expect a 404 since frontend/index.html doesn't exist in test environment
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing file, got %d", w.Code)
	}
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
	incrementality := gtfs.FeedHeader_FULL_DATASET
	
	mockFeed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: &version,
			Timestamp:           &timestamp,
			Incrementality:      &incrementality,
		},
		Entity: []*gtfs.FeedEntity{
			{
				Id: proto.String("entity1"),
				TripUpdate: &gtfs.TripUpdate{
					Trip: &gtfs.TripDescriptor{
						RouteId: proto.String("6"),
						TripId:  proto.String("trip1"),
					},
					StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{
						{
							StopId: proto.String("635N"),
							Arrival: &gtfs.TripUpdate_StopTimeEvent{
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

	stations := []Station{{StopID: "635N", Name: "Test", Lat: 40.75, Lon: -73.98}}
	deps, err := departuresForStops(stations)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deps) != 1 {
		t.Errorf("expected 1 departure with arrival-only time, got %d", len(deps))
	}
}
