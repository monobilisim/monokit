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
    // For JWT tokens, parse the expiration time
    const payload = JSON.parse(atob(token.split('.')[1]));
    const exp = payload.exp * 1000; // Convert to milliseconds
    
    return Date.now() >= exp;
  } catch (error) {
    console.warn('Error checking token expiration:', error);
    // If we can't parse the token, assume it's not expired
    // This is safer for development, but in production you might want to
    // err on the side of expiring the token if it can't be verified
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
    return config.environment.tokenDuration;
  }
  
  try {
    // For JWT tokens, calculate time until expiration
    const payload = JSON.parse(atob(token.split('.')[1]));
    const exp = payload.exp * 1000; // Convert to milliseconds
    
    return Math.max(0, exp - Date.now());
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
    axiosInstance.defaults.headers.common['Authorization'] = `Bearer ${token}`;
  } else {
    delete axiosInstance.defaults.headers.common['Authorization'];
  }
};

// Decode JWT token and return its payload
export const decodeToken = (token) => {
  if (!token) return null;
  
  try {
    // JWT tokens are in the format header.payload.signature
    // We only need the payload part
    const base64Url = token.split('.')[1];
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split('')
        .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    );
    
    return JSON.parse(jsonPayload);
  } catch (error) {
    console.error('Failed to decode token:', error);
    return null;
  }
}; 