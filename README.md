# NYC Subway Departures

## Development

1. Start the backend API server:
```bash
go run backend/main.go
```

2. In a separate terminal, start the React dev server:
```bash
cd frontend
npm start
```

3. Navigate to `http://localhost:3000` to view the React app
4. The React app will proxy API calls to `http://localhost:8080`

## Production

1. Build the frontend:
```bash
cd frontend
npm run build
```

2. Serve the built files with a static server:
```bash
npm install -g serve
serve -s build -p 3000
```

3. Ensure the backend is running on port 8080:
```bash
go run backend/main.go
```

4. Navigate to `http://localhost:3000`

## Testing

### Backend Tests

Run Go tests from the backend directory:

```bash
cd backend
go test -v
```

Run specific test suites:

```bash
# Test headsign extraction functionality
go test -v -run TestHeadsignExtraction

# Test departure limiting functionality
go test -v -run TestLimitDeparturesByRouteAndDirection
```

### Frontend Tests

Run React tests from the frontend directory:

```bash
cd frontend
npm test
```

Run tests in CI mode (non-interactive):

```bash
npm test -- --watchAll=false
```

#### Test Coverage

The frontend tests cover:
- Headsign display when available from GTFS data
- Fallback to direction-based text (Northbound/Southbound/Eastbound/Westbound)
- Mixed headsign availability scenarios
- Edge cases like empty/missing headsign fields

## API Endpoints

- `GET /api/stops` - List all subway stops
- `GET /api/departures/nearest?lat=<lat>&lon=<lon>` - Get departures for nearest stop
- `GET /api/departures/by-name?name=<stop name>` - Get departures by stop name

## Features

### Train Direction Display

The app displays train destinations using a hybrid approach combining real-time and static GTFS data:

1. **Primary**: Train headsign (e.g., "Times Sq-42 St", "Coney Island-Stillwell Av") from static GTFS trips.txt data
2. **Fallback**: Direction-based text ("Northbound", "Southbound", "Eastbound", "Westbound") when headsign is unavailable

#### Implementation Details

- **Static GTFS Data**: On startup, the backend downloads and parses the MTA's static GTFS ZIP file to build a lookup table from `trip_id` to `trip_headsign`
- **Intelligent Caching**: 24-hour TTL cache prevents unnecessary downloads, with background refresh when data expires
- **Real-time Integration**: For each real-time departure, the backend uses the `trip_id` from the GTFS-RT feed to lookup the corresponding headsign
- **Performance Optimizations**: 
  - Proper CSV reader with memory optimization for large files
  - Atomic cache updates to prevent inconsistent states
  - Background refresh to avoid blocking user requests
- **Graceful Degradation**: If static data fails to load or a trip_id is not found, the system falls back to directional text

This provides users with accurate destination information while maintaining excellent performance and system reliability.