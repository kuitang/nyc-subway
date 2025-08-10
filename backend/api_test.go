package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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




