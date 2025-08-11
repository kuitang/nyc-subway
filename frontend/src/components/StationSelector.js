import React, { useState, useEffect } from 'react';
import Select, { components } from 'react-select';
import { getLineColor } from '../constants/subwayColors';
import './StationSelector.css';

function StationSelector({ onStationSelect, currentStation }) {
  const [stations, setStations] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchStations();
  }, []);

  const fetchStations = async () => {
    const apiBaseUrl = process.env.REACT_APP_API_BASE_URL || 'http://localhost:8080';
    try {
      const response = await fetch(`${apiBaseUrl}/api/stops`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      const data = await response.json();
      
      // Transform and sort stations for react-select
      const options = data
        .map(station => ({
          value: station.stop_name,
          label: station.stop_name,
          data: station
        }))
        .sort((a, b) => a.label.localeCompare(b.label));
      
      setStations(options);
      setLoading(false);
    } catch (err) {
      console.error('Error fetching stations:', err);
      setLoading(false);
    }
  };

  // Custom option component to display route circles
  const CustomOption = (props) => {
    const { data } = props;
    return (
      <components.Option {...props}>
        <div className="station-option-content">
          {data.data.routes && data.data.routes.length > 0 && (
            <span className="station-routes-container">
              {data.data.routes.map((route, index) => (
                <span
                  key={index}
                  className="route-circle dropdown-route"
                  style={{ backgroundColor: getLineColor(route) }}
                >
                  {route}
                </span>
              ))}
            </span>
          )}
          <span>{props.children}</span>
        </div>
      </components.Option>
    );
  };

  // Custom single value component to display route circles
  const CustomSingleValue = (props) => {
    const { data } = props;
    return (
      <components.SingleValue {...props}>
        <div className="station-option-content">
          {data.data.routes && data.data.routes.length > 0 && (
            <span className="station-routes-container">
              {data.data.routes.map((route, index) => (
                <span
                  key={index}
                  className="route-circle dropdown-route"
                  style={{ backgroundColor: getLineColor(route) }}
                >
                  {route}
                </span>
              ))}
            </span>
          )}
          <span>{props.children}</span>
        </div>
      </components.SingleValue>
    );
  };

  const handleChange = (selectedOption) => {
    if (selectedOption) {
      onStationSelect(selectedOption.data);
    } else {
      onStationSelect(null);
    }
  };

  const currentValue = currentStation 
    ? stations.find(s => s.value === currentStation.stop_name)
    : null;

  return (
    <div className="station-selector">
      <Select
        options={stations}
        value={currentValue}
        onChange={handleChange}
        placeholder="Select station..."
        isLoading={loading}
        isClearable
        isSearchable
        classNamePrefix="station-select"
        components={{
          Option: CustomOption,
          SingleValue: CustomSingleValue
        }}
      />
    </div>
  );
}

export default StationSelector;