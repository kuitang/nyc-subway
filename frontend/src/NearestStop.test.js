import React from 'react';
import { render, screen, waitFor, act } from '@testing-library/react';
import NearestStop from './components/NearestStop';

beforeEach(() => {
  jest.clearAllMocks();
  jest.useFakeTimers();
  global.fetch = jest.fn();
});

afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});

describe('NearestStop Refresh Counter', () => {
  const mockDepartures = [
    {
      route_id: 'L',
      direction: 'N',
      headsign: 'Manhattan',
      eta_seconds: 120,
      is_delayed: false,
      is_assigned: true
    }
  ];

  const mockData = {
    station: {
      stop_name: 'Union Sq - 14 St',
      routes: ['L', 'N', 'Q', 'R', 'W', '4', '5', '6']
    },
    walking: {
      seconds: 300
    },
    departures: mockDepartures
  };

  test('displays "updated 29s ago" at 29 seconds without triggering refresh', async () => {
    const lastRefresh = new Date();
    
    render(<NearestStop data={mockData} lastRefresh={lastRefresh} />);

    expect(screen.getByText(/Union Sq - 14 St/)).toBeInTheDocument();

    act(() => {
      jest.advanceTimersByTime(29000);
    });

    expect(screen.getByText(/updated 29s ago/)).toBeInTheDocument();
  });


  test('displays "just updated" for first 4 seconds after load', async () => {
    const lastRefresh = new Date();
    
    render(<NearestStop data={mockData} lastRefresh={lastRefresh} />);

    expect(screen.getByText(/Union Sq - 14 St/)).toBeInTheDocument();
    expect(screen.getByText(/just updated/)).toBeInTheDocument();

    act(() => {
      jest.advanceTimersByTime(3000);
    });

    expect(screen.getByText(/just updated/)).toBeInTheDocument();

    act(() => {
      jest.advanceTimersByTime(2000);
    });

    expect(screen.getByText(/updated 5s ago/)).toBeInTheDocument();
  });

  test('shows correct time format at 30 seconds', async () => {
    const lastRefresh = new Date();
    const { rerender } = render(<NearestStop data={mockData} lastRefresh={lastRefresh} />);

    expect(screen.getByText(/just updated/)).toBeInTheDocument();

    act(() => {
      jest.advanceTimersByTime(30000);
    });
    expect(screen.getByText(/updated 30s ago/)).toBeInTheDocument();
  });

  test('renders departure information correctly', async () => {
    const lastRefresh = new Date();
    
    render(<NearestStop data={mockData} lastRefresh={lastRefresh} />);

    expect(screen.getByText(/Union Sq - 14 St/)).toBeInTheDocument();
    expect(screen.getByText(/5 min walk/)).toBeInTheDocument();
    // The L line appears multiple times in the UI (station routes and departure), use getAllByText
    const lElements = screen.getAllByText('L');
    expect(lElements.length).toBeGreaterThan(0);
    expect(screen.getByText(/Manhattan/)).toBeInTheDocument();
    expect(screen.getByText(/2 min/)).toBeInTheDocument();
  });
});