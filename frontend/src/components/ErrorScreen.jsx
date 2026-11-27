import React from 'react';
import './ErrorScreen.css';

function ErrorScreen({ message }) {
  return (
    <div className="error-screen">
      <div className="error-content">
        <div className="error-icon">⚠️</div>
        <h2>NYC Subway</h2>
        <p className="error-message">{message}</p>
        <button 
          className="retry-button" 
          onClick={() => window.location.reload()}
        >
          Try Again
        </button>
      </div>
    </div>
  );
}

export default ErrorScreen;