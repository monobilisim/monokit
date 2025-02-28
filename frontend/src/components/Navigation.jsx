import React, { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTheme } from '../ThemeContext.jsx';
import { GruvboxColors } from '../ThemeContext.jsx';
import { 
  TachometerAltIcon, 
  ServerIcon, 
  UserIcon, 
  StorageDomainIcon,
  UsersIcon,
  MoonIcon,
  SunIcon,
  CogIcon
} from '@patternfly/react-icons';
import { Label } from '@patternfly/react-core';
import CenteredIcon from './CenteredIcon';
import axios from 'axios';

const Navigation = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { theme, toggleTheme } = useTheme();
  const [userInfo, setUserInfo] = useState(null);
  
  // Get the appropriate color palette based on the current theme
  const colors = theme === 'dark' ? GruvboxColors.dark : GruvboxColors.light;

  const navItems = [
    { name: 'Dashboard', path: '/', icon: <TachometerAltIcon /> },
    { name: 'Hosts', path: '/hosts', icon: <ServerIcon /> },
    { name: 'Users', path: '/users', icon: <UserIcon /> },
    { name: 'Inventories', path: '/inventories', icon: <StorageDomainIcon /> },
    { name: 'Groups', path: '/groups', icon: <UsersIcon /> }
  ];

  // Container styles
  const navContainerStyles = {
    width: '250px', // Set a fixed width
    height: '100vh', // Fill the entire viewport height
    backgroundColor: colors.bg0,
    color: colors.fg,
    display: 'flex',
    flexDirection: 'column',
    position: 'fixed', // Fix the sidebar to the viewport
    left: 0,
    top: 0,
    bottom: 0,
    boxShadow: theme === 'dark' ? '2px 0 5px rgba(0,0,0,0.3)' : '2px 0 5px rgba(0,0,0,0.1)',
    zIndex: 1000, // Ensure it stays above other content
  };

  // Logo/title styles
  const logoStyles = {
    padding: '20px 24px',
    fontSize: '20px',
    fontWeight: '700',
    color: colors.fg0,
    borderBottom: `1px solid ${colors.bg3}`,
    display: 'flex',
    alignItems: 'center',
  };

  // Navigation item styles
  const getNavItemStyles = (isActive) => ({
    display: 'flex',
    alignItems: 'center',
    padding: '12px 24px',
    cursor: 'pointer',
    borderLeft: isActive ? `3px solid ${colors.blue}` : '3px solid transparent',
    backgroundColor: isActive 
      ? (theme === 'dark' ? colors.bg1 : colors.bg2) 
      : 'transparent',
    fontWeight: isActive ? '600' : '400',
    color: isActive
      ? colors.fg0
      : colors.fg2,
  });

  // Icon styles
  const iconStyles = {
    marginRight: '16px',
    width: '18px',
    height: '18px',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    color: colors.fg2,
  };

  // Theme toggle styles
  const themeToggleContainerStyles = {
    padding: '16px 24px',
    borderTop: `1px solid ${colors.bg3}`,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    color: colors.fg2,
    cursor: 'pointer',
  };

  const toggleSwitchStyles = {
    position: 'relative',
    width: '40px',
    height: '20px',
    backgroundColor: theme === 'dark' ? colors.blue : colors.bg3,
    borderRadius: '34px',
    transition: '.4s',
    display: 'flex',
    alignItems: 'center',
    padding: '0 2px',
  };

  const toggleKnobStyles = {
    height: '16px',
    width: '16px',
    backgroundColor: colors.fg0,
    borderRadius: '50%',
    transition: '.4s',
    transform: theme === 'dark' ? 'translateX(20px)' : 'translateX(0)',
  };

  // Enhanced theme toggle handler with error catching
  const handleThemeToggle = () => {
    try {
      console.log('Before toggle - Current theme:', theme);
      console.log('toggleTheme is type:', typeof toggleTheme);
      
      if (typeof toggleTheme === 'function') {
        toggleTheme();
        console.log('Toggle function called');
      } else {
        console.error('toggleTheme is not a function!', toggleTheme);
      }
      
      // Force an update using setTimeout to see if there's a delay in state change
      setTimeout(() => {
        console.log('After toggle - Current theme:', theme);
      }, 100);
    } catch (error) {
      console.error('Error in handleThemeToggle:', error);
    }
  };

  const renderNavItem = (path, label, icon) => {
    const isActive = location.pathname === path || 
                    (path !== '/' && location.pathname.startsWith(path));
    
    return (
      <li key={path}>
        <div 
          style={getNavItemStyles(isActive)}
          onClick={() => navigate(path)}
          onMouseEnter={(e) => {
            if (!isActive) {
              e.currentTarget.style.backgroundColor = theme === 'dark' ? colors.bg1 : colors.bg2;
            }
          }}
          onMouseLeave={(e) => {
            if (!isActive) {
              e.currentTarget.style.backgroundColor = 'transparent';
            }
          }}
        >
          <CenteredIcon icon={icon} style={{ marginRight: '8px', color: isActive ? colors.fg0 : colors.fg3 }} />
          {label}
        </div>
      </li>
    );
  };

  // User details button styles
  const userDetailsStyles = {
    padding: '16px 24px',
    border: 'none',
    boxShadow: `0 1px 0 ${colors.bg3}, 0 -1px 0 ${colors.bg3}`,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    color: colors.fg2,
    cursor: 'pointer',
    background: 'none',
    width: '100%',
    textAlign: 'left',
  };

  useEffect(() => {
    const fetchUserInfo = async () => {
      try {
        const response = await axios.get('/api/v1/auth/me', {
          headers: {
            Authorization: localStorage.getItem('token')
          }
        });
        setUserInfo(response.data);
      } catch (err) {
        console.error('Failed to fetch user information');
      }
    };

    fetchUserInfo();
  }, []);

  return (
    <div style={navContainerStyles}>
      <div style={logoStyles}>
        Monokit
      </div>
      <nav style={{ 
        flex: 1, 
        overflowY: 'auto'
      }}> {/* Add overflow for scrolling if needed */}
        <ul style={{ listStyle: 'none', padding: '8px 0', margin: 0 }}>
          {navItems.map(({ name, path, icon }) => renderNavItem(path, name, icon))}
        </ul>
      </nav>
      
      {/* User details button */}
      {userInfo && (
        <button 
          style={userDetailsStyles}
          onClick={() => navigate(`/users/${userInfo.username}`)}
        >
          <span style={{ display: 'flex', alignItems: 'center' }}>
            {userInfo.username}
          </span>
          <Label 
            style={{
              backgroundColor: userInfo.role === 'admin' ? colors.blue : colors.green,
              color: theme === 'dark' ? colors.bg0 : colors.fg0,
              padding: '4px 8px',
              borderRadius: '30px',
              fontSize: '12px',
              fontWeight: '500',
            }}
          >
            {userInfo.role === 'admin' ? 'Administrator' : 'User'}
          </Label>
        </button>
      )}
      
      {/* Dark mode toggle with enhanced handler */}
      <button 
        style={{
          ...themeToggleContainerStyles,
          border: 'none',
          background: 'none',
          width: '100%',
          textAlign: 'left',
          outline: 'none'
        }} 
        onClick={handleThemeToggle}
      >
        <span style={{ display: 'flex', alignItems: 'center' }}>
          {theme === 'dark' ? 'Dark Mode' : 'Light Mode'}
          <CenteredIcon 
            icon={theme === 'dark' ? <MoonIcon /> : <SunIcon />}
            style={{ marginLeft: '8px', color: theme === 'dark' ? colors.yellow : colors.orange }}
          />
        </span>
        <div style={toggleSwitchStyles}>
          <div style={toggleKnobStyles}></div>
        </div>
      </button>
    </div>
  );
};

export default Navigation; 