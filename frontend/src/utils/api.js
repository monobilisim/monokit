import axios from 'axios';

// Simply get the current domain's base URL
const getBaseUrl = () => {
  const protocol = window.location.protocol;
  const hostname = window.location.hostname;
  return `${protocol}//${hostname}/api/v1`;
};

// Get the base URL
const baseURL = getBaseUrl();
console.log('Using API base URL:', baseURL);

// Create an axios instance with the dynamic base URL
const api = axios.create({
  baseURL,
  headers: {
    'Content-Type': 'application/json',
  }
});

// Add a request interceptor to include the API key in all requests
api.interceptors.request.use(
  (config) => {
    const apiKey = localStorage.getItem('token');
    if (apiKey) {
      // Check if this is likely a JWT token (Keycloak tokens are JWTs)
      // A valid JWT has 3 parts separated by dots and is typically long
      const isJWT = apiKey.split('.').length === 3 && apiKey.length > 100;
      
      // Always add Bearer prefix for JWT tokens (Keycloak tokens)
      if (isJWT) {
        console.log('Adding Bearer prefix to JWT token');
        config.headers.Authorization = `Bearer ${apiKey}`;
      } else {
        // For regular session tokens, use as-is
        console.log('Using regular token without Bearer prefix');
        config.headers.Authorization = apiKey;
      }
      
      // Debug the final authorization header
      console.log('Final Authorization header:', config.headers.Authorization.substring(0, 20) + '...');
    }
    return config;
  },
  (error) => {
    console.error('Request interceptor error:', error);
    return Promise.reject(error);
  }
);

// Add a response interceptor to handle common errors
api.interceptors.response.use(
  (response) => {
    return response;
  },
  (error) => {
    const status = error.response?.status;
    const url = error.config?.url;
    
    console.error(`API Error: ${status} on ${url}`, error.response?.data || error.message);
    
    // Don't handle auth errors here, let the components decide what to do
    return Promise.reject(error);
  }
);

// Auth API calls
export const login = async (username, password) => {
  try {
    // Use the dynamic base URL for login
    const response = await api.post('/auth/login', { 
      username, 
      password 
    });
    
    if (response.data) {
      // Get API key from response based on API format
      const apiKey = response.data.token || response.data.api_key || response.data.apiKey || response.data.key || response.data;
      
      // Make sure we're storing the API key in the proper format
      const cleanedApiKey = typeof apiKey === 'string' ? apiKey.trim().replace(/^["']|["']$/g, '') : apiKey;
      
      // Check if this is likely a JWT token (Keycloak tokens are JWTs)
      const isJWT = cleanedApiKey.split('.').length === 3 && cleanedApiKey.length > 100;
      
      // Set the appropriate authorization header
      if (isJWT) {
        console.log('Setting Bearer token for JWT');
        api.defaults.headers.common['Authorization'] = `Bearer ${cleanedApiKey}`;
        axios.defaults.headers.common['Authorization'] = `Bearer ${cleanedApiKey}`;
      } else {
        console.log('Setting regular token without Bearer prefix');
        api.defaults.headers.common['Authorization'] = cleanedApiKey;
        axios.defaults.headers.common['Authorization'] = cleanedApiKey;
      }
      
      // Store it in localStorage
      localStorage.setItem('token', cleanedApiKey);
      
      return response.data;
    } else {
      throw new Error('No data received from login');
    }
  } catch (error) {
    console.error('Login error:', error);
    throw error;
  }
};

// Hosts API calls
export const getHosts = async () => {
  try {
    const response = await api.get('/hosts');
    return response;
  } catch (error) {
    console.error('Error fetching hosts:', error);
    throw error;
  }
};

export const getHost = async (hostname) => {
  try {
    const response = await api.get(`/hosts/${hostname}`);
    return response;
  } catch (error) {
    console.error(`Error fetching host ${hostname}:`, error);
    throw error;
  }
};

// Users API calls
export const getUsers = async () => {
  try {
    const response = await api.get('/users');
    return response;
  } catch (error) {
    console.error('Error fetching users:', error);
    throw error;
  }
};

export const getUser = async (username) => {
  try {
    const response = await api.get(`/users/${username}`);
    return response;
  } catch (error) {
    console.error(`Error fetching user ${username}:`, error);
    throw error;
  }
};

export const createUser = async (userData) => {
  try {
    const response = await api.post('/users', userData);
    return response;
  } catch (error) {
    console.error('Error creating user:', error);
    throw error;
  }
};

export const updateUser = async (username, userData) => {
  try {
    const response = await api.put(`/users/${username}`, userData);
    return response;
  } catch (error) {
    console.error(`Error updating user ${username}:`, error);
    throw error;
  }
};

export const deleteUser = async (username) => {
  try {
    const response = await api.delete(`/users/${username}`);
    return response;
  } catch (error) {
    console.error(`Error deleting user ${username}:`, error);
    throw error;
  }
};

// Inventories API calls
export const getInventories = async () => {
  try {
    const response = await api.get('/inventory');
    return response;
  } catch (error) {
    console.error('Error fetching inventories:', error);
    throw error;
  }
};

// Create a new inventory
export const createInventory = async (inventoryName) => {
  try {
    const response = await api.post('/inventory', {
      name: inventoryName
    });
    return response;
  } catch (error) {
    console.error('Error creating inventory:', error);
    throw error;
  }
};

// Delete an inventory
export const deleteInventory = async (inventoryName) => {
  try {
    const response = await api.delete(`/inventory/${inventoryName}`);
    return response;
  } catch (error) {
    console.error(`Error deleting inventory ${inventoryName}:`, error);
    throw error;
  }
};

// Groups API calls
export const getGroups = async () => {
  try {
    const response = await api.get('/admin/groups');
    return response;
  } catch (error) {
    console.error('Error fetching groups:', error);
    throw error;
  }
};

// Create a new group
export const createGroup = async (groupName) => {
  try {
    const response = await api.post('/admin/groups', {
      name: groupName
    });
    return response;
  } catch (error) {
    console.error('Error creating group:', error);
    throw error;
  }
};

// Delete a group
export const deleteGroup = async (groupName, withHosts = false) => {
  try {
    const response = await api.delete(`/admin/groups/${groupName}`, {
      params: { withHosts }
    });
    return response;
  } catch (error) {
    console.error(`Error deleting group ${groupName}:`, error);
    throw error;
  }
};

export default api;
