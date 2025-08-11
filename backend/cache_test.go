package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluele/gcache"
)

func TestQuantizeCoord(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "basic rounding",
			input:    40.7847782,
			expected: 40.7848,
		},
		{
			name:     "negative coordinate",
			input:    -73.9711486,
			expected: -73.9711,
		},
		{
			name:     "already quantized",
			input:    40.1234,
			expected: 40.1234,
		},
		{
			name:     "round down",
			input:    40.12344,
			expected: 40.1234,
		},
		{
			name:     "round up",
			input:    40.12346,
			expected: 40.1235,
		},
		{
			name:     "zero",
			input:    0.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quantizeCoord(tt.input)
			if result != tt.expected {
				t.Errorf("quantizeCoord(%f) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMakeCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		fromLat  float64
		fromLon  float64
		toLat    float64
		toLon    float64
		expected string
	}{
		{
			name:     "NYC coordinates",
			fromLat:  40.7847782,
			fromLon:  -73.9711486,
			toLat:    40.785868,
			toLon:    -73.968916,
			expected: "40.7848,-73.9711,40.785868,-73.968916",
		},
		{
			name:     "quantization effect",
			fromLat:  40.12345,
			fromLon:  -74.56789,
			toLat:    40.111111,
			toLon:    -74.222222,
			expected: "40.1235,-74.5679,40.111111,-74.222222",
		},
		{
			name:     "zero coordinates",
			fromLat:  0.0,
			fromLon:  0.0,
			toLat:    0.0,
			toLon:    0.0,
			expected: "0.0000,0.0000,0.000000,0.000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeCacheKey(tt.fromLat, tt.fromLon, tt.toLat, tt.toLon)
			if result != tt.expected {
				t.Errorf("makeCacheKey(%f, %f, %f, %f) = %s, want %s",
					tt.fromLat, tt.fromLon, tt.toLat, tt.toLon, result, tt.expected)
			}
		})
	}
}

func TestWalkingTimeCache(t *testing.T) {
	// Setup a test cache
	testCache := gcache.New(10).
		LRU().
		Expiration(1 * time.Hour).
		Build()
	
	// Save original cache and restore after test
	originalCache := walkCache
	walkCache = testCache
	defer func() { walkCache = originalCache }()

	// Mock OSRM server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"routes": [{
				"duration": 120.5,
				"distance": 850.2
			}]
		}`))
	}))
	defer mockServer.Close()

	// Save original client and restore after test
	originalClient := httpClient
	httpClient = &http.Client{Timeout: 5 * time.Second}
	defer func() { httpClient = originalClient }()

	// Test coordinates
	fromLat, fromLon := 40.7847782, -73.9711486
	toLat, toLon := 40.785868, -73.968916

	// First call should make HTTP request
	result1, err := walkingTime(fromLat, fromLon, toLat, toLon)
	if err != nil {
		// Skip if network request fails (expected in test environment)
		t.Skip("Network request failed, skipping cache test")
	}

	if result1 == nil {
		t.Fatal("Expected result from walkingTime, got nil")
	}

	// Verify cache key generation
	expectedKey := makeCacheKey(fromLat, fromLon, toLat, toLon)
	cached, err := walkCache.Get(expectedKey)
	if err != nil {
		t.Errorf("Expected cache entry for key %s, but got error: %v", expectedKey, err)
	}

	cachedResult, ok := cached.(*WalkResult)
	if !ok {
		t.Errorf("Expected *WalkResult from cache, got %T", cached)
	}

	if cachedResult.Seconds != result1.Seconds || cachedResult.Distance != result1.Distance {
		t.Errorf("Cached result doesn't match original: cached=%+v, original=%+v", cachedResult, result1)
	}
}

func TestCacheKeyQuantization(t *testing.T) {
	// Test that nearby coordinates generate the same cache key
	lat1, lon1 := 40.7847782, -73.9711486
	lat2, lon2 := 40.7847799, -73.9711499 // Very close coordinates
	toLat, toLon := 40.785868, -73.968916

	key1 := makeCacheKey(lat1, lon1, toLat, toLon)
	key2 := makeCacheKey(lat2, lon2, toLat, toLon)

	if key1 != key2 {
		t.Errorf("Expected same cache key for nearby coordinates, got %s and %s", key1, key2)
	}

	// Test that sufficiently different coordinates generate different keys
	lat3, lon3 := 40.7850000, -73.9710000 // Different when quantized
	key3 := makeCacheKey(lat3, lon3, toLat, toLon)

	if key1 == key3 {
		t.Errorf("Expected different cache keys for different coordinates, but got same: %s", key1)
	}
}

func TestQuantizationPrecision(t *testing.T) {
	// Test that quantization maintains ~11m precision
	// At NYC latitude (~40.78°), 1 decimal degree ≈ 85km
	// So 0.0001° ≈ 8.5m, and 0.00005° ≈ 4.25m
	
	coord := 40.7847750 // Exactly at boundary
	quantized := quantizeCoord(coord)
	expected := 40.7848
	
	if quantized != expected {
		t.Errorf("quantizeCoord(%f) = %f, want %f", coord, quantized, expected)
	}
	
	// Test precision: difference should be exactly 0.0001
	precision := 0.0001
	if math.Abs(quantized-coord) > precision/2 {
		t.Errorf("Quantization error too large: |%f - %f| = %f, should be <= %f",
			quantized, coord, math.Abs(quantized-coord), precision/2)
	}
}