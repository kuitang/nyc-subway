import React, { useState, useEffect } from 'react';
import { getLineColor } from '../constants/subwayColors';
import './NearestStop.css';

function NearestStop({ data }) {
  const [currentTime, setCurrentTime] = useState(new Date());

  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentTime(new Date());
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  if (!data || !data.station) {
    return null;
  }

  const { station, walking, departures } = data;
  const walkTimeMinutes = walking ? Math.ceil(walking.seconds / 60) : null;

  const formatDateTime = () => {
    const options = { 
      weekday: 'long', 
      year: 'numeric', 
      month: 'long', 
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit'
    };
    return currentTime.toLocaleDateString('en-US', options);
  };
  
  const formatWalkTime = () => {
    if (!walkTimeMinutes) return '';
    return `🚶 ${walkTimeMinutes} min walk`;
  };

  const formatETA = (etaMinutes) => {
    if (etaMinutes === 0) return 'Now';
    return `${etaMinutes} min`;
  };

  const getDepartureTimeClass = (etaMinutes, walkTimeMinutes) => {
    if (walkTimeMinutes && etaMinutes < walkTimeMinutes) {
      return 'departure-time--red';
    }
    if (etaMinutes > 20) {
      return 'departure-time--red';
    }
    if (etaMinutes < 10) {
      return 'departure-time--good';
    }
    return '';
  };

  const getDirectionText = (direction, headsign) => {
    // Prioritize headsign if available and not empty
    if (headsign && headsign.trim() !== '') {
      return headsign;
    }
    
    // Fallback to direction-based text
    const directionMap = {
      'N': 'Northbound',
      'S': 'Southbound', 
      'E': 'Eastbound',
      'W': 'Westbound'
    };
    return directionMap[direction] || direction || '';
  };

  // Group departures by route, direction, and headsign for better display
  const groupedDepartures = departures.reduce((acc, departure) => {
    // Use headsign in grouping key if available, otherwise use direction
    const displayKey = departure.headsign && departure.headsign.trim() !== '' 
      ? departure.headsign 
      : departure.direction;
    const key = `${departure.route_id}_${displayKey}`;
    
    if (!acc[key]) {
      acc[key] = {
        route_id: departure.route_id,
        direction: departure.direction,
        headsign: departure.headsign,
        departures: []
      };
    }
    acc[key].departures.push(departure);
    return acc;
  }, {});

  return (
    <div className="nearest-stop">
      <div className="current-time">{formatDateTime()}</div>
      <div className="station-header">
        <h1 className="station-name">{station.stop_name}</h1>
        {walking && (
          <span className="walk-time">
            {formatWalkTime()}
          </span>
        )}
      </div>

      <div className="departures-list">
        {Object.values(groupedDepartures).map((group, index) => (
          <div key={index} className="departure-group">
            <div className="route-header">
              <div 
                className="line-circle"
                style={{ backgroundColor: getLineColor(group.route_id) }}
              >
                {group.route_id}
              </div>
              <span className="direction-text">
                {getDirectionText(group.direction, group.headsign)}
              </span>
            </div>
            
            <div className="departure-times">
              {group.departures.slice(0, 2).map((departure, depIndex) => {
                const etaMinutes = Math.floor(departure.eta_seconds / 60);
                return (
                  <div 
                    key={depIndex} 
                    className={`departure-time ${getDepartureTimeClass(etaMinutes, walkTimeMinutes)}`}
                  >
                    {formatETA(etaMinutes)}
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>

      {departures.length === 0 && (
        <div className="no-departures">
          <p>No departures found for this station</p>
        </div>
      )}
    </div>
  );
}

export default NearestStop;