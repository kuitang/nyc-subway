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

## Deployment to Fly.io

### Initial Setup (One-time)

1. **Install Fly CLI locally:**
```bash
curl -L https://fly.io/install.sh | sh
export PATH="$HOME/.fly/bin:$PATH"
```

2. **Create a Fly.io account:**
```bash
flyctl auth signup
# Or if you already have an account:
flyctl auth login
```

3. **Create the apps on Fly.io:**
```bash
# Create backend app
cd backend
flyctl apps create nyc-subway-backend --region ewr

# Create frontend app
cd ../frontend
flyctl apps create nyc-subway-frontend --region ewr
```

4. **Get your Fly.io API token for GitHub Actions:**
```bash
flyctl auth token
```
Copy this token - you'll need it for GitHub secrets.

5. **Add the token to GitHub repository secrets:**
   - Go to your GitHub repository → Settings → Secrets and variables → Actions
   - Click "New repository secret"
   - Name: `FLY_API_TOKEN`
   - Value: Paste the token from step 4
   - Click "Add secret"

### Deploy Manually (Optional)

```bash
# Deploy backend
cd backend
flyctl deploy

# Deploy frontend
cd frontend
flyctl deploy
```

### Automatic Deployment via GitHub Actions

The app will automatically deploy when you push to the `main` branch. The GitHub Actions workflow:
1. Deploys the backend first
2. Then deploys the frontend (configured to use the backend URL)

You can also trigger deployment manually from GitHub:
- Go to Actions tab → Deploy to Fly.io → Run workflow

### View Your Deployed Apps

After deployment, your apps will be available at:
- Backend: https://nyc-subway-backend.fly.dev
- Frontend: https://nyc-subway-frontend.fly.dev

### Monitor Your Apps

```bash
# View backend logs
flyctl logs -a nyc-subway-backend

# View frontend logs
flyctl logs -a nyc-subway-frontend

# Check app status
flyctl status -a nyc-subway-backend
flyctl status -a nyc-subway-frontend

# SSH into running app (for debugging)
flyctl ssh console -a nyc-subway-backend
```

### Scaling and Configuration

```bash
# Scale app instances
flyctl scale count 2 -a nyc-subway-backend

# View current configuration
flyctl config show -a nyc-subway-backend

# Update environment variables
flyctl secrets set KEY=value -a nyc-subway-backend
```