import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import axios from 'axios';
import {
  Page,
  Sidebar,
  Masthead,
  MastheadMain,
  MastheadContent,
  PageSection,
  Button,
  Dropdown,
  DropdownList,
  DropdownItem,
  MenuToggle,
} from '@patternfly/react-core';
import { EllipsisVIcon, MoonIcon, SunIcon } from '@patternfly/react-icons';
import Navigation from './components/Navigation.jsx';
import ErrorBoundary from './components/ErrorBoundary.jsx'; // Import ErrorBoundary
import Dashboard from './pages/Dashboard.jsx';
import Hosts from './pages/Hosts.jsx';
import Users from './pages/Users.jsx';
import Inventories from './pages/Inventories.jsx';
import Login from './pages/Login.jsx';
import HostDetails from './pages/HostDetails.jsx';
import UserForm from './pages/UserForm.jsx';
import UserDetails from './pages/UserDetails.jsx';
import Groups from './pages/Groups.jsx';
import Logs from './pages/Logs.jsx';
import MonokitConfig from './pages/MonokitConfig.jsx';
import AwxJobLogViewer from './pages/AwxJobLogViewer.jsx';
import AwxHostAddPage from './pages/AwxHostAddPage.jsx';
import { useTheme } from './ThemeContext.jsx';
import api from './utils/api';
import { isTokenExpired, setupAuthHeaders } from './utils/authCore'; // Import the isTokenExpired and setupAuthHeaders utility

