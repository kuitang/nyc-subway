import React, { useState, useEffect, useRef } from 'react';
import NearestStop from './components/NearestStop';
import LoadingScreen from './components/LoadingScreen';
import ErrorScreen from './components/ErrorScreen';
import StationSelector from './components/StationSelector';
import './App.css';

function App() {
  const [locationData, setLocationData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedStation, setSelectedStation] = useState(null);
  const [isUsingGeolocation, setIsUsingGeolocation] = useState(true);
  const [lastRefresh, setLastRefresh] = useState(new Date());
  const positionRef = useRef(null);

  useEffect(() => {
    // Only use geolocation if no station is manually selected
    if (!selectedStation && isUsingGeolocation) {
      if (navigator.geolocation) {
        navigator.geolocation.getCurrentPosition(
          (position) => {
            const { latitude, longitude } = position.coords;
            positionRef.current = { latitude, longitude };
            fetchNearestDepartures(latitude, longitude);
          },
          (err) => {
            setError('Location access denied or unavailable');
            setLoading(false);
          },
          { 
            enableHighAccuracy: true, 
            maximumAge: 30000, 
            timeout: 15000 
          }
        );
      } else {
        setError('Geolocation not supported in this browser');
        setLoading(false);
      }
    } else if (selectedStation) {
      // Fetch departures for manually selected station
      fetchDeparturesByStation(selectedStation);
    } else {
      // No station selected and geolocation disabled
      setLoading(false);
    }
  }, [selectedStation, isUsingGeolocation]);

  // Set up 30-second refresh interval after data loads
  useEffect(() => {
    // Only set up interval if we have data and no error
    if (!locationData || error) {
      return;
    }

    // Set up interval
    const id = setInterval(() => {
      // Refresh based on current state
      if (selectedStation) {
        // Refresh selected station
        fetchDeparturesByStation(selectedStation, true);
      } else if (isUsingGeolocation && positionRef.current) {
        // Refresh using stored geolocation
        const { latitude, longitude } = positionRef.current;
        fetchNearestDepartures(latitude, longitude, true);
      }
    }, 30000); // 30 seconds

    // Cleanup on unmount or dependency change
    return () => {
      clearInterval(id);
    };
    // Dependencies include locationData which will reset the interval on each refresh
    // This is a known issue but ensures reliable operation
    // eslint-disable-next-line
  }, [locationData, selectedStation, isUsingGeolocation, error]);

  const fetchNearestDepartures = async (lat, lon, isRefresh = false) => {
    const apiBaseUrl = process.env.REACT_APP_API_BASE_URL || 'http://localhost:8080';
    
    // Don't show loading state during refresh to avoid UI flashing
    if (!isRefresh) {
      setLoading(true);
    }
    
    try {
      const response = await fetch(
        `${apiBaseUrl}/api/departures/nearest?lat=${encodeURIComponent(lat)}&lon=${encodeURIComponent(lon)}`
      );
      
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      
      const data = await response.json();
      
      if (data.error) {
        throw new Error(data.error);
      }
      
      setLocationData(data);
      setLoading(false);
      setLastRefresh(new Date());
    } catch (err) {
      // During refresh, don't update error state if fetch fails - keep showing old data
      if (!isRefresh) {
        setError(`Error fetching departures: ${err.message}`);
        setLoading(false);
      } else {
        console.error('Refresh failed:', err.message);
      }
    }
  };

  const fetchDeparturesByStation = async (station, isRefresh = false) => {
    const apiBaseUrl = process.env.REACT_APP_API_BASE_URL || 'http://localhost:8080';
    
    // Don't show loading state during refresh to avoid UI flashing
    if (!isRefresh) {
      setLoading(true);
      setError(null);
    }
    
    try {
      const response = await fetch(
        `${apiBaseUrl}/api/departures/by-id?id=${encodeURIComponent(station.gtfs_stop_id)}`
      );
      
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      
      const data = await response.json();
      
      if (data.error) {
        throw new Error(data.error);
      }
      
      setLocationData(data);
      setLoading(false);
      setLastRefresh(new Date());
    } catch (err) {
      // During refresh, don't update error state if fetch fails - keep showing old data
      if (!isRefresh) {
        setError(`Error fetching departures: ${err.message}`);
        setLoading(false);
      } else {
        console.error('Refresh failed:', err.message);
      }
    }
  };

  const handleStationSelect = (station) => {
    setSelectedStation(station);
    if (station) {
      setIsUsingGeolocation(false);
    }
  };

  const handleUseLocation = () => {
    setSelectedStation(null);
    setIsUsingGeolocation(true);
    setError(null);
  };

  if (loading && !selectedStation && isUsingGeolocation) {
    return <LoadingScreen />;
  }

  return (
    <div className="App">
      <div className="app-header">
        <StationSelector 
          onStationSelect={handleStationSelect}
          currentStation={selectedStation}
        />
        {!isUsingGeolocation && (
          <button 
            className="use-location-btn"
            onClick={handleUseLocation}
          >
            Use my location
          </button>
        )}
      </div>
      
      {loading ? (
        <LoadingScreen />
      ) : error ? (
        <ErrorScreen message={error} />
      ) : locationData ? (
        <NearestStop data={locationData} lastRefresh={lastRefresh} />
      ) : (
        <div className="no-station-message">
          Select a station or enable location access to see departures
        </div>
      )}
    </div>
  );
}

export default App;