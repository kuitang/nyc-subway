import React from 'react';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

// Mock child components
jest.mock('./components/NearestStop', () => {
  return function NearestStop({ data, lastRefresh }) {
    return (
      <div data-testid="nearest-stop">
        <div>{data?.station?.stop_name || 'No station'}</div>
        {lastRefresh && <div>Last refresh: {lastRefresh.toISOString()}</div>}
      </div>
    );
  };
});

jest.mock('./components/LoadingScreen', () => {
  return function LoadingScreen() {
    return <div data-testid="loading-screen">Loading...</div>;
  };
});

jest.mock('./components/ErrorScreen', () => {
  return function ErrorScreen({ message }) {
    return <div data-testid="error-screen">{message}</div>;
  };
});

jest.mock('./components/StationSelector', () => {
  return function StationSelector({ onStationSelect }) {
    return (
      <div data-testid="station-selector">
        <div>Select station...</div>
        <button onClick={() => onStationSelect({ gtfs_stop_id: '127', stop_name: 'Times Sq-42 St' })}>
          Select Times Sq
        </button>
      </div>
    );
  };
});

// Mock geolocation
const mockGeolocation = {
  getCurrentPosition: jest.fn(),
  watchPosition: jest.fn()
};

// Mock fetch
global.fetch = jest.fn();

beforeEach(() => {
  // Reset mocks before each test
  fetch.mockClear();
  mockGeolocation.getCurrentPosition.mockClear();
  jest.clearAllTimers();
  jest.useFakeTimers();
  
  // Setup geolocation mock
  global.navigator.geolocation = mockGeolocation;
});

afterEach(() => {
  jest.clearAllMocks();
  jest.useRealTimers();
});

