/**
 * Authentication utility functions
 */

import config from '../config';

// For development, set this to a very long expiration time (e.g., days)
const DEV_TOKEN_DURATION = 30 * 24 * 60 * 60 * 1000; // 30 days in milliseconds

/**
 * Checks if the token is expired
 * 
 * @param {string} token - JWT token
 * @returns {boolean} - True if token is expired, false otherwise
 */
export const isTokenExpired = (token) => {
  if (!token) return true;
  
  // In development, return false to keep user logged in
  if (config.environment.isDevelopment) {
    console.log('Development mode: Token never expires');
    return false;
  }
  
  try {
    // First check if the token looks like a JWT (has 3 parts separated by dots)
    if (!token.includes('.') || token.split('.').length !== 3) {
      console.warn('Token does not appear to be a valid JWT');
      return false; // Assume not expired for non-JWT tokens
    }
    
    // For JWT tokens, parse the expiration time
    const base64Url = token.split('.')[1];
    
    // Fix base64 encoding issues by replacing characters and padding
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
    
    try {
      // Try to decode using safe approach
      const jsonPayload = decodeURIComponent(
        atob(base64)
          .split('')
          .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
          .join('')
      );
      
      const payload = JSON.parse(jsonPayload);
      const exp = payload.exp * 1000; // Convert to milliseconds
      
      return Date.now() >= exp;
    } catch (decodeError) {
      console.warn('Error decoding token payload:', decodeError);
      return false; // Assume not expired if we can't decode
    }
  } catch (error) {
    console.warn('Error checking token expiration:', error);
    // If we can't parse the token, assume it's not expired
    return false;
  }
};

/**
 * Gets the token expiration time in milliseconds
 * 
 * @param {string} token - JWT token
 * @returns {number} - Time until expiration in milliseconds
 */
export const getTokenExpirationTime = (token) => {
  if (!token) return 0;
  
  // In development, return a long duration
  if (config.environment.isDevelopment) {
    return config.environment.tokenDuration || DEV_TOKEN_DURATION;
  }
  
  try {
    // First check if the token looks like a JWT (has 3 parts separated by dots)
    if (!token.includes('.') || token.split('.').length !== 3) {
      console.warn('Token does not appear to be a JWT, returning default expiration');
      return DEV_TOKEN_DURATION; // For non-JWT tokens, return a default value
    }
    
    // For JWT tokens, calculate time until expiration
    const base64Url = token.split('.')[1];
    
    // Fix base64 encoding issues by replacing characters and padding
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
    
    try {
      // Try to decode using safe approach
      const jsonPayload = decodeURIComponent(
        atob(base64)
          .split('')
          .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
          .join('')
      );
      
      const payload = JSON.parse(jsonPayload);
      const exp = payload.exp * 1000; // Convert to milliseconds
      
      return Math.max(0, exp - Date.now());
    } catch (decodeError) {
      console.warn('Error decoding token payload:', decodeError);
      return DEV_TOKEN_DURATION; // Return default for decode errors
    }
  } catch (error) {
    console.warn('Error getting token expiration time:', error);
    // If we can't parse the token, return a default expiration time
    return DEV_TOKEN_DURATION;
  }
};

/**
 * Sets up authentication headers for axios
 * 
 * @param {object} axiosInstance - Axios instance
 */
export const setupAuthHeaders = (axiosInstance) => {
  const token = localStorage.getItem('token');
  
  if (token) {
    // Check if this is likely a JWT token (Keycloak tokens are JWTs)
    const isJWT = token.includes('.') && token.split('.').length === 3 && token.length > 100;
    
    // If it looks like a JWT (Keycloak token), add Bearer prefix
    if (isJWT) {
      axiosInstance.defaults.headers.common['Authorization'] = `Bearer ${token}`;
    } else {
      axiosInstance.defaults.headers.common['Authorization'] = token;
    }
  } else {
    delete axiosInstance.defaults.headers.common['Authorization'];
  }
};

/**
 * Decode JWT token and return its payload
 * Uses a safe approach that handles malformed tokens
 * 
 * @param {string} token - JWT token
 * @returns {object|null} - Decoded payload or null if invalid
 */
export const decodeToken = (token) => {
  if (!token) return null;
  
  try {
    // Check if token has the basic JWT structure
    if (!token.includes('.') || token.split('.').length !== 3) {
      console.warn('Token is not in valid JWT format');
      return null;
    }
    
    // JWT tokens are in the format header.payload.signature
    // We only need the payload part
    const base64Url = token.split('.')[1];
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
    
    // Add padding if needed
    let padding = '';
    if (base64.length % 4 > 0) {
      padding = '='.repeat(4 - (base64.length % 4));
    }
    
    try {
      const jsonPayload = decodeURIComponent(
        atob(base64 + padding)
          .split('')
          .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
          .join('')
      );
      
      return JSON.parse(jsonPayload);
    } catch (decodingError) {
      console.error('Failed to decode token payload:', decodingError);
      return null;
    }
  } catch (error) {
    console.error('Failed to decode token:', error);
    return null;
  }
};

/**
 * Creates a mock JWT token for development purposes
 * 
 * @returns {string} - A mock JWT token
 */
export const createMockToken = () => {
  // Create a simple JWT-like structure with a header and payload
  const header = {
    alg: 'mock',
    typ: 'JWT'
  };
  
  const payload = {
    sub: 'admin',
    name: 'Admin User',
    role: 'admin',
    exp: Math.floor(Date.now() / 1000) + (30 * 24 * 60 * 60), // 30 days from now
    iat: Math.floor(Date.now() / 1000)
  };
  
  // Convert to base64
  const headerBase64 = btoa(JSON.stringify(header));
  const payloadBase64 = btoa(JSON.stringify(payload));
  
  // Return a JWT-like token
  return `${headerBase64}.${payloadBase64}.mocksignature`;
};