#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"

echo "1) /api/stops (first 3 rows):"
curl -s "$BASE_URL/api/stops" | jq '.[0:3]'

echo
echo "2) /api/departures/nearest (Midtown coords):"
LAT=40.7580
LON=-73.9855
curl -s "$BASE_URL/api/departures/nearest?lat=$LAT&lon=$LON" | jq '{station, walking, departures: (.departures[0:5])}'

echo
echo "3) /api/departures/by-name (\"Grand Central - 42 St\"):"
curl -s "$BASE_URL/api/departures/by-name?name=Grand%20Central%20-%2042%20St" | jq '{station, departures: (.departures[0:5])}'

echo
echo "4) /api/departures/nearest outside NYC (expect error):"
curl -s -o /dev/stderr -w "\nHTTP %{http_code}\n" "$BASE_URL/api/departures/nearest?lat=34.0522&lon=-118.2437"