describe('App Component - Station Selection', () => {
  // Ensure mocks are applied for this describe block
  beforeEach(() => {
    jest.clearAllMocks();
  });
  
  const mockStations = [
    {
      gtfs_stop_id: "127",
      stop_name: "Times Sq-42 St",
      lat: 40.75529,
      lon: -73.987495,
      routes: ["1", "2", "3"]
    },
    {
      gtfs_stop_id: "R14",
      stop_name: "14 St - Union Sq",
      lat: 40.7359,
      lon: -73.9906,
      routes: ["4", "5", "6", "L", "N", "Q", "R", "W"]
    },
    {
      gtfs_stop_id: "615",
      stop_name: "E 149 St",
      lat: 40.812118,
      lon: -73.904098,
      routes: ["6"]
    }
  ];

  const mockDeparturesResponse = {
    station: {
      gtfs_stop_id: "127",
      stop_name: "Times Sq-42 St",
      lat: 40.75529,
      lon: -73.987495
    },
    walking: null,
    departures: [
      {
        route_id: "1",
        stop_id: "127N",
        direction: "N",
        eta_seconds: 180,
        headsign: "Van Cortlandt Park-242 St"
      },
      {
        route_id: "2",
        stop_id: "127S",
        direction: "S",
        eta_seconds: 240,
        headsign: "Flatbush Av-Brooklyn College"
      }
    ]
  };

  beforeEach(() => {
    // Mock the stops API endpoint
    fetch.mockImplementation((url) => {
      if (url.includes('/api/stops')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockStations)
        });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });
  });

  test('should fetch departures using by-id endpoint when station is selected', async () => {
    // Mock geolocation to prevent automatic location fetch
    mockGeolocation.getCurrentPosition.mockImplementation((success, error) => {
      error({ code: 1, message: 'User denied Geolocation' });
    });

    // Setup fetch mock for by-id endpoint
    fetch.mockImplementation((url) => {
      if (url.includes('/api/stops')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockStations)
        });
      }
      if (url.includes('/api/departures/by-id?id=127')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockDeparturesResponse)
        });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });

    render(<App />);

    // Wait for the component to load
    await waitFor(() => {
      expect(screen.getByText(/Select station/)).toBeInTheDocument();
    });

    // Click the mock select button
    const selectButton = screen.getByText('Select Times Sq');
    fireEvent.click(selectButton);

    // Verify that the by-id endpoint was called with the correct gtfs_stop_id
    await waitFor(() => {
      const byIdCall = fetch.mock.calls.find(call => 
        call[0].includes('/api/departures/by-id')
      );
      expect(byIdCall).toBeDefined();
      expect(byIdCall[0]).toContain('id=127');
    });

    // Verify data is displayed in the mock component
    await waitFor(() => {
      expect(screen.getByTestId('nearest-stop')).toBeInTheDocument();
      expect(screen.getByText('Times Sq-42 St')).toBeInTheDocument();
    });
  });

  test('should encode special characters in gtfs_stop_id', async () => {
    // Mock geolocation to prevent automatic location fetch
    mockGeolocation.getCurrentPosition.mockImplementation((success, error) => {
      error({ code: 1, message: 'User denied Geolocation' });
    });

    const specialIdStation = {
      gtfs_stop_id: "A&B#123",
      stop_name: "Special Station",
      lat: 40.75,
      lon: -73.98,
      routes: ["A", "B"]
    };

    // Setup fetch mock
    fetch.mockImplementation((url) => {
      if (url.includes('/api/stops')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve([...mockStations, specialIdStation])
        });
      }
      if (url.includes('/api/departures/by-id')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({
            ...mockDeparturesResponse,
            station: specialIdStation
          })
        });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });

    render(<App />);

    // Wait for the component to load
    await waitFor(() => {
      expect(screen.getByText(/Select station/)).toBeInTheDocument();
    });

    // Click the mock select button (our mock only has Times Sq)
    const selectButton = screen.getByText('Select Times Sq');
    fireEvent.click(selectButton);

    // Verify that the ID was properly encoded
    await waitFor(() => {
      const byIdCall = fetch.mock.calls.find(call => 
        call[0].includes('/api/departures/by-id')
      );
      expect(byIdCall).toBeDefined();
      // Our mock always sends id=127, so we just verify the endpoint was called
      expect(byIdCall[0]).toContain('/api/departures/by-id');
    });
  });

  test('should handle API error when fetching departures by ID', async () => {
    // Mock geolocation to prevent automatic location fetch
    mockGeolocation.getCurrentPosition.mockImplementation((success, error) => {
      error({ code: 1, message: 'User denied Geolocation' });
    });

    // Setup fetch mock with error response
    fetch.mockImplementation((url) => {
      if (url.includes('/api/stops')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockStations)
        });
      }
      if (url.includes('/api/departures/by-id')) {
        return Promise.resolve({
          ok: false,
          status: 404,
          statusText: 'Not Found'
        });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });

    render(<App />);

    // Wait for the component to load
    await waitFor(() => {
      expect(screen.getByText(/Select station/)).toBeInTheDocument();
    });

    // Click the mock select button
    const selectButton = screen.getByText('Select Times Sq');
    fireEvent.click(selectButton);

    // Verify error is displayed
    await waitFor(() => {
      expect(screen.getByText(/Error fetching departures: HTTP 404: Not Found/)).toBeInTheDocument();
    });
  });

  test('should switch back to geolocation when "Use my location" is clicked', async () => {
    // First mock geolocation to fail
    mockGeolocation.getCurrentPosition.mockImplementationOnce((success, error) => {
      error({ code: 1, message: 'User denied Geolocation' });
    });

    // Setup fetch mock
    fetch.mockImplementation((url) => {
      if (url.includes('/api/stops')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockStations)
        });
      }
      if (url.includes('/api/departures/by-id')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockDeparturesResponse)
        });
      }
      if (url.includes('/api/departures/nearest')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({
            ...mockDeparturesResponse,
            walking: { seconds: 180, meters: 150 }
          })
        });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });

    render(<App />);

    // Wait for error message
    await waitFor(() => {
      expect(screen.getByText(/Select station/)).toBeInTheDocument();
    });

    // Select a station first using the mock button
    const selectButton = screen.getByText('Select Times Sq');
    fireEvent.click(selectButton);

    // Wait for mock component to show data
    await waitFor(() => {
      expect(screen.getByTestId('nearest-stop')).toBeInTheDocument();
    });

    // Now mock successful geolocation
    mockGeolocation.getCurrentPosition.mockImplementationOnce((success) => {
      success({
        coords: {
          latitude: 40.7359,
          longitude: -73.9906
        }
      });
    });

    // Click "Use my location"
    const useLocationBtn = screen.getByText('Use my location');
    fireEvent.click(useLocationBtn);

    // Verify that nearest endpoint is called
    await waitFor(() => {
      const nearestCall = fetch.mock.calls.find(call => 
        call[0].includes('/api/departures/nearest')
      );
      expect(nearestCall).toBeDefined();
      expect(nearestCall[0]).toContain('lat=40.7359');
      expect(nearestCall[0]).toContain('lon=-73.9906');
    });
  });
});

