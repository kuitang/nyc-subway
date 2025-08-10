import React, { useState, useEffect } from 'react';
import NearestStop from './components/NearestStop';
import LoadingScreen from './components/LoadingScreen';
import ErrorScreen from './components/ErrorScreen';
import './App.css';

function App() {
  const [locationData, setLocationData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
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
  }, []);

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

  if (loading) {
    return <LoadingScreen />;
  }

  if (error) {
    return <ErrorScreen message={error} />;
  }

  return (
    <div className="App">
      <NearestStop data={locationData} />
    </div>
  );
}

export default App;