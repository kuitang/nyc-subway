# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

### Backend (Go 1.19)
```bash
# Run backend server (default port 8080)
go run backend/main.go

# Run with custom port
PORT=8081 go run backend/main.go

# Run backend tests
cd backend && go test -v

# Run tests with coverage
cd backend && go test -v -cover

# Generate coverage report
cd backend && go test -coverprofile=coverage.out && go tool cover -html=coverage.out
```

### Frontend (React 18)
```bash
# Install dependencies
cd frontend && npm install

# Start development server (default port 3000)
cd frontend && npm start

# Start with custom port and API endpoint
cd frontend && REACT_APP_API_BASE_URL=http://localhost:8081 PORT=3001 npm start

# Run tests (with --watchAll=false to avoid interactive mode)
cd frontend && npm test -- --watchAll=false

# Build for production
cd frontend && npm build
```

### API Testing
```bash
# Test all API endpoints (backend must be running)
./tests/test_api.sh

# Test with custom backend URL
./tests/test_api.sh http://localhost:8081
```

### Protobuf Generation
```bash
# Generate GTFS protobuf files (required if gtfs-realtime.proto changes)
GOPATH=/tmp/gopath go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
mkdir -p gtfs_realtime
PATH=/tmp/gopath/bin:$PATH protoc --go_out=gtfs_realtime --go_opt=paths=source_relative gtfs-realtime.proto
```

## Architecture Overview

### Backend Structure
- **main.go**: Single-file Go backend implementing REST API
- **Cache**: Uses gcache for walking time results (15-minute TTL)
- **Data Sources**: 
  - MTA GTFS-RT feeds (9 endpoints for different subway lines)
  - NYC Open Data for station metadata
  - OSRM for walking time calculations
- **Key Endpoints**:
  - `/api/stops` - Returns all stations
  - `/api/departures/nearest?lat=<>&lon=<>` - Nearest station departures
  - `/api/departures/by-name?name=<>` - Station departures by name

### Frontend Structure
- **React SPA** with functional components
- **Components**:
  - `App.js` - Main component handling geolocation and station selection
  - `NearestStop.js` - Displays departures with auto-refresh
  - `StationSelector.js` - Autocomplete station selector using react-select
  - `LoadingScreen.js` / `ErrorScreen.js` - UI states
- **Auto-refresh**: 30-second interval for departure updates
- **Environment Variables**: `REACT_APP_API_BASE_URL` for API endpoint

### Key Implementation Details

- **Departure Limiting**: Backend returns max 2 departures per route+direction combination
- **Station Routes**: Backend associates stations with specific routes to optimize feed fetching
- **Walking Time**: Calculated using OSRM API with caching
- **Error Handling**: Validates NYC area coordinates (40.3-41.1 lat, -74.5 to -73.3 lon)
- **Testing**: Separate test files for backend components (api_test.go, cache_test.go, main_test.go)

## Development Notes

- The developer is not fluent in Go or React - use standard patterns and reference official documentation
- Follow test-driven development: write tests first, verify they fail, then implement
- Always check current directory when running bash commands
- Stop after 3+ consecutive command failures
- Backend requires Go 1.19 compatibility
- Frontend uses React 18 with react-scripts 5.0.1