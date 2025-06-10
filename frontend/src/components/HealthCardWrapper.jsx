import React, { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Alert, Spinner } from '@patternfly/react-core';

/**
 * HealthCardWrapper is a React component that wraps the <health-card> web component.
 * It fetches data for a specific health tool and passes it to the web component.
 */
const HealthCardWrapper = ({ toolName, hostname }) => {
  const ref = useRef(null);
  const [healthData, setHealthData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchHealthData = async () => {
      setLoading(true);
      setError(null);
      try {
        const response = await fetch(`/api/v1/hosts/${hostname}/health/${toolName}`, {
          headers: {
            Authorization: localStorage.getItem('token'),
          },
        });
        if (!response.ok) {
          const errorData = await response.json();
          throw new Error(errorData.error || `Failed to fetch ${toolName} health data`);
        }
        const data = await response.json();
        setHealthData(data);
      } catch (err) {
        console.error(`Error fetching health data for ${toolName}:`, err);
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    if (hostname && toolName) {
      fetchHealthData();
    }
  }, [hostname, toolName]);

  useEffect(() => {
    // Ensure the web component's 'data' and 'tool' properties are updated when healthData changes.
    // The web component itself should be responsible for re-rendering when its properties change.
    if (ref.current && healthData) {
      ref.current.tool = toolName; // Pass the tool name, web component uses it for the title
      ref.current.data = healthData;
    }
     // Also update if error or loading state changes, so the web component can clear old data or show states
     if (ref.current) {
        ref.current.loading = loading;
        ref.current.error = error;
        if (!healthData && !loading) { // If no data and not loading (e.g. after an error or initial empty state)
            ref.current.data = null; 
        }
    }
  }, [healthData, toolName, loading, error]);

  if (loading) {
    return <Spinner size="xl" aria-label={`Loading ${toolName} health data...`} />;
  }

  if (error) {
    return (
      <Alert variant="danger" title={`Error loading ${toolName} health data`}>
        {error}
      </Alert>
    );
  }
  
  // health-card is a custom element (web component)
  // It's assumed to be registered globally, e.g. via a script tag in index.html
  // or loaded via the frontend build process.
  // The actual <health-card> element will handle rendering its own content based on the 'data' prop.
  return <health-card ref={ref}></health-card>;
};

HealthCardWrapper.propTypes = {
  toolName: PropTypes.string.isRequired,
  hostname: PropTypes.string.isRequired,
};

export default HealthCardWrapper;