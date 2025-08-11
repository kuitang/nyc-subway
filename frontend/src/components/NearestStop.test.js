import React from 'react';
import { render, screen, cleanup } from '@testing-library/react';
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
          eta_seconds: 180,
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
          eta_seconds: 180,
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
          eta_seconds: 300,
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
          eta_seconds: 120,
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
          eta_seconds: 420,
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
          eta_seconds: 180
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
          eta_seconds: 180,
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
          eta_seconds: 240,
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
          eta_seconds: 120,
          headsign: "Times Sq-42 St"
        },
        {
          route_id: "6", 
          stop_id: "R14S",
          direction: "S",
          eta_seconds: 300,
          headsign: ""
        }
      ]
    };

    render(<NearestStop data={mockData} />);
    
    // Should display both: headsign for first, direction for second
    expect(screen.getByText("Times Sq-42 St")).toBeInTheDocument();
    expect(screen.getByText("Southbound")).toBeInTheDocument();
  });

  describe('Departure Time Color Coding', () => {
    test('should apply red class when departure is less than walk time', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 300, meters: 250 }, // 5 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 180, // Less than 5 min walk
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("3 min");
      expect(departureTime).toHaveClass('departure-time--red');
      expect(departureTime).not.toHaveClass('departure-time--good');
    });

    test('should apply red class when departure is more than 20 minutes', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 180, meters: 150 }, // 3 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 1260, // 21 minutes
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("21 min");
      expect(departureTime).toHaveClass('departure-time--red');
      expect(departureTime).not.toHaveClass('departure-time--good');
    });

    test('should not apply red class when departure is exactly 20 minutes', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 180, meters: 150 }, // 3 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 1200, // Exactly 20 minutes
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("20 min");
      expect(departureTime).toHaveClass('departure-time');
      expect(departureTime).not.toHaveClass('departure-time--red');
      expect(departureTime).not.toHaveClass('departure-time--good');
    });

    test('should apply good class when departure is less than 10 minutes but more than walk time', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 180, meters: 150 }, // 3 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 420, // More than 3 min walk, less than 10 min
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("7 min");
      expect(departureTime).toHaveClass('departure-time--good');
      expect(departureTime).not.toHaveClass('departure-time--red');
    });

    test('should apply no special class when departure is 10 minutes or more', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 180, meters: 150 }, // 3 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 900, // More than 10 min
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("15 min");
      expect(departureTime).toHaveClass('departure-time');
      expect(departureTime).not.toHaveClass('departure-time--good');
      expect(departureTime).not.toHaveClass('departure-time--red');
    });

    test('should handle edge case when departure equals walk time', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 300, meters: 250 }, // 5 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 300, // Exactly equal to walk time
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("5 min");
      // Should apply good class (not red) since it's not less than walk time
      expect(departureTime).toHaveClass('departure-time--good');
      expect(departureTime).not.toHaveClass('departure-time--red');
    });

    test('should handle case with no walking data', () => {
      const mockData = {
        station: mockStation,
        walking: null, // No walking data
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 180,
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("3 min");
      // Should apply good class since eta < 10 and no walk time to compare
      expect(departureTime).toHaveClass('departure-time--good');
      expect(departureTime).not.toHaveClass('departure-time--red');
    });

    test('should handle "Now" departure correctly', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 300, meters: 250 }, // 5 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 0, // Now
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const departureTime = screen.getByText("Now");
      // Should apply red class since 0 < walk time (5)
      expect(departureTime).toHaveClass('departure-time--red');
      expect(departureTime).not.toHaveClass('departure-time--good');
    });

    test('should apply different classes to multiple departures correctly', () => {
      const mockData = {
        station: mockStation,
        walking: { seconds: 240, meters: 200 }, // 4 minutes walk
        departures: [
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 120, // Less than walk time
            headsign: "Times Sq-42 St"
          },
          {
            route_id: "6",
            stop_id: "R14N",
            direction: "N",
            eta_seconds: 360, // More than walk time, less than 10
            headsign: "Times Sq-42 St"
          }
        ]
      };

      render(<NearestStop data={mockData} />);
      
      const firstDeparture = screen.getByText("2 min");
      const secondDeparture = screen.getByText("6 min");
      
      expect(firstDeparture).toHaveClass('departure-time--red');
      expect(secondDeparture).toHaveClass('departure-time--good');
    });
  });

  describe('Minute Calculation Rounding', () => {
    test('should round up walk time (Math.ceil)', () => {
      const mockDataWithWalk90s = {
        station: mockStation,
        walking: { seconds: 90, meters: 75 }, // 90 seconds should round up to 2 minutes
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 600,
          headsign: "Times Sq-42 St"
        }]
      };

      const mockDataWithWalk61s = {
        station: mockStation,
        walking: { seconds: 61, meters: 50 }, // 61 seconds should round up to 2 minutes
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 600,
          headsign: "Times Sq-42 St"
        }]
      };

      const mockDataWithWalk60s = {
        station: mockStation,
        walking: { seconds: 60, meters: 50 }, // 60 seconds should be exactly 1 minute
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 600,
          headsign: "Times Sq-42 St"
        }]
      };

      const mockDataWithWalk59s = {
        station: mockStation,
        walking: { seconds: 59, meters: 45 }, // 59 seconds should round up to 1 minute
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 600,
          headsign: "Times Sq-42 St"
        }]
      };

      // Test 90 seconds = 2 minutes (rounded up)
      render(<NearestStop data={mockDataWithWalk90s} />);
      expect(screen.getByText("🚶 2 min walk")).toBeInTheDocument();
      cleanup();

      // Test 61 seconds = 2 minutes (rounded up)
      render(<NearestStop data={mockDataWithWalk61s} />);
      expect(screen.getByText("🚶 2 min walk")).toBeInTheDocument();
      cleanup();

      // Test 60 seconds = 1 minute (exact)
      render(<NearestStop data={mockDataWithWalk60s} />);
      expect(screen.getByText("🚶 1 min walk")).toBeInTheDocument();
      cleanup();

      // Test 59 seconds = 1 minute (rounded up)
      render(<NearestStop data={mockDataWithWalk59s} />);
      expect(screen.getByText("🚶 1 min walk")).toBeInTheDocument();
    });

    test('should round down ETA time (Math.floor)', () => {
      const mockData90s = {
        station: mockStation,
        walking: mockWalking,
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 90, // 90 seconds should round down to 1 minute
          headsign: "Times Sq-42 St"
        }]
      };

      const mockData119s = {
        station: mockStation,
        walking: mockWalking,
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 119, // 119 seconds should round down to 1 minute
          headsign: "Times Sq-42 St"
        }]
      };

      const mockData120s = {
        station: mockStation,
        walking: mockWalking,
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 120, // 120 seconds should be exactly 2 minutes
          headsign: "Times Sq-42 St"
        }]
      };

      const mockData59s = {
        station: mockStation,
        walking: mockWalking,
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 59, // 59 seconds should round down to 0 minutes
          headsign: "Times Sq-42 St"
        }]
      };

      // Test 90 seconds = 1 minute (rounded down)
      render(<NearestStop data={mockData90s} />);
      expect(screen.getByText("1 min")).toBeInTheDocument();
      cleanup();

      // Test 119 seconds = 1 minute (rounded down)
      render(<NearestStop data={mockData119s} />);
      expect(screen.getByText("1 min")).toBeInTheDocument();
      cleanup();

      // Test 120 seconds = 2 minutes (exact)
      render(<NearestStop data={mockData120s} />);
      expect(screen.getByText("2 min")).toBeInTheDocument();
      cleanup();

      // Test 59 seconds = "Now" (rounded down to 0)
      render(<NearestStop data={mockData59s} />);
      expect(screen.getByText("Now")).toBeInTheDocument();
    });

    test('should ensure walk time is always overestimated and ETA is always underestimated', () => {
      // Edge case: 61 second walk, 119 second ETA
      // Walk should round up to 2 min, ETA should round down to 1 min
      // This ensures we overestimate walk time and underestimate ETA
      const mockData = {
        station: mockStation,
        walking: { seconds: 61, meters: 50 }, // Rounds up to 2 min
        departures: [{
          route_id: "6",
          stop_id: "R14N",
          direction: "N",
          eta_seconds: 119, // Rounds down to 1 min
          headsign: "Times Sq-42 St"
        }]
      };

      render(<NearestStop data={mockData} />);
      
      // Walk time should be 2 minutes (overestimated)
      expect(screen.getByText("🚶 2 min walk")).toBeInTheDocument();
      
      // ETA should be 1 minute (underestimated)
      expect(screen.getByText("1 min")).toBeInTheDocument();
      
      // This departure should be marked as too late since 1 < 2
      const departureTime = screen.getByText("1 min");
      expect(departureTime).toHaveClass('departure-time--red');
    });
  });
});