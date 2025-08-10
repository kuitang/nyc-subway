# NYC Subway Departures

## Setup

### Generate Protobuf Files
```bash
# Install protoc-gen-go (compatible with Go 1.19)
GOPATH=/tmp/gopath go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28

# Create output directory
mkdir -p gtfs_realtime

# Compile protobuf
PATH=/tmp/gopath/bin:$PATH protoc --go_out=gtfs_realtime --go_opt=paths=source_relative gtfs-realtime.proto
```

## Development

### Backend
```bash
go run backend/main.go
```

### Frontend
```bash
cd frontend
npm start
```

Navigate to `http://localhost:3000`

## Testing

### Backend Tests
```bash
cd backend
go test -v
```

### Frontend Tests
```bash
cd frontend
npm test
```

## API Endpoints

- `GET /api/stops` - List all subway stops
- `GET /api/departures/nearest?lat=<lat>&lon=<lon>` - Get departures for nearest stop
- `GET /api/departures/by-name?name=<stop name>` - Get departures by stop name