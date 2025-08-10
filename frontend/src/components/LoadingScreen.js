import React from 'react';
import './LoadingScreen.css';

function LoadingScreen() {
  return (
    <div className="loading-screen">
      <div className="loading-content">
        <div className="loading-spinner"></div>
        <h2>NYC Subway</h2>
        <p>Getting your location...</p>
      </div>
    </div>
  );
}

export default LoadingScreen;