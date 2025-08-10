package main

import (
	"testing"
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
