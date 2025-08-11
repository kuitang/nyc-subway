import React, { useState, useEffect } from 'react';
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

  useEffect(() => {
    // Only use geolocation if no station is manually selected
    if (!selectedStation && isUsingGeolocation) {
      if (navigator.geolocation) {
        navigator.geolocation.getCurrentPosition(
          (position) => {
            const { latitude, longitude } = position.coords;
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

  const fetchNearestDepartures = async (lat, lon) => {
    const apiBaseUrl = process.env.REACT_APP_API_BASE_URL || 'http://localhost:8080';
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
    } catch (err) {
      setError(`Error fetching departures: ${err.message}`);
      setLoading(false);
    }
  };

  const fetchDeparturesByStation = async (station) => {
    const apiBaseUrl = process.env.REACT_APP_API_BASE_URL || 'http://localhost:8080';
    setLoading(true);
    setError(null);
    
    try {
      const response = await fetch(
        `${apiBaseUrl}/api/departures/by-name?name=${encodeURIComponent(station.stop_name)}`
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
    } catch (err) {
      setError(`Error fetching departures: ${err.message}`);
      setLoading(false);
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
        <NearestStop data={locationData} />
      ) : (
        <div className="no-station-message">
          Select a station or enable location access to see departures
        </div>
      )}
    </div>
  );
}

export default App;