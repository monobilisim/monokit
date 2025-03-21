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
    const token = localStorage.getItem('token');
    if (token) {
      try {
        // Check if this is likely a JWT token
        const isJWT = token.includes('.') && token.split('.').length === 3 && token.length > 50;
        
        if (isJWT) {
          config.headers['Authorization'] = `Bearer ${token}`;
          console.log('Adding JWT Bearer token to request');
        } else {
          // For non-JWT tokens, just use the token directly
          config.headers['Authorization'] = token;
          console.log('Adding standard token to request');
        }
      } catch (error) {
        // If there's any error detecting token type, just use the token as-is
        console.warn('Error checking token type:', error);
        config.headers['Authorization'] = token;
      }
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
    
    // If we get a 401 error, it could mean the token is expired
    if (error.response && error.response.status === 401) {
      // Only log out if not on the login page already
      if (!window.location.pathname.includes('/login')) {
        console.log('Received 401 error, session may have expired');
        // Mark this request as failed authentication
        error.isAuthError = true;
      }
    }
    
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
      
      // Store the token in localStorage regardless of type
      localStorage.setItem('token', cleanedApiKey);
      
      try {
        // Check if this is likely a JWT token (Keycloak tokens are JWTs)
        const isJWT = cleanedApiKey && 
                    cleanedApiKey.includes('.') && 
                    cleanedApiKey.split('.').length === 3 && 
                    cleanedApiKey.length > 50;
        
        // Set the appropriate authorization header
        if (isJWT) {
          console.log('Setting Bearer token for JWT');
          api.defaults.headers.common['Authorization'] = `Bearer ${cleanedApiKey}`;
          axios.defaults.headers.common['Authorization'] = `Bearer ${cleanedApiKey}`;
        } else {
          console.log('Setting standard token (non-JWT)');
          api.defaults.headers.common['Authorization'] = cleanedApiKey;
          axios.defaults.headers.common['Authorization'] = cleanedApiKey;
        }
      } catch (headerError) {
        console.warn('Error setting auth headers:', headerError);
        // Fallback to setting token without Bearer prefix
        api.defaults.headers.common['Authorization'] = cleanedApiKey;
        axios.defaults.headers.common['Authorization'] = cleanedApiKey;
      }
      
      return response.data;
    } else {
      throw new Error('Invalid response format from server');
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

// Logs API calls
export const getLogs = async () => {
  try {
    const response = await api.get('/logs');
    return response;
  } catch (error) {
    console.error('Error fetching logs:', error);
    throw error;
  }
};

// AWX Jobs Logs API call
export const getAwxJobLogs = async (hostname, jobId, focusOnHost = true) => {
  try {
    const response = await api.get(`/hosts/${hostname}/awx-jobs/${jobId}/logs`, {
      params: {
        focus_host: focusOnHost
      }
    });
    return response;
  } catch (error) {
    console.error(`Error fetching AWX job logs for job ${jobId}:`, error);
    throw error;
  }
};

// Config API calls
export const getConfig = async (name) => {
  try {
    const endpoint = name ? `/hosts/${name}/config` : '/host/config';
    const response = await api.get(endpoint);
    return response;
  } catch (error) {
    console.error('Error fetching config for host %s:', name || 'current host', error);
    throw error;
  }
};

export const updateConfig = async (name, configData) => {
  try {
    const endpoint = name ? `/hosts/${name}/config` : '/host/config';
    const response = await api.post(endpoint, configData);
    return response;
  } catch (error) {
    console.error('Error updating config:', error);
    throw error;
  }
};

// Add this new function for deleting configuration files
export const deleteConfig = async (name, fileName) => {
  try {
    const endpoint = name ? `/hosts/${name}/config/${fileName}` : `/host/config/${fileName}`;
    const response = await api.delete(endpoint);
    return response;
  } catch (error) {
    console.error(`Error deleting config file ${fileName}:`, error);
    throw error;
  }
};

export const createLog = async (logData) => {
  try {
    const response = await api.post('/logs', logData);
    return response;
  } catch (error) {
    console.error('Error creating log:', error);
    throw error;
  }
};

export default api;
