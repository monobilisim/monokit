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
import { useTheme } from './ThemeContext.jsx';
import api from './utils/api';

function App() {
  const [isNavOpen, setIsNavOpen] = useState(true);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const { theme, toggleTheme } = useTheme();

  useEffect(() => {
    // Check if user is already authenticated
    const token = localStorage.getItem('token');
    
    if (token) {
      console.log('Token found in localStorage, setting authenticated state');
      console.log('Token value (first few chars):', token.substring(0, 15) + '...');
      setIsAuthenticated(true);
      
      // Set the token in the default headers for all axios requests
      api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
      
      // Also set for axios global defaults to ensure all requests include the token
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`;
      
      // Verify the headers are set
      console.log('Authorization header set in api instance:', !!api.defaults.headers.common['Authorization']);
      console.log('Authorization header set in axios global:', !!axios.defaults.headers.common['Authorization']);
    } else {
      console.log('No token found in localStorage');
    }
    
    setIsLoading(false);
  }, []);

  const handleLogin = (token) => {
    console.log('Login successful, setting authenticated state');
    
    // Clean the token (remove quotes and whitespace)
    const cleanedToken = typeof token === 'string' ? token.trim().replace(/^["']|["']$/g, '') : token;
    
    // Store the clean token
    localStorage.setItem('token', cleanedToken);
    console.log('Token stored in localStorage:', cleanedToken);
    
    // Set the token in the default headers for all axios requests
    api.defaults.headers.common['Authorization'] = `Bearer ${cleanedToken}`;
    axios.defaults.headers.common['Authorization'] = `Bearer ${cleanedToken}`;
    
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
              <Routes>
                <Route path="/" element={<Dashboard onAuthError={handleAuthError} />} />
                <Route path="/hosts" element={<Hosts onAuthError={handleAuthError} />} />
                <Route path="/hosts/:hostname" element={<HostDetails onAuthError={handleAuthError} />} />
                <Route path="/users" element={<Users onAuthError={handleAuthError} />} />
                <Route path="/users/new" element={<UserForm onAuthError={handleAuthError} />} />
                <Route path="/users/:username/edit" element={<UserForm onAuthError={handleAuthError} />} />
                <Route path="/users/:username" element={<UserDetails onAuthError={handleAuthError} />} />
                <Route path="/inventories" element={<Inventories onAuthError={handleAuthError} />} />
                <Route path="/groups" element={<Groups onAuthError={handleAuthError} />} />
                <Route path="/logs" element={<Logs onAuthError={handleAuthError} />} />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </main>
          </div>
        </div>
      )}
    </Router>
  );
}

export default App;