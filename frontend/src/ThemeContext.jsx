import React, { createContext, useState, useEffect, useContext, useCallback } from 'react';

// Gruvbox color palette
export const GruvboxColors = {
  dark: {
    bg: '#282828',
    bg0: '#1d2021',
    bg1: '#3c3836',
    bg2: '#504945',
    bg3: '#665c54',
    bg4: '#7c6f64',
    fg: '#ebdbb2',
    fg0: '#fbf1c7',
    fg1: '#ebdbb2',
    fg2: '#d5c4a1',
    fg3: '#bdae93',
    fg4: '#a89984',
    red: '#fb4934',
    green: '#b8bb26',
    yellow: '#fabd2f',
    blue: '#83a598',
    purple: '#d3869b',
    aqua: '#8ec07c',
    orange: '#fe8019',
    gray: '#928374',
  },
  light: {
    bg: '#fbf1c7',
    bg0: '#f9f5d7',
    bg1: '#ebdbb2',
    bg2: '#d5c4a1',
    bg3: '#bdae93',
    bg4: '#a89984',
    fg: '#3c3836',
    fg0: '#282828',
    fg1: '#3c3836',
    fg2: '#504945',
    fg3: '#665c54',
    fg4: '#7c6f64',
    red: '#9d0006',
    green: '#79740e',
    yellow: '#b57614',
    blue: '#076678',
    purple: '#8f3f71',
    aqua: '#427b58',
    orange: '#af3a03',
    gray: '#7c6f64',
  }
};

// Create the context with default values
export const ThemeContext = createContext({
  theme: 'light',
  toggleTheme: () => {},
});

// Custom hook to use the theme context
export const useTheme = () => {
  const context = useContext(ThemeContext);
  if (!context) {
    console.error('useTheme must be used within a ThemeProvider');
    return { theme: 'light', toggleTheme: () => console.warn('ThemeProvider not found') };
  }
  return context;
};

// Create a provider component
export const ThemeProvider = ({ children }) => {
  console.log('ThemeProvider initialized');
  
  // Try to get the theme from localStorage or default to 'dark'
  const [theme, setTheme] = useState(() => {
    try {
      const savedTheme = localStorage.getItem('theme');
      console.log('Initial theme from localStorage:', savedTheme);
      return savedTheme || 'dark';
    } catch (error) {
      console.error('Error accessing localStorage:', error);
      return 'dark';
    }
  });

  // Toggle theme function using useCallback to prevent recreation on renders
  const toggleTheme = useCallback(() => {
    try {
      console.log('toggleTheme called, current theme:', theme);
      const newTheme = theme === 'dark' ? 'light' : 'dark';
      setTheme(newTheme);
      localStorage.setItem('theme', newTheme);
      console.log('Theme toggled to:', newTheme);
    } catch (error) {
      console.error('Error in toggleTheme:', error);
    }
  }, [theme]);

  // Apply theme to body when it changes
  useEffect(() => {
    try {
      console.log('Theme changed to:', theme);
      
      // Keep data-theme attribute for compatibility with existing styles during transition
      document.documentElement.setAttribute('data-theme', theme);
      
      // Apply or remove the PatternFly dark theme class to the HTML element
      if (theme === 'dark') {
        document.documentElement.classList.add('pf-v6-theme-dark');
      } else {
        document.documentElement.classList.remove('pf-v6-theme-dark');
      }
      
      // Apply background and text color to body for compatibility
      document.body.style.backgroundColor = theme === 'dark' ? GruvboxColors.dark.bg : GruvboxColors.light.bg;
      document.body.style.color = theme === 'dark' ? GruvboxColors.dark.fg : GruvboxColors.light.fg;
    } catch (error) {
      console.error('Error applying theme:', error);
    }
  }, [theme]);

  // Create memoized context value to prevent unnecessary re-renders
  const contextValue = React.useMemo(() => ({
    theme,
    toggleTheme
  }), [theme, toggleTheme]);

  // Provide the theme context to children
  return (
    <ThemeContext.Provider value={contextValue}>
      {children}
    </ThemeContext.Provider>
  );
}; 