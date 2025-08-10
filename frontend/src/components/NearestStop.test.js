import React from 'react';
import { render, screen } from '@testing-library/react';
import NearestStop from './NearestStop';

// Test data
const mockStation = {
  gtfs_stop_id: "R14N",
  stop_name: "14 St - Union Sq",
  lat: 40.7359,
  lon: -73.9906
};

const mockWalking = {
  seconds: 180,
  meters: 150
};

describe('NearestStop Component', () => {
  test('should display headsign when available', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "6",
          stop_id: "R14N", 
          direction: "N",
          eta_minutes: 3,
          headsign: "Times Sq-42 St"
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    // Should display the headsign instead of direction
    expect(screen.getByText("Times Sq-42 St")).toBeInTheDocument();
    // Should NOT display the direction fallback
    expect(screen.queryByText("Northbound")).not.toBeInTheDocument();
  });

  test('should fallback to Northbound when headsign is empty', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "6",
          stop_id: "R14N",
          direction: "N", 
          eta_minutes: 3,
          headsign: ""
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    // Should display direction fallback
    expect(screen.getByText("Northbound")).toBeInTheDocument();
  });

  test('should fallback to Southbound when headsign is empty', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "Q", 
          stop_id: "R14S",
          direction: "S",
          eta_minutes: 5,
          headsign: ""
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    expect(screen.getByText("Southbound")).toBeInTheDocument();
  });

  test('should fallback to Eastbound when headsign is empty', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "L",
          stop_id: "R14E", 
          direction: "E",
          eta_minutes: 2,
          headsign: ""
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    expect(screen.getByText("Eastbound")).toBeInTheDocument();
  });

  test('should fallback to Westbound when headsign is empty', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "L",
          stop_id: "R14W",
          direction: "W", 
          eta_minutes: 7,
          headsign: ""
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    expect(screen.getByText("Westbound")).toBeInTheDocument();
  });

  test('should handle missing headsign field (undefined)', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_minutes: 3
          // headsign field is missing/undefined
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    // Should fallback to direction when headsign is undefined
    expect(screen.getByText("Northbound")).toBeInTheDocument();
  });

  test('should display empty string when both headsign and direction are empty', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "6",
          stop_id: "456", 
          direction: "",
          eta_minutes: 3,
          headsign: ""
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    // Should find the route but not display any direction text
    expect(screen.getByText("6")).toBeInTheDocument();
    expect(screen.queryByText(/bound/)).not.toBeInTheDocument();
  });

  test('should prioritize headsign over direction when both are present', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "N",
          stop_id: "R14S",
          direction: "S",
          eta_minutes: 4,
          headsign: "Coney Island-Stillwell Av"
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    // Should display headsign, not direction
    expect(screen.getByText("Coney Island-Stillwell Av")).toBeInTheDocument();
    expect(screen.queryByText("Southbound")).not.toBeInTheDocument();
  });

  test('should handle multiple departures with mixed headsign availability', () => {
    const mockData = {
      station: mockStation,
      walking: mockWalking,
      departures: [
        {
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_minutes: 2,
          headsign: "Times Sq-42 St"
        },
        {
          route_id: "6", 
          stop_id: "R14S",
          direction: "S",
          eta_minutes: 5,
          headsign: ""
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    // Should display both: headsign for first, direction for second
    expect(screen.getByText("Times Sq-42 St")).toBeInTheDocument();
    expect(screen.getByText("Southbound")).toBeInTheDocument();
  });
});