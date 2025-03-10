import React, { useState } from 'react';
import {
  LoginPage,
  Form,
  FormGroup,
  ActionGroup,
  Button,
  Alert,
} from '@patternfly/react-core';
import { useNavigate } from 'react-router-dom';
import { login } from '../utils/api';
import axios from 'axios';
import './Login.css';
import config from '../config';

const Login = ({ onLoginSuccess }) => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);
const navigate = useNavigate();
  const handleKeycloakLogin = () => {
    const origin = window.location.protocol + "//" + window.location.host;
    const { baseUrl } = config.api;
    const backendUrl = baseUrl.startsWith('http') ? baseUrl : `${origin}${baseUrl}`;
    // Directly use the full URL as redirectUri without encoding to ensure the domain is included as expected.
    const redirectUri = `${origin}/api/v1/auth/sso/callback`;
    window.location.href = `${backendUrl}/auth/sso/login?redirect_uri=${redirectUri}`;
  };
  
  const handleSubmit = async (e) => {
    e.preventDefault();
    setIsLoading(true);
    setError('');
    
    console.log('Attempting login with:', { username }); // Don't log password
    
    // For development, allow a mock login if we're in dev mode
    if (config.environment.isDevelopment && username === 'admin' && password === 'admin') {
      console.log('Using mock login for development');
      // Create a mock token
      const mockToken = 'mock-jwt-token-for-development';
      localStorage.setItem('token', mockToken);
      onLoginSuccess(mockToken);
      setIsLoading(false);
      navigate('/');
      return;
    }

    try {
      const response = await login(username, password);
      console.log('Login response:', response);
      
      // Extract token from response based on API structure
      const token = response.token || response.access_token || response.jwt || response;
      
      if (!token) {
        console.error('No token found in response:', response);
        throw new Error('Invalid response format from server - missing token');
      }
      
      // Clean the token (remove quotes and whitespace)
      const cleanedToken = typeof token === 'string' ? token.trim().replace(/^["']|["']$/g, '') : token;
      
      // Store token in localStorage
      localStorage.setItem('token', cleanedToken);
      console.log('Token stored in localStorage:', cleanedToken);
      
      // Set token in axios header for future requests
      axios.defaults.headers.common['Authorization'] = `Bearer ${cleanedToken}`;
      
      // Log the headers to verify
      console.log('Authorization header set:', axios.defaults.headers.common['Authorization']);
      
      // Call the onLoginSuccess callback
      onLoginSuccess(cleanedToken);
      
      // Navigate to dashboard
      navigate('/');
    } catch (err) {
      console.error('Login error:', err);
      setError(err.response?.data?.error || err.response?.data?.message || err.message || 'Failed to login. Please check your credentials.');
    } finally {
      setIsLoading(false);
    }
  };

  // Custom input styles
  const inputStyles = {
    backgroundColor: 'transparent',
    border: '1px solid rgba(255, 255, 255, 0.3)',
    borderRadius: '24px',
    padding: '12px 16px',
    width: '100%',
    color: '#ffffff',
    fontSize: '16px',
    outline: 'none',
    boxSizing: 'border-box'
  };

  const labelStyles = {
    color: '#ffffff',
    marginBottom: '8px',
    display: 'block',
    fontWeight: '500'
  };

  const requiredStyles = {
    color: 'red',
    marginLeft: '4px'
  };

  return (
    <div style={{
      height: '100vh',
      backgroundImage: 'linear-gradient(rgba(0, 0, 0, 0.6), rgba(0, 0, 0, 0.6)), url(https://live.staticflickr.com/4519/38494602082_516d2b15cd_o_d.jpg)',
      backgroundSize: 'cover',
      backgroundPosition: 'center',
      position: 'relative',
      display: 'flex',
      alignItems: 'center',
      paddingLeft: '10vw'
    }}>
      <div style={{ 
        backgroundColor: 'rgba(255, 255, 255, 0.1)',
        backdropFilter: 'blur(10px)',
        padding: '2rem',
        borderRadius: '8px',
        width: '300px',
        boxShadow: '0 4px 8px rgba(0,0,0,0.2)',
        border: '1px solid rgba(255, 255, 255, 0.2)'
      }}>
        <div style={{ marginBottom: '2rem', textAlign: 'center' }}>
          <h1 style={{ 
            fontSize: '24px',
            color: '#ffffff',
            margin: 0
          }}>
            Welcome to Monokit!
          </h1>
        </div>
        <form onSubmit={handleSubmit}>
          {error && (
            <Alert variant="danger" title={error} style={{ marginBottom: '20px' }} />
          )}
          <div style={{ marginBottom: '16px' }}>
            <label htmlFor="username" style={labelStyles}>
              Username <span style={requiredStyles}>*</span>
            </label>
            <input
              type="text"
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              style={inputStyles}
              required
            />
          </div>
          <div style={{ marginBottom: '24px' }}>
            <label htmlFor="password" style={labelStyles}>
              Password <span style={requiredStyles}>*</span>
            </label>
            <input
              type="password"
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              style={inputStyles}
              required
            />
          </div>
          <div>
            <Button 
              variant="primary" 
              type="submit"
              isLoading={isLoading}
              isDisabled={isLoading || !username || !password}
              style={{ width: '100%', borderRadius: '24px' }}
            >
              Log in
            </Button>
          </div>
        </form>
        <div style={{ marginTop: '20px', textAlign: 'center' }}>
          <Button variant="primary" onClick={handleKeycloakLogin}>Login with Keycloak</Button>
        </div>
      </div>
      <div style={{
        position: 'absolute',
        bottom: '1rem',
        right: '1rem',
        color: '#ffffff',
        textShadow: '2px 2px 4px rgba(0,0,0,0.8)'
      }}>
        <small>
          "Furggelen afterglow" by Lukas Schlagenhauf is licensed under CC BY-ND 2.0.
        </small>
      </div>
    </div>
  );
};

export default Login;
