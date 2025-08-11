#!/bin/bash

# Test 30-second refresh behavior
# This script makes repeated requests to verify caching behavior

API_URL="${1:-http://localhost:8080}"
STATION_ID="127"  # Times Sq
LAT="40.7580"
LON="-73.9855"

echo "Testing 30-second refresh behavior"
echo "==================================="
echo ""

echo "Test 1: Verifying backend cache (30s TTL for departures)"
echo "---------------------------------------------------------"

# First request - should be fresh
echo "Request 1 (t=0s):"
RESPONSE1=$(curl -s "$API_URL/api/departures/nearest?lat=$LAT&lon=$LON")
echo "$RESPONSE1" | jq -r '.station.stop_name // "Failed to get station"'
DEPARTURES1=$(echo "$RESPONSE1" | jq '.departures | length')
echo "  Departures count: $DEPARTURES1"

# Second request immediately - should be cached
echo ""
echo "Request 2 (t=1s) - Should be from cache:"
sleep 1
RESPONSE2=$(curl -s "$API_URL/api/departures/nearest?lat=$LAT&lon=$LON")
DEPARTURES2=$(echo "$RESPONSE2" | jq '.departures | length')
echo "  Departures count: $DEPARTURES2"

# Check if responses are identical (cached)
if [ "$RESPONSE1" = "$RESPONSE2" ]; then
  echo "  ✓ Response is cached (identical to first request)"
else
  echo "  ⚠ Response differs (may not be cached or data changed)"
fi

echo ""
echo "Test 2: Checking HTTP Cache-Control headers"
echo "--------------------------------------------"
HEADERS=$(curl -sI "$API_URL/api/departures/nearest?lat=$LAT&lon=$LON" | grep -i cache-control)
echo "Cache-Control header: $HEADERS"
if echo "$HEADERS" | grep -q "max-age=30"; then
  echo "  ✓ Correct cache header (30s max-age)"
else
  echo "  ✗ Incorrect cache header"
fi

echo ""
echo "Test 3: Testing by-id endpoint caching"
echo "---------------------------------------"

# First request
echo "Request 1 (t=0s):"
RESPONSE3=$(curl -s "$API_URL/api/departures/by-id?id=$STATION_ID")
echo "$RESPONSE3" | jq -r '.station.stop_name // "Failed to get station"'
DEPARTURES3=$(echo "$RESPONSE3" | jq '.departures | length')
echo "  Departures count: $DEPARTURES3"

# Second request - should be cached
echo ""
echo "Request 2 (t=1s) - Should be from cache:"
sleep 1
RESPONSE4=$(curl -s "$API_URL/api/departures/by-id?id=$STATION_ID")
DEPARTURES4=$(echo "$RESPONSE4" | jq '.departures | length')
echo "  Departures count: $DEPARTURES4"

if [ "$RESPONSE3" = "$RESPONSE4" ]; then
  echo "  ✓ Response is cached (identical to first request)"
else
  echo "  ⚠ Response differs (may not be cached or data changed)"
fi

echo ""
echo "Test 4: Simulating frontend 30s refresh pattern"
echo "------------------------------------------------"
echo "Making 3 requests with 30s intervals (this will take 60 seconds)..."
echo ""

for i in 1 2 3; do
  TIME=$((($i - 1) * 30))
  echo "Request $i (t=${TIME}s):"
  
  RESPONSE=$(curl -s "$API_URL/api/departures/nearest?lat=$LAT&lon=$LON")
  STATION=$(echo "$RESPONSE" | jq -r '.station.stop_name // "Unknown"')
  DEPARTURES=$(echo "$RESPONSE" | jq '.departures | length // 0')
  
  if [ $DEPARTURES -gt 0 ]; then
    # Get first departure ETA
    FIRST_ETA=$(echo "$RESPONSE" | jq -r '.departures[0].eta_seconds // "N/A"')
    echo "  Station: $STATION"
    echo "  Departures: $DEPARTURES"
    echo "  First departure ETA: ${FIRST_ETA}s"
  else
    echo "  Station: $STATION"  
    echo "  No departures found"
  fi
  
  if [ $i -lt 3 ]; then
    echo "  Waiting 30 seconds for next refresh..."
    sleep 30
  fi
  echo ""
done

echo "==================================="
echo "Refresh test completed!"
echo ""
echo "Summary:"
echo "- Backend cache TTL: 30 seconds for departure data"
echo "- HTTP Cache-Control: max-age=30, stale-while-revalidate=10"
echo "- Frontend refresh interval: 30 seconds"
echo "- This ensures fresh data while minimizing API load"