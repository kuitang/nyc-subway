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

### Single Instance
```bash
# Backend (port 8080)
go run backend/main.go

# Frontend (port 3000)
cd frontend
npm start
```
Navigate to `http://localhost:3000`

### Multiple Instances (Git Worktree)

**Setup worktrees:**
```bash
cd ..
git worktree add ./nyc-subway-main main
git worktree add ./nyc-subway-feature your-branch-name
```

**Instance 1 (main branch):**
```bash
cd ../nyc-subway-main

# Backend on port 8080
PORT=8080 go run backend/main.go

# Frontend on port 3000 (new terminal)
cd frontend
REACT_APP_API_BASE_URL=http://localhost:8080 npm start
```

**Instance 2 (feature branch):**
```bash
cd ../nyc-subway-feature

# Backend on port 8081
PORT=8081 go run backend/main.go

# Frontend on port 3001 (new terminal)  
cd frontend
REACT_APP_API_BASE_URL=http://localhost:8081 PORT=3001 npm start
```

**Access:**
- Instance 1: `http://localhost:3000`
- Instance 2: `http://localhost:3001`

**Cleanup:**
```bash
git worktree remove ../nyc-subway-main
git worktree remove ../nyc-subway-feature
```

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