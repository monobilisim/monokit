import React, { useState, useEffect } from 'react';
import { Table, Thead, Tbody, Tr, Td, Th } from '@patternfly/react-table';
import { 
  Spinner, 
  Bullseye, 
  EmptyState, 
  Title, 
  Label, 
  Button,
  Pagination,
  Badge
} from '@patternfly/react-core';
import { ExclamationCircleIcon, SearchIcon, ListIcon } from '@patternfly/react-icons';
import axios from 'axios';
import { format } from 'date-fns';

const AwxJobsTable = ({ hostName }) => {
  const [jobs, setJobs] = useState([]);
  const [filteredJobs, setFilteredJobs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showDebugInfo, setShowDebugInfo] = useState(false);
  const [errorDetails, setErrorDetails] = useState({});
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreAvailable, setMoreAvailable] = useState(false);
  const [nextPageUrl, setNextPageUrl] = useState(null);
  const [pagesFetched, setPagesFetched] = useState(0);
  
  // Pagination state
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [totalItems, setTotalItems] = useState(0);

  const fetchJobs = async (url = null, append = false) => {
    try {
      if (!url) {
        setLoading(true);
        console.log(`Fetching initial AWX jobs for host: ${hostName}`);
      } else {
        setLoadingMore(true);
        console.log(`Fetching more AWX jobs from: ${url}`);
      }
      
      // Prepare API call
      const endpoint = url || `/api/v1/hosts/${hostName}/awx-jobs`;
      
      // If we're fetching the initial data, add some params
      const params = !url ? { page_size: 50 } : {}; // Request larger page size for initial load
      
      const response = await axios.get(endpoint, {
        headers: {
          Authorization: localStorage.getItem('token')
        },
        params,
        timeout: 15000 // 15 second timeout
      });
      
      console.log('AWX jobs API response:', response.data);
      
      // Check if response has results array and is paginated
      if (response.data && Array.isArray(response.data.results)) {
        // This is a paginated response from AWX directly
        const { results, next, count } = response.data;
        
        // Transform the job results for display
        const processedJobs = results.map(job => ({
          id: job.id,
          operation: job.name || job.job_template_name || 'Unknown Operation',
          status: job.status || 'Unknown',
          summary: job.job_template_name ? `Template: ${job.job_template_name}` : `Job #${job.id}`,
          timestamp: job.started || job.created || new Date().toISOString(),
          node: hostName,
          // Additional job info for rendering
          failed: job.failed,
          elapsed: job.elapsed,
          finished: job.finished
        }));
        
        // Update next page information
        setNextPageUrl(next);
        setMoreAvailable(!!next);
        setPagesFetched(prev => prev + 1);
        
        // Either append or replace the job list
        if (append) {
          setJobs(prevJobs => {
            const combined = [...prevJobs, ...processedJobs];
            // Sort by timestamp descending (newest first)
            combined.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
            return combined;
          });
        } else {
          setJobs(processedJobs);
        }
        
      } else if (response.data && Array.isArray(response.data)) {
        // This is the regular non-paginated response
        // Assume this is already a processed list of jobs from our backend
        const processedJobs = response.data.map(job => ({
          id: job.id,
          operation: job.name || job.job_template_name || 'Unknown Operation',
          status: job.status || 'Unknown',
          summary: job.job_template_name ? `Template: ${job.job_template_name}` : `Job #${job.id}`,
          timestamp: job.started || job.timestamp || new Date().toISOString(),
          node: hostName,
          // Additional job info for rendering
          failed: job.failed,
          elapsed: job.elapsed,
          finished: job.finished,
          // Keep the url for linking to AWX interface
          url: job.url || null
        }));
        
        console.log(`Processed ${processedJobs.length} AWX jobs for display`);
        
        // Sort by timestamp descending (newest first)
        processedJobs.sort((a, b) => {
          return new Date(b.timestamp) - new Date(a.timestamp);
        });
        
        // Either append or replace the job list
        if (append) {
          setJobs(prevJobs => {
            const uniqueJobIds = new Set(prevJobs.map(job => job.id));
            const newJobs = processedJobs.filter(job => !uniqueJobIds.has(job.id));
            return [...prevJobs, ...newJobs];
          });
        } else {
          setJobs(processedJobs);
        }
        
        // No more pagination available since this is not a paginated response
        setMoreAvailable(false);
      } else {
        if (!append) {
          setJobs([]);
          setFilteredJobs([]);
        }
        console.warn('Unexpected response format from AWX jobs API:', response.data);
      }
      
      setError(null);
    } catch (err) {
      console.error('Error fetching AWX jobs:', err);
      
      // Store detailed error information for debugging
      const details = {
        timestamp: new Date().toISOString(),
        errorName: err.name,
        errorMessage: err.message,
        statusCode: err.response?.status,
        responseData: JSON.stringify(err.response?.data || {}),
        requestUrl: url || `/api/v1/hosts/${hostName}/awx-jobs`,
        stackTrace: err.stack
      };
      setErrorDetails(details);
      
      // Only set the error if this was the initial load
      if (!url) {
        // Handle different error cases based on response status and data
        if (err.response) {
          // Server responded with an error status code
          if (err.response.status === 404) {
            if (err.response.data?.detail?.includes("resource could not be found")) {
              setError(`AWX host ID not found in AWX. The ID in Monokit doesn't match any host in AWX.`);
            } else {
              setError('AWX jobs endpoint not found. Check server configuration.');
            }
          } else if (err.response.status === 400) {
            if (err.response.data?.error) {
              setError(err.response.data.error);
            } else {
              setError('AWX integration is not properly configured for this host.');
            }
          } else if (err.response.status === 401 || err.response.status === 403) {
            setError('Authentication error. Please log in again.');
          } else {
            setError(err.response.data?.error || `Server error (${err.response.status})`);
          }
        } else if (err.request) {
          // Request was made but no response received
          setError('No response from server. The server may be offline or the connection timed out.');
        } else {
          // Something happened while setting up the request
          setError('Failed to make request. Check your network connection.');
        }
      } else {
        // For "load more" errors, just show an alert but don't block the UI
        console.warn('Failed to load more jobs:', err.message);
        setMoreAvailable(false);
      }
    } finally {
      if (!url) {
        setLoading(false);
      } else {
        setLoadingMore(false);
      }
      
      // Update filtered jobs and total count
      if (jobs.length > 0) {
        setTotalItems(jobs.length);
        updateFilteredJobs(jobs, page, perPage);
      }
    }
  };

  // Update filtered jobs based on pagination
  const updateFilteredJobs = (allJobs, currentPage, itemsPerPage) => {
    const startIdx = (currentPage - 1) * itemsPerPage;
    const endIdx = startIdx + itemsPerPage;
    setFilteredJobs(allJobs.slice(startIdx, endIdx));
  };

  // Handle pagination changes
  const onSetPage = (_event, newPage) => {
    setPage(newPage);
    updateFilteredJobs(jobs, newPage, perPage);
  };

  const onPerPageSelect = (_event, newPerPage) => {
    setPerPage(newPerPage);
    // When changing items per page, go to the first page
    setPage(1);
    updateFilteredJobs(jobs, 1, newPerPage);
  };
  
  // Load more jobs from next page
  const handleLoadMore = () => {
    if (nextPageUrl && !loadingMore) {
      fetchJobs(nextPageUrl, true);
    }
  };

  useEffect(() => {
    if (hostName) {
      fetchJobs();
    }
  }, [hostName]);
  
  useEffect(() => {
    if (jobs.length > 0) {
      setTotalItems(jobs.length);
      updateFilteredJobs(jobs, page, perPage);
    }
  }, [jobs, page, perPage]);

  // Handle manual refresh of jobs
  const handleRefresh = () => {
    setPagesFetched(0);
    setNextPageUrl(null);
    setMoreAvailable(false);
    fetchJobs();
  };

  const columns = [
    { title: 'Operation' },
    { title: 'Status' },
    { title: 'Summary' },
    { title: 'Timestamp' },
    { title: 'Node' }
  ];

  const getStatusLabel = (status) => {
    const statusMap = {
      successful: 'success',
      failed: 'danger',
      running: 'info',
      pending: 'warning',
      canceled: 'warning'
    };
    return <Label variant="outline" color={statusMap[status.toLowerCase()] || 'default'}>{status}</Label>;
  };

  if (loading) {
    return (
      <Bullseye>
        <div style={{ textAlign: 'center' }}>
          <Spinner size="xl" />
          <p style={{ marginTop: '1rem' }}>Connecting to AWX server...</p>
        </div>
      </Bullseye>
    );
  }

  if (error) {
    // Special handling for common errors
    let errorTitle = "Error loading AWX jobs";
    let errorMessage = error;
    let iconColor = "#C9190B"; // Default red
    
    // Handle specific error messages
    if (error.includes("AWX integration is disabled")) {
      errorTitle = "AWX Integration Not Available";
      errorMessage = "AWX integration is disabled in the server configuration. Contact your administrator to enable this feature in the Monokit server settings.";
      iconColor = "#2B9AF3"; // Information blue
    } else if (error.includes("AWX Host ID not configured")) {
      errorTitle = "AWX Not Configured";
      errorMessage = "This host is not registered with AWX. Contact your administrator to configure AWX for this host.";
      iconColor = "#F0AB00"; // Warning yellow
    } else if (error.includes("authentication")) {
      errorTitle = "Authentication Error";
      errorMessage = "Failed to authenticate with the AWX server. Please check your credentials or session status.";
    } else if (error.includes("AWX API error")) {
      errorTitle = "AWX API Error";
      errorMessage = "There was an error communicating with the AWX API. The server may need to be reconfigured.";
      iconColor = "#F0AB00"; // Warning yellow
    } else if (error.includes("AWX host ID not found in AWX")) {
      errorTitle = "AWX Host ID Mismatch";
      errorMessage = "The host ID recorded in Monokit doesn't match any host in the AWX system. The AWX integration needs to be reconfigured with the correct host ID mapping.";
      iconColor = "#F0AB00"; // Warning yellow
    } else if (error.includes("Failed to connect to AWX") || error.includes("timeout")) {
      errorTitle = "AWX Connection Failed";
      errorMessage = "Could not establish a connection to the AWX server. Please check that the AWX server is running and reachable from the Monokit server.";
      iconColor = "#F0AB00"; // Warning yellow
    }
    
    return (
      <EmptyState variant="large">
        <Title headingLevel="h4" size="lg" className="pf-v5-c-empty-state__title">
          <ExclamationCircleIcon color={iconColor} /> {errorTitle}
        </Title>
        <p>{errorMessage}</p>
        
        <div style={{ marginTop: '1rem' }}>
          <Button 
            variant="primary" 
            onClick={handleRefresh}
            style={{ marginRight: '10px' }}
          >
            Retry Connection
          </Button>
          
          <Button 
            variant="link" 
            onClick={() => setShowDebugInfo(!showDebugInfo)}
          >
            {showDebugInfo ? 'Hide' : 'Show'} Technical Details
          </Button>
        </div>
        
        {showDebugInfo && (
          <div style={{ 
            marginTop: '20px', 
            textAlign: 'left',
            maxWidth: '800px',
            margin: '0 auto',
            padding: '15px',
            backgroundColor: '#f5f5f5',
            border: '1px solid #ddd',
            borderRadius: '4px',
            fontFamily: 'monospace',
            fontSize: '12px'
          }}>
            <h3 style={{ marginTop: '0' }}>AWX Integration Debug Information</h3>
            <p><strong>Host:</strong> {hostName}</p>
            <p><strong>Error Message:</strong> {error}</p>
            <p><strong>Error Type:</strong> {errorDetails.errorName}</p>
            <p><strong>Status Code:</strong> {errorDetails.statusCode}</p>
            <p><strong>Timestamp:</strong> {errorDetails.timestamp}</p>
            <p><strong>API Response Data:</strong> {errorDetails.responseData}</p>
            <p><strong>Request URL:</strong> {errorDetails.requestUrl}</p>
          </div>
        )}
      </EmptyState>
    );
  }

  if (!jobs.length) {
    return (
      <EmptyState variant="large">
        <Title headingLevel="h4" size="lg" className="pf-v5-c-empty-state__title">
          <SearchIcon /> No AWX jobs found
        </Title>
        <p>There are no job records for this host in the AWX system.</p>
        <p>This may occur if the host is not registered with AWX or no jobs have been run against it.</p>
        <Button variant="primary" onClick={handleRefresh}>
          Refresh
        </Button>
      </EmptyState>
    );
  }

  return (
    <div>
      <div style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        alignItems: 'center', 
        marginBottom: '0.5rem',
        flexWrap: 'wrap'
      }}>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <Button variant="secondary" onClick={handleRefresh}>
            Refresh Jobs
          </Button>
          <span style={{ fontSize: '0.9rem', color: '#666', display: 'flex', alignItems: 'center' }}>
            <ListIcon style={{ marginRight: '5px' }} />
            Showing {totalItems} jobs for this host
            {pagesFetched > 1 && (
              <Badge style={{ marginLeft: '8px' }}>
                {pagesFetched} pages loaded
              </Badge>
            )}
          </span>
        </div>
        <div>
          <Pagination
            itemCount={totalItems}
            perPage={perPage}
            page={page}
            onSetPage={onSetPage}
            onPerPageSelect={onPerPageSelect}
            widgetId="awx-jobs-pagination-top"
            perPageOptions={[
              { title: '5', value: 5 },
              { title: '10', value: 10 },
              { title: '20', value: 20 },
              { title: '50', value: 50 },
              { title: '100', value: 100 },
            ]}
          />
        </div>
      </div>

      <Table variant="compact" aria-label="AWX Jobs Table">
        <Thead>
          <Tr>
            {columns.map((column, index) => (
              <Th key={index}>{column.title}</Th>
            ))}
          </Tr>
        </Thead>
        <Tbody>
          {filteredJobs.map((job, rowIndex) => (
            <Tr key={rowIndex}>
              <Td>
                {job.url ? (
                  <a href={job.url} target="_blank" rel="noopener noreferrer">
                    {job.operation}
                  </a>
                ) : (
                  job.operation
                )}
              </Td>
              <Td>{getStatusLabel(job.status)}</Td>
              <Td>{job.summary}</Td>
              <Td>{format(new Date(job.timestamp), 'PPpp')}</Td>
              <Td>{job.node}</Td>
            </Tr>
          ))}
        </Tbody>
      </Table>

      <div style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        alignItems: 'center', 
        marginTop: '1rem' 
      }}>
        {moreAvailable && (
          <Button 
            variant="link" 
            onClick={handleLoadMore} 
            isLoading={loadingMore}
            isDisabled={loadingMore}
          >
            {loadingMore ? 'Loading more jobs...' : 'Load more jobs'}
          </Button>
        )}
        <div style={{ marginLeft: 'auto' }}>
          <Pagination
            itemCount={totalItems}
            perPage={perPage}
            page={page}
            onSetPage={onSetPage}
            onPerPageSelect={onPerPageSelect}
            widgetId="awx-jobs-pagination-bottom"
            perPageOptions={[
              { title: '5', value: 5 },
              { title: '10', value: 10 },
              { title: '20', value: 20 },
              { title: '50', value: 50 },
              { title: '100', value: 100 },
            ]}
          />
        </div>
      </div>
    </div>
  );
};

export default AwxJobsTable;