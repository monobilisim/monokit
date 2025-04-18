import React, { useState, useEffect, useRef } from 'react';
import {
  Spinner,
  Button,
  Card,
  CardTitle,
  CardBody,
  TextContent,
  Text,
  TextVariants,
  Modal,
  ModalVariant,
  Bullseye,
  Alert,
  EmptyState,
  Title,
  ClipboardCopy,
  ClipboardCopyVariant,
  Badge,
  Flex,
  FlexItem
} from '@patternfly/react-core';
import { ExclamationCircleIcon, DownloadIcon, AngleDoubleDownIcon, TimesIcon } from '@patternfly/react-icons';
import Editor from '@monaco-editor/react';
import { getAwxJobLogs } from '../utils/api';

const AwxJobLogs = ({ isOpen, onClose, jobId, hostname, jobName }) => {
  const [logs, setLogs] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [logFormat, setLogFormat] = useState('text');
  const editorRef = useRef(null);
  const monacoRef = useRef(null);

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

  useEffect(() => {
    if (isOpen && jobId) {
      fetchJobLogs();
    }
  }, [isOpen, jobId, hostname]);

  // Apply auto-scroll when logs update
  useEffect(() => {
    if (autoScroll && editorRef.current && logs) {
      const lineCount = editorRef.current.getModel()?.getLineCount() || 0;
      editorRef.current.revealLine(lineCount);
    }
  }, [logs, autoScroll]);

  const fetchJobLogs = async () => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await getAwxJobLogs(hostname, jobId);
      console.log('AWX job logs response:', response.data);
      
      // We expect the response to be plain text from the backend
      // since we've modified the API to request format=txt_download
      if (response.data && response.data.logs) {
        // Standard format from our API with logs field
        setLogFormat('text');
        setLogs(response.data.logs);
      } else if (response.data && typeof response.data === 'object') {
        // Handle case where API returns a JSON structure
        const formattedLogs = JSON.stringify(response.data, null, 2);
        setLogFormat('json');
        setLogs(formattedLogs);
      } else {
        // Default to treating as text
        setLogFormat('text');
        setLogs(String(response.data));
      }
    } catch (err) {
      console.error('Error fetching AWX job logs:', err);
      setError(err.response?.data?.error || err.message || 'Failed to load job logs');
    } finally {
      setLoading(false);
    }
  };

  const handleDownload = () => {
    if (!logs) return;
    
    const jobNameSafe = jobName?.replace(/[^a-z0-9]/gi, '_').toLowerCase() || 'job';
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

  return (
    <Modal
      variant={ModalVariant.large}
      title={`AWX Job Logs #${jobId} ${jobName ? `- ${jobName}` : ''} (${logFormat === 'json' ? 'JSON' : 'Text'})`}
      isOpen={isOpen}
      onClose={onClose}
      actions={[
        <Button key="download" variant="secondary" onClick={handleDownload} isDisabled={!logs || loading}>
          <DownloadIcon /> Download Logs
        </Button>,
        <Button key="refresh" variant="secondary" onClick={fetchJobLogs} isDisabled={loading}>
          Refresh
        </Button>,
        <Button key="close" variant="link" onClick={onClose}>
          Close
        </Button>
      ]}
    >
      <div style={{ position: 'relative' }}>
        {loading ? (
          <Bullseye>
            <Spinner size="xl" />
          </Bullseye>
        ) : error ? (
          <Alert variant="danger" title="Error loading logs" isInline>
            <p>{error}</p>
            <Button variant="secondary" onClick={fetchJobLogs}>
              Retry
            </Button>
          </Alert>
        ) : !logs ? (
          <EmptyState>
            <Title headingLevel="h4" size="lg">
              No logs available
            </Title>
            <p>No log data was found for this job.</p>
          </EmptyState>
        ) : (
          <>
            <div style={{ marginBottom: '10px' }}>
              <Button 
                variant={autoScroll ? "primary" : "secondary"}
                onClick={toggleAutoScroll}
                style={{ float: 'right' }}
              >
                <AngleDoubleDownIcon /> {autoScroll ? 'Auto-scroll on' : 'Auto-scroll off'}
              </Button>
            </div>
            
            <div style={{ 
              height: "550px", 
              border: '1px solid #333', 
              clear: 'both',
              position: 'relative'
            }}>
              <Editor
                height="550px"
                language={logFormat === 'json' ? 'json' : 'plaintext'}
                value={logs}
                theme="vs-dark"
                options={{
                  readOnly: true,
                  scrollBeyondLastLine: false,
                  wordWrap: 'on',
                  minimap: { enabled: false }, // Disable minimap for cleaner view
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
          </>
        )}
      </div>
    </Modal>
  );
};

export default AwxJobLogs;