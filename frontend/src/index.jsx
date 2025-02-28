import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App.jsx';
import { ThemeProvider } from './ThemeContext.jsx';
import config from './config';

// Import PatternFly 6 styles first
import '@patternfly/patternfly/patternfly.css';
import '@patternfly/patternfly/patternfly-addons.css';
// Import PatternFly charts CSS for dark theme support
import '@patternfly/patternfly/patternfly-charts.css';

// Import our custom styles last so they can override PatternFly styles if needed
import './index.css';

// Application configuration
console.log('App Name:', config.app.name);
console.log('App Version:', config.app.version);

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <React.StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </React.StrictMode>
); 