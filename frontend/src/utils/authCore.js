// Core authentication functions needed by App.jsx
import config from '../config';

// Extract only the functions needed by App.jsx
export const isTokenExpired = (token) => {
  // ...existing implementation from auth.js...
};

export const setupAuthHeaders = (axiosInstance) => {
  // ...existing implementation from auth.js...
};
