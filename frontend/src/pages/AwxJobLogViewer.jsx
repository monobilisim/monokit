import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTheme } from '../ThemeContext.jsx';
import {
  PageSection,
  Title,
  Button,
  Spinner,
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Card,
  CardHeader,
  CardBody,
  CardTitle,
  Split,
  SplitItem,
  Badge,
  Tooltip,
  EmptyState,
  Switch
} from '@patternfly/react-core';
import { AngleDoubleDownIcon, DownloadIcon, SyncAltIcon, ArrowLeftIcon } from '@patternfly/react-icons';
import Editor from '@monaco-editor/react';
import { getAwxJobLogs } from '../utils/api';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';

const AwxJobLogViewer = () => {
  const { hostname, jobId } = useParams();
  const navigate = useNavigate();
  const { theme } = useTheme();
  
  const [logs, setLogs] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [logFormat, setLogFormat] = useState('text');
  const [jobDetails, setJobDetails] = useState(null);
  const [focusOnHost, setFocusOnHost] = useState(true);
  const [isFiltered, setIsFiltered] = useState(true);
  
  const editorRef = useRef(null);
  const monacoRef = useRef(null);

  useEffect(() => {
    if (hostname && jobId) {
      fetchJobLogs();
      // We could also fetch job details here if needed
    }
  }, [hostname, jobId]); // Intentionally not including focusOnHost in dependencies

  // Apply auto-scroll when logs update
  useEffect(() => {
    if (autoScroll && editorRef.current && logs) {
      const lineCount = editorRef.current.getModel()?.getLineCount() || 0;
      editorRef.current.revealLine(lineCount);
    }
  }, [logs, autoScroll]);

  // Function to handle the editor mount
  const handleEditorDidMount = (editor, monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco;
    
    // Apply auto-scroll if enabled
    if (autoScroll && logs) {
      const lineCount = editor.getModel().getLineCount();
      editor.revealLine(lineCount);
    }
  };

  const fetchJobLogs = async () => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await getAwxJobLogs(hostname, jobId, focusOnHost);
      console.log('AWX job logs response:', response.data);
      
      // Extract job details if available
      if (response.data && response.data.job_id) {
        setJobDetails({
          id: response.data.job_id,
          name: response.data.job_name || `Job #${response.data.job_id}`
        });
      }
      
      // Track if the logs are filtered
      if (response.data && response.data.filtered !== undefined) {
        setIsFiltered(response.data.filtered);
      }
      
      let logsContent;
      
      if (response.data && response.data.logs) {
        // Standard format from our API with logs field
        setLogFormat('text');
        logsContent = response.data.logs;
      } else if (response.data && typeof response.data === 'object') {
        // Handle case where API returns a JSON structure
        const formattedLogs = JSON.stringify(response.data, null, 2);
        setLogFormat('json');
        logsContent = formattedLogs;
      } else {
        // Default to treating as text
        setLogFormat('text');
        logsContent = String(response.data);
      }
      
      setLogs(logsContent);
    } catch (err) {
      console.error('Error fetching AWX job logs:', err);
      setError(err.response?.data?.error || err.message || 'Failed to load job logs');
    } finally {
      setLoading(false);
    }
  };

  const handleDownload = () => {
    if (!logs) return;
    
    const jobNameSafe = jobDetails?.name?.replace(/[^a-z0-9]/gi, '_').toLowerCase() || `job_${jobId}`;
    let filename, mimeType;
    
    switch (logFormat) {
      case 'json':
        filename = `awx_${jobNameSafe}_${jobId}_logs.json`;
        mimeType = 'application/json';
        break;
      default:
        filename = `awx_${jobNameSafe}_${jobId}_logs.txt`;
        mimeType = 'text/plain';
    }
    
    const blob = new Blob([logs], { type: mimeType });
    
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const toggleAutoScroll = () => {
    const newState = !autoScroll;
    setAutoScroll(newState);
    
    // If enabling auto-scroll, immediately scroll to bottom
    if (newState && editorRef.current) {
      const lineCount = editorRef.current.getModel()?.getLineCount() || 0;
      editorRef.current.revealLine(lineCount);
    }
  };
  
  const toggleHostFocus = () => {
    const newState = !focusOnHost;
    setFocusOnHost(newState);
    // Refetch logs with the new filter setting
    fetchJobLogs();
  };

  return (
    <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <Breadcrumb>
          <BreadcrumbItem onClick={() => navigate('/hosts')}>Hosts</BreadcrumbItem>
          <BreadcrumbItem onClick={() => navigate(`/hosts/${hostname}`)}>
            {hostname}
          </BreadcrumbItem>
          <BreadcrumbItem isActive>AWX Job Logs</BreadcrumbItem>
        </Breadcrumb>
        
        <Split hasGutter>
          <SplitItem>
            <ButtonWithCenteredIcon
              variant="link"
              icon={<ArrowLeftIcon />}
              onClick={() => navigate(`/hosts/${hostname}`)}
            >
              Back to Host
            </ButtonWithCenteredIcon>
          </SplitItem>
          <SplitItem isFilled>
            <Title headingLevel="h1" size="xl">
              AWX Job Logs
              {jobDetails?.name ? (
                <Badge style={{ marginLeft: '10px' }}>{jobDetails.name}</Badge>
              ) : (
                <Badge style={{ marginLeft: '10px' }}>Job #{jobId}</Badge>
              )}
            </Title>
          </SplitItem>
        </Split>
        
        <Card>
          <CardHeader>
            <Split hasGutter>
              <SplitItem isFilled>
                <CardTitle>Log Output</CardTitle>
              </SplitItem>
              <SplitItem>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <Tooltip content="Show only host-related content and task headers (filtered)">
                    <Switch
                      id="host-focus-toggle"
                      label="Focus on host"
                      labelOff="Show all logs"
                      isChecked={focusOnHost}
                      onChange={toggleHostFocus}
                    />
                  </Tooltip>
                  
                  <Tooltip content="Toggle auto-scrolling to follow log output">
                    <Button
                      variant={autoScroll ? "primary" : "secondary"}
                      onClick={toggleAutoScroll}
                    >
                      <AngleDoubleDownIcon /> {autoScroll ? 'Auto-scroll on' : 'Auto-scroll off'}
                    </Button>
                  </Tooltip>
                  <Tooltip content="Download the full log file">
                    <Button
                      variant="secondary"
                      onClick={handleDownload}
                      isDisabled={!logs || loading}
                    >
                      <DownloadIcon /> Download
                    </Button>
                  </Tooltip>
                  <Tooltip content="Refresh log data">
                    <Button
                      variant="secondary"
                      onClick={fetchJobLogs}
                      isDisabled={loading}
                    >
                      <SyncAltIcon /> Refresh
                    </Button>
                  </Tooltip>
                </div>
              </SplitItem>
            </Split>
          </CardHeader>
          <CardBody style={{ padding: '0' }}>
            {loading ? (
              <div style={{ padding: '32px', textAlign: 'center' }}>
                <Spinner size="xl" />
                <p style={{ marginTop: '16px' }}>Loading job logs...</p>
              </div>
            ) : error ? (
              <Alert
                variant="danger"
                title="Error loading logs"
                style={{ margin: '16px' }}
                actionLinks={
                  <Button variant="secondary" onClick={fetchJobLogs}>
                    Retry
                  </Button>
                }
              >
                <p>{error}</p>
              </Alert>
            ) : !logs ? (
              <EmptyState style={{ padding: '32px' }}>
                <Title headingLevel="h4" size="lg">
                  No logs available
                </Title>
                <p>No log data was found for this job.</p>
              </EmptyState>
            ) : (
              <div style={{ height: "calc(100vh - 250px)", minHeight: "500px" }}>
                <Editor
                  height="100%"
                  language={logFormat === 'json' ? 'json' : 'plaintext'}
                  value={logs}
                  theme="vs-dark"
                  options={{
                    readOnly: true,
                    scrollBeyondLastLine: false,
                    wordWrap: 'on',
                    minimap: { enabled: true },
                    lineNumbers: 'on',
                    scrollbar: {
                      vertical: 'visible',
                      horizontal: 'visible',
                      verticalScrollbarSize: 12,
                      horizontalScrollbarSize: 12
                    },
                    renderLineHighlight: 'all',
                    renderWhitespace: 'none',
                    fontFamily: 'Monaco, Menlo, "Courier New", monospace',
                    fontSize: 13,
                  }}
                  onMount={handleEditorDidMount}
                />
              </div>
            )}
          </CardBody>
        </Card>
      </div>
    </PageSection>
  );
};

export default AwxJobLogViewer;