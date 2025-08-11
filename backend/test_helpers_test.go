package main

import (
	"time"

	"github.com/bluele/gcache"
)

// initTestCaches initializes all caches with test-appropriate configurations.
// Uses smaller cache sizes than production for testing efficiency.
func initTestCaches() {
	// Walk cache: smaller size (10 vs 10000 in production)
	walkCache = gcache.New(10).
		LRU().
		Expiration(1 * time.Hour).
		Build()

	// Stops cache: same size as production (1)
	stopsCache = gcache.New(1).
		LRU().
		Expiration(24 * time.Hour).
		Build()

	// Transit feed cache: same size as production (20)
	transitFeedCache = gcache.New(20).
		LRU().
		Expiration(30 * time.Second).
		Build()
}