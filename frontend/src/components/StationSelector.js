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

  const customStyles = {
    control: (provided, state) => ({
      ...provided,
      backgroundColor: '#1a1a1a',
      borderColor: state.isFocused ? '#4a4a4a' : '#2a2a2a',
      boxShadow: state.isFocused ? '0 0 0 1px #4a4a4a' : 'none',
      '&:hover': {
        borderColor: '#4a4a4a'
      }
    }),
    menu: (provided) => ({
      ...provided,
      backgroundColor: '#1a1a1a',
      border: '1px solid #2a2a2a'
    }),
    option: (provided, state) => ({
      ...provided,
      backgroundColor: state.isSelected ? '#3a3a3a' : state.isFocused ? '#2a2a2a' : '#1a1a1a',
      color: '#ffffff',
      cursor: 'pointer',
      '&:active': {
        backgroundColor: '#3a3a3a'
      }
    }),
    singleValue: (provided) => ({
      ...provided,
      color: '#ffffff'
    }),
    placeholder: (provided) => ({
      ...provided,
      color: '#666666'
    }),
    input: (provided) => ({
      ...provided,
      color: '#ffffff'
    }),
    dropdownIndicator: (provided) => ({
      ...provided,
      color: '#666666',
      '&:hover': {
        color: '#999999'
      }
    }),
    clearIndicator: (provided) => ({
      ...provided,
      color: '#666666',
      '&:hover': {
        color: '#999999'
      }
    })
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
        styles={customStyles}
        components={{
          Option: CustomOption,
          SingleValue: CustomSingleValue
        }}
        theme={(theme) => ({
          ...theme,
          colors: {
            ...theme.colors,
            primary: '#4a4a4a',
            primary25: '#2a2a2a',
            neutral0: '#1a1a1a',
            neutral80: '#ffffff'
          }
        })}
      />
    </div>
  );
}

export default StationSelector;