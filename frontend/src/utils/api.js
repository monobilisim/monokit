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
    
    // Add more detailed information for common network errors
    if (error.response) {
      switch (error.response.status) {
        case 401:
          // Only log out if not on the login page already
          if (!window.location.pathname.includes('/login')) {
            console.log('Received 401 error, session may have expired');
            // Mark this request as failed authentication
            error.isAuthError = true;
          }
          break;
        case 502:
          console.error('Bad Gateway (502) error - Connection to backend service failed');
          error.userMessage = 'Unable to connect to the server. Check network configuration and server status.';
          break;
        case 504:
          console.error('Gateway Timeout (504) error - Connection timed out');
          error.userMessage = 'The connection timed out. The server might be overloaded or unavailable.';
          break;
      }
    } else if (error.request) {
      // Request was made but no response received
      console.error('No response received for request:', error.request);
      error.userMessage = 'No response received from the server. Please check your network connection.';
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

// Force delete a host (bypassing all safety checks)
export const forceDeleteHost = async (hostname) => {
  try {
    const response = await api.delete(`/hosts/${hostname}/force`);
    return response;
  } catch (error) {
    console.error(`Error force deleting host ${hostname}:`, error);
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
    console.error('Error fetching AWX job logs for job %s:', jobId, error);
    throw error;
  }
};

// Get AWX Job Templates API call - now returns hardcoded data
export const getAwxJobTemplates = async (hostname) => {
  console.log(`Returning hardcoded job templates for host ${hostname}`);
  // Return a mock response with hardcoded templates
  return {
    data: [
      { id: 107, name: "manual-check-ping" },
      { id: 95, name: "workflow-manual-setup-fresh" },
      { id: 258, name: "manual-install-monokit-client" }
    ]
  };
};

// Get AWX Job Templates without requiring a host - uses hardcoded template IDs
export const getAwxTemplatesGlobal = async () => {
  console.log("Returning hardcoded job templates");
  // Return a mock response with hardcoded templates
  return {
    data: {
      results: [
        { id: 107, name: "manual-check-ping" },
        { id: 258, name: "manual-install-monokit-client" }
      ]
    }
  };
};

// Get AWX Workflow Templates without requiring a host - uses hardcoded template IDs
export const getAwxWorkflowTemplatesGlobal = async () => {
  console.log("Returning hardcoded workflow templates");
  // Return a mock response with hardcoded templates
  return {
    data: {
      results: [
        { id: 95, name: "workflow-manual-setup-fresh" }
      ]
    }
  };
};

// Get AWX Job Template Details API call - now returns hardcoded data
export const getAwxJobTemplateDetails = async (hostname, templateId) => {
  console.log(`Returning hardcoded details for template ${templateId} on host ${hostname}`);
  // Return a mock response with template details
  return {
    data: {
      id: templateId,
      name: `Template ${templateId}`,
      description: "Template description",
      variables: {
        extra_vars: {
          example_var: "example_value"
        }
      }
    }
  };
};

// Execute AWX Job API call
export const executeAwxJob = async (hostname, templateId, extraVars = {}, inventoryId = null) => {
  try {
    console.log("Sending execute AWX job request for host:", hostname);
    console.log("Template ID:", templateId);
    console.log("Extra vars:", extraVars);
    
    // Find the template name based on templateId to determine if it's a workflow
    let isWorkflow = false;
    
    // Check for workflow templates - 95 is workflow-manual-setup-fresh
    if (templateId === 95) {
      isWorkflow = true;
      console.log("Detected workflow template. Using workflow job endpoint.");
    }
    
    const payload = {
      template_id: templateId,
      extra_vars: extraVars,
      format: "yaml"  // Indicate that we're using YAML format
    };
    
    // Always include inventory_id in the payload
    if (inventoryId) {
      payload.inventory_id = inventoryId;
      console.log("Using explicit inventory ID:", inventoryId);
    } else {
      // Try to get inventory ID from server default
      console.log("No inventory ID provided, using server default");
      
      // Set a default inventory ID here as a fallback (you can adjust this as needed)
      payload.inventory_id = 2; // This should match the server default_inventory_id
    }
    
    // For workflow templates, use a specialized endpoint
    let endpoint = `/hosts/${hostname}/awx-jobs/execute`;
    
    if (isWorkflow) {
      console.log("Using dedicated workflow endpoint for template ID:", templateId);
      // Use workflow-specific endpoint
      endpoint = `/hosts/${hostname}/awx-workflow-jobs/execute`;
      // Rename the key for workflow templates
      payload.workflow_template_id = payload.template_id;
      delete payload.template_id;
    }
    
    console.log(`Executing AWX job at endpoint: ${endpoint}`);
    const response = await api.post(endpoint, payload);
    return response;
  } catch (error) {
    console.error('Error executing AWX job template %s for host %s:', templateId, hostname, error);
    
    // Add more context to the error for debugging
    if (error.response && error.response.status === 500) {
      console.error('Error details from server:', error.response.data);
      
      // Check if this might be a workflow template error
      if (templateId === 95) {
        console.warn('This appears to be a workflow template (ID: 95). The server may need to use a different endpoint for workflows.');
      }
    }
    
    throw error;
  }
};

// Create a host in AWX
export const createAwxHost = async (name, ipAddress) => {
  try {
    console.log('Creating AWX host with params:', { name, ip_address: ipAddress });
    const response = await api.post('/hosts/awx', {
      name: name,
      ip_address: ipAddress
    });
    console.log('AWX host creation response:', response);
    return response;
  } catch (error) {
    // Log detailed error information
    console.error('Error creating host in AWX:', error);
    
    // Safely extract error information without causing secondary errors
    const statusCode = error.response?.status;
    let errorMessage = 'Unknown error occurred';
    
    if (error.response) {
      console.error('Error response status:', statusCode);
      console.error('Error response headers:', error.response.headers);
      
      try {
        // Safely log the error data
        const errorData = error.response.data;
        console.error('Error data:', typeof errorData === 'object' ? JSON.stringify(errorData) : errorData);
        
        // Extract a user-friendly error message
        if (typeof errorData === 'string') {
          errorMessage = errorData;
        } else if (errorData?.error) {
          errorMessage = typeof errorData.error === 'string' 
            ? errorData.error 
            : JSON.stringify(errorData.error);
        } else if (statusCode === 502) {
          errorMessage = 'Unable to connect to AWX service. Please check if AWX is running and accessible.';
        }
      } catch (parseError) {
        console.error('Error parsing error response:', parseError);
      }
    } else if (error.request) {
      errorMessage = 'No response received from server. Network issue or server unreachable.';
    } else {
      errorMessage = error.message || 'Unknown error occurred';
    }
    
    // Create a new error with a clean message that won't cause issues when displayed
    const enhancedError = new Error(errorMessage);
    enhancedError.originalError = error;
    enhancedError.statusCode = statusCode;
    throw enhancedError;
  }
};

// Delete a host from AWX
export const deleteAwxHost = async (hostId) => {
  try {
    const response = await api.delete(`/hosts/awx/${hostId}`);
    return response;
  } catch (error) {
    console.error(`Error deleting host ${hostId} from AWX:`, error);
    throw error;
  }
};

// Get AWX job status
export const getAwxJobStatus = async (jobId) => {
  try {
    const response = await api.get(`/awx/jobs/${jobId}`);
    return response;
  } catch (error) {
    console.error(`Error fetching AWX job ${jobId} status:`, error);
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
    console.error('Error deleting config file %s:', fileName, error);
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