function App() {
  const [isNavOpen, setIsNavOpen] = useState(true);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [user, setUser] = useState(null); // Add missing user state
  const { theme, toggleTheme } = useTheme();

  useEffect(() => {
    const checkAuthentication = async () => {
      setIsLoading(true);
      const token = localStorage.getItem('token');
      
      if (token) {
        console.log('Token found in localStorage, checking validity');
        // Check if token is expired
        if (isTokenExpired(token)) {
          console.log('Token is expired, logging out');
          handleLogout();
          setIsLoading(false);
          return;
        }
        
        try {
          // Check if it's a JWT token (Keycloak)
          const isJWT = token.split('.').length === 3 && token.length > 100;
          
          // Set the appropriate Authorization header
          if (isJWT) {
            axios.defaults.headers.common['Authorization'] = `Bearer ${token}`;
            api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
          } else {
            axios.defaults.headers.common['Authorization'] = token;
            api.defaults.headers.common['Authorization'] = token;
          }
          
          // Test authentication with an API call
          try {
            const response = await api.get('/auth/me');
            if (response && response.data) {
              setUser(response.data); // Save user data if available
            }
          } catch (userError) {
            console.warn('Could not fetch user details, but continuing with authentication', userError);
            // Don't fail authentication just because we couldn't get user details
          }
          
          // Set authenticated state regardless of user details
          setIsAuthenticated(true);
          console.log('Authentication successful');
        } catch (err) {
          console.error('Authentication failed:', err.message);
          // Only logout if it's truly an auth error
          if (err.response?.status === 401) {
            handleLogout();
          } else {
            // For other errors (like network issues), still attempt to authenticate
            // based on the presence of the token, to prevent unnecessary logouts on page refresh
            console.warn('Non-authentication error encountered, maintaining session based on token presence');
            setIsAuthenticated(true);
          }
        }
      } else {
        console.log('No token found in localStorage');
      }
      
      setIsLoading(false);
    };
    
    checkAuthentication();
  }, []);

  useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    const tokenParam = urlParams.get('token');
    if (tokenParam) {
      console.log('Token found in URL, storing in localStorage and setting auth headers');
      handleLogin(tokenParam);
      window.history.replaceState(null, '', window.location.pathname);
    }
  }, []);

  const handleLogin = (token) => {
    console.log('Login successful, setting authenticated state');
    
    // Clean the token (remove quotes and whitespace)
    const cleanedToken = typeof token === 'string' ? token.trim().replace(/^["']|["']$/g, '') : token;
    
    // Store the clean token
    localStorage.setItem('token', cleanedToken);
    console.log('Token stored in localStorage:', cleanedToken);
      
      // Set the token in the default headers for all axios requests
      api.defaults.headers.common['Authorization'] = cleanedToken;
      axios.defaults.headers.common['Authorization'] = cleanedToken;
      
      // Verify headers
      console.log('Authorization header set in api:', !!api.defaults.headers.common['Authorization']);
      console.log('Authorization header set in axios:', !!axios.defaults.headers.common['Authorization']);
    
    setIsAuthenticated(true);
  };

  const handleLogout = () => {
    console.log('Manual logout triggered, clearing authentication state');
    localStorage.removeItem('token');
    
    // Remove the token from axios headers
    delete api.defaults.headers.common['Authorization'];
    delete axios.defaults.headers.common['Authorization'];
    
    setIsAuthenticated(false);
  };

  // Handle authentication errors (called from components)
  const handleAuthError = (error) => {
    console.log('Authentication error handler called');
    
    // Check if it's an authentication error
    if (error?.response?.status === 401) {
      console.log('Token seems to be invalid, logging out:', error.response?.data?.error || 'Unknown reason');
      
      // Display the error and token for debugging
      console.log('Current token (first few chars):', localStorage.getItem('token')?.substring(0, 15) + '...');
      console.log('Authorization header in api:', api.defaults.headers.common['Authorization']);
      
      // In this case, log out the user
      handleLogout();
    } else {
      console.log('Not an auth error or login process error, not logging out');
    }
  };

  if (isLoading) {
    return (
      <div style={{ 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center', 
        height: '100vh',
        backgroundColor: theme === 'dark' ? '#151515' : '#f0f0f0',
      }}>
        <span style={{ fontSize: '18px' }}>Loading...</span>
      </div>
    );
  }

  return (
    <Router>
      {!isAuthenticated ? (
        <Routes>
          <Route path="/login" element={<Login onLoginSuccess={handleLogin} />} />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      ) : (
        <div style={{ 
          display: 'flex', 
          height: '100vh',
          backgroundColor: theme === 'dark' ? '#151515' : '#f0f0f0',
          padding: '0'
        }}>
          {/* Navigation sidebar */}
          <div style={{ 
            width: '280px', 
            height: '100%',
            flexShrink: 0,
            backgroundColor: theme === 'dark' ? '#1b1b1b' : '#f0f0f0',
          }}>
            <Navigation />
          </div>
          
          {/* Main content area */}
          <div style={{ 
            display: 'flex', 
            flexDirection: 'column', 
            flex: 1, 
            overflow: 'auto',
            padding: '24px'
          }}>
            {/* Header with theme toggle and logout */}
            <div style={{
              display: 'flex',
              justifyContent: 'flex-end',
              padding: '8px 16px',
              marginBottom: '16px'
            }}>
              <Button 
                variant="secondary"
                onClick={handleLogout}
              >
                Logout
              </Button>
            </div>
            
            {/* Page content */}
            <main style={{ 
              flex: 1,
              backgroundColor: theme === 'dark' ? '#212427' : '#ffffff',
              borderRadius: '8px',
              overflow: 'auto',
              boxShadow: theme === 'dark' ? 'none' : '0 2px 4px rgba(0,0,0,0.1)',
              padding: '24px'
            }}>
              <ErrorBoundary> {/* Wrap Routes with ErrorBoundary */}
                <Routes>
                  <Route path="/" element={<Dashboard onAuthError={handleAuthError} />} />
                  <Route path="/hosts" element={<Hosts onAuthError={handleAuthError} />} />
                <Route path="/hosts/awx/add" element={<AwxHostAddPage onAuthError={handleAuthError} />} />
                <Route path="/hosts/:hostname" element={<HostDetails onAuthError={handleAuthError} />} />
                <Route path="/hosts/:hostname/config" element={<MonokitConfig onAuthError={handleAuthError} />} />
                <Route path="/hosts/:hostname/awx-jobs/:jobId/logs" element={<AwxJobLogViewer onAuthError={handleAuthError} />} />
                <Route path="/users" element={<Users onAuthError={handleAuthError} />} />
                <Route path="/users/new" element={<UserForm onAuthError={handleAuthError} />} />
                <Route path="/users/:username/edit" element={<UserForm onAuthError={handleAuthError} />} />
                <Route path="/users/:username" element={<UserDetails onAuthError={handleAuthError} />} />
                <Route path="/inventories" element={<Inventories onAuthError={handleAuthError} />} />
                <Route path="/groups" element={<Groups onAuthError={handleAuthError} />} />
                  <Route path="/logs" element={<Logs onAuthError={handleAuthError} />} />
                  <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
              </ErrorBoundary> {/* Close ErrorBoundary */}
            </main>
          </div>
        </div>
      )}
    </Router>
  );
}

export default App;
