/**
 * Application configuration
 * This file contains all configuration parameters for the application
 */

// Determine if we're in development mode based on URL or another method
// For local development we can hardcode this to true
const isDevelopment = window.location.hostname === 'localhost' || 
                       window.location.hostname === '127.0.0.1';

const config = {
  // Application information
  app: {
    name: 'Monokit',
    version: '0.1.0',
  },
  
  // API configuration
  api: {
    baseUrl: '/api/v1',
    timeout: 30000, // 30 seconds
  },
  
  // Environment
  environment: {
    isDevelopment: isDevelopment,
    isProduction: !isDevelopment,
    mockAuthEnabled: isDevelopment,
    tokenDuration: isDevelopment ? 24 * 60 * 60 * 1000 : null, // 24 hours in ms for dev
  },
  
  // Feature flags
  features: {
    darkMode: true,
    notifications: true,
  },
  
  // Development settings
  development: {
    enableLogging: true,
    mockApi: false,
  }
};

export default config; 