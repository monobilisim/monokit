// Core authentication functions needed by App.jsx
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
      console.log('Using non-JWT token format - skipping expiration check');
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
