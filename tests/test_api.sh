#!/usr/bin/env bash
set -euo pipefail

# Test script to verify the API endpoints and that departures are limited to 2 per route/direction
# Usage: ./test_api.sh [BASE_URL]
# Run the server first: go run backend/main.go

BASE_URL="${1:-http://localhost:8080}"

echo "Testing NYC Subway API endpoints..."
echo "================================="
echo

echo "1) Testing /api/stops endpoint (first 5 stations):"
echo "---------------------------------------------------"
curl -s "$BASE_URL/api/stops" | jq '.[0:5] | .[] | {stop_id: .gtfs_stop_id, name: .stop_name}'
echo

echo "2) Testing /api/departures/nearest endpoint (Times Square area):"
echo "----------------------------------------------------------------"
LAT=40.7580
LON=-73.9855
echo "Coordinates: lat=$LAT, lon=$LON"
response=$(curl -s "$BASE_URL/api/departures/nearest?lat=$LAT&lon=$LON")
echo "$response" | jq '{
  station: .station.stop_name,
  walking_time_seconds: .walking.seconds,
  total_departures: (.departures | length),
  first_5_departures: (.departures[0:5] | map({
    route: .route_id,
    direction: .direction,
    eta_minutes: .eta_minutes,
    headsign: .headsign
  }))
}'

# Check if headsigns are being returned
echo
echo "Verifying headsign data:"
headsigns_found=$(echo "$response" | jq '.departures | map(.headsign) | map(select(. != "")) | length')
total_departures=$(echo "$response" | jq '.departures | length')
echo "✓ Found $headsigns_found headsigns out of $total_departures departures"

# Verify the 2 per route/direction limit
echo
echo "Verifying departure limits (max 2 per route+direction):"
violations=$(echo "$response" | jq '.departures | group_by("\(.route_id)_\(.direction)") | map({
  route_direction: "\(.[0].route_id)_\(.[0].direction)",
  count: length
}) | .[] | select(.count > 2)')

if [ -z "$violations" ]; then
  echo "✓ All route+direction combinations have ≤ 2 departures"
else
  echo "✗ FAILED: Found route+direction combinations with > 2 departures:"
  echo "$violations"
  exit 1
fi
echo

echo "3) Testing /api/departures/by-id endpoint (Times Sq-42 St, ID: 127):"
echo "---------------------------------------------------------------------"
response=$(curl -s "$BASE_URL/api/departures/by-id?id=127")
echo "$response" | jq '{
  station: .station.stop_name,
  station_id: .station.gtfs_stop_id,
  total_departures: (.departures | length),
  departures_by_route: (.departures | group_by("\(.route_id)_\(.direction)") | map({
    route_direction: "\(.[0].route_id)_\(.[0].direction)",
    count: length,
    times: map(.eta_minutes),
    headsigns: map(.headsign)
  }))
}'

# Check headsigns for this station  
headsigns_127=$(echo "$response" | jq '.departures | map(.headsign) | map(select(. != "")) | length')
total_127=$(echo "$response" | jq '.departures | length')
echo "✓ Found $headsigns_127 headsigns out of $total_127 departures"

# Check departure limit for this station
violations=$(echo "$response" | jq '.departures | group_by("\(.route_id)_\(.direction)") | map({
  route_direction: "\(.[0].route_id)_\(.[0].direction)",
  count: length
}) | .[] | select(.count > 2)')

if [ -n "$violations" ]; then
  echo "✗ FAILED: Station ID 127 has route+direction combinations with > 2 departures:"
  echo "$violations"
  exit 1
fi
echo

echo "4) Testing /api/departures/by-id with another station (14 St-Union Sq, ID: 635):"
echo "---------------------------------------------------------------------------------"
response635=$(curl -s "$BASE_URL/api/departures/by-id?id=635")
echo "$response635" | jq '{
  station: .station.stop_name,
  station_id: .station.gtfs_stop_id,
  total_departures: (.departures | length)
}'

# Check departure limit for this station
violations635=$(echo "$response635" | jq '.departures | group_by("\(.route_id)_\(.direction)") | map({
  route_direction: "\(.[0].route_id)_\(.[0].direction)",
  count: length
}) | .[] | select(.count > 2)')

if [ -n "$violations635" ]; then
  echo "✗ FAILED: Station ID 635 has route+direction combinations with > 2 departures:"
  echo "$violations635"
  exit 1
fi
echo

echo "5) Testing HTTP cache headers:"
echo "-------------------------------"
echo "a) /api/stops should have 24h cache (86400 seconds):"
curl -sI "$BASE_URL/api/stops" | grep -i "cache-control" || { echo "No Cache-Control header found" && exit 1; }

echo "b) /api/departures/nearest should have 30s cache with stale-while-revalidate:"
curl -sI "$BASE_URL/api/departures/nearest?lat=$LAT&lon=$LON" | grep -i "cache-control" || { echo "No Cache-Control header found" && exit 1; }

echo "c) /api/departures/by-id should have 30s cache with stale-while-revalidate:"
curl -sI "$BASE_URL/api/departures/by-id?id=127" | grep -i "cache-control" || { echo "No Cache-Control header found" && exit 1; }
echo

echo "6) Testing error cases:"
echo "-----------------------"
echo "a) Outside NYC area (Los Angeles coordinates):"
curl -s "$BASE_URL/api/departures/nearest?lat=34.0522&lon=-118.2437" | jq '.error'

echo "b) Missing latitude parameter:"
curl -s "$BASE_URL/api/departures/nearest?lon=-73.9855" | jq '.error'

echo "c) Invalid latitude format:"
curl -s "$BASE_URL/api/departures/nearest?lat=abc&lon=-73.9855" | jq '.error'

echo "d) Station ID not found:"
curl -s "$BASE_URL/api/departures/by-id?id=NoSuchIDExists" | jq '.error'

echo "e) Missing ID parameter:"
curl -s "$BASE_URL/api/departures/by-id" | jq '.error'
echo

echo "================================="
echo "API tests completed!"
echo
echo "Note: The departure limiting logic ensures max 2 departures per route+direction."
echo "This helps provide a concise view of upcoming trains without overwhelming users."