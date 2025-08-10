// NYC Subway Line Colors - Official MTA Colors
// Based on the NYC Transit Authority Graphics Standards Manual
// 
// Sources:
// - http://web.mta.info/developers/resources/line_colors.htm
// - https://data.ny.gov/Transportation/MTA-Colors/3uhz-sej2
// - https://www.mta.info/document/168976 (MTA Brand Colors / Subway, SIR & ADA)
// - Original Vignelli/Noorda Graphics Standards Manual (1970)

export const SUBWAY_COLORS = {
  // Broadway-Seventh Avenue Line (Red)
  '1': '#EE352E',
  '2': '#EE352E', 
  '3': '#EE352E',
  
  // Lexington Avenue Line (Green)
  '4': '#00933C',
  '5': '#00933C',
  '6': '#00933C',
  
  // Flushing Line (Purple)
  '7': '#B933AD',
  
  // Eighth Avenue Line (Blue)
  'A': '#0039A6',
  'C': '#0039A6', 
  'E': '#0039A6',
  
  // Sixth Avenue Line (Orange)
  'B': '#FF6319',
  'D': '#FF6319',
  'F': '#FF6319',
  'M': '#FF6319',
  
  // Crosstown Line (Light Green)
  'G': '#6CBE45',
  
  // Canarsie Line (Gray)
  'L': '#A7A9AC',
  
  // Broadway Line (Yellow)
  'N': '#FCCC0A',
  'Q': '#FCCC0A',
  'R': '#FCCC0A',
  'W': '#FCCC0A',
  
  // Nassau Street Line (Brown)
  'J': '#996633',
  'Z': '#996633',
  
  // Staten Island Railway (Blue - same as A/C/E)
  'SIR': '#0039A6'
};

// Get color for any subway line
export const getLineColor = (lineId) => {
  return SUBWAY_COLORS[lineId] || '#6F6F6F'; // Default gray for unknown lines
};

// MTA Brand Colors
// Source: https://www.mta.info/document/168976
export const MTA_COLORS = {
  black: '#000000',
  white: '#FFFFFF',
  darkGray: '#53565A',
  lightGray: '#C8C9CA',
  blue: '#0039A6'  // Official MTA blue
};