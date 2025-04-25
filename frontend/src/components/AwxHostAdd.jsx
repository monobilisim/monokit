import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  Form,
  FormGroup,
  TextInput,
  ActionGroup,
  Alert,
  Spinner,
  Popover,
  Split,
  SplitItem,
  Title,
  Switch,
  Tooltip
} from '@patternfly/react-core';
import { PlusCircleIcon, InfoCircleIcon, HelpIcon } from '@patternfly/react-icons';
import { getAwxTemplatesGlobal, getAwxWorkflowTemplatesGlobal } from '../utils/api';
import axios from 'axios';
import CenteredIcon from './CenteredIcon';
import Editor from '@monaco-editor/react';
import jsyaml from 'js-yaml';
import ErrorBoundary from './ErrorBoundary';
import ErrorModal from './ErrorModal'; // Import the new ErrorModal

/**
 * Modal component for adding a host to AWX and validating it
 */
const AwxHostAdd = ({ isOpen, onClose, onHostAdded = () => {} }) => {
  // State management
  const [hostName, setHostName] = useState('');
  const [ipAddress, setIpAddress] = useState('');
  const [extraVars, setExtraVars] = useState('');
  const [loading, setLoading] = useState(false);
  // const [errorMessage, setErrorMessage] = useState(''); // Removed, replaced by errorModalDetails
  const [successMessage, setSuccessMessage] = useState('');
  const [hostNameError, setHostNameError] = useState('');
  const [ipAddressError, setIpAddressError] = useState('');
  const [extraVarsError, setExtraVarsError] = useState('');
  const [processingStep, setProcessingStep] = useState(''); // '', 'adding', 'validating', 'success'
  const [pingTemplateId, setPingTemplateId] = useState(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [runSetupAfterPing, setRunSetupAfterPing] = useState(false);
  const [setupTemplateId, setSetupTemplateId] = useState(null);
  const [isErrorModalOpen, setIsErrorModalOpen] = useState(false); // State for error modal visibility
  const [errorModalDetails, setErrorModalDetails] = useState({ title: '', message: '', debugInfo: null }); // State for error details

  // Reset form when opening modal
  useEffect(() => {
    if (isOpen) {
      // Reset all state
      setHostName('');
      setIpAddress('');
      setExtraVars('');
      // setErrorMessage(''); // Removed
      setSuccessMessage('');
      setHostNameError('');
      setIpAddressError('');
      setExtraVarsError('');
      setProcessingStep('');
      setShowAdvanced(false);
      setRunSetupAfterPing(false);
      setIsErrorModalOpen(false); // Reset error modal state
      setErrorModalDetails({ title: '', message: '', debugInfo: null }); // Clear error details

      // Initialize templates with hardcoded IDs
      initializeTemplates();
    }
  }, [isOpen]);

  // Initialize with hardcoded template IDs
  const initializeTemplates = async () => {
    try {
      console.log("Initializing with hardcoded template IDs...");
      
      // Use the template IDs you provided
      const pingId = 107;  // manual-check-ping
      const setupId = 95;  // workflow-manual-setup-fresh
      
      setPingTemplateId(pingId);
      setSetupTemplateId(setupId);
      
      console.log(`Using hardcoded template IDs: ping=${pingId}, setup=${setupId}`);
    } catch (err) {
      console.error("Error initializing templates:", err);
      // setErrorMessage("Error initializing templates"); // Removed
      setErrorModalDetails({ title: 'Initialization Error', message: 'Failed to initialize AWX templates.', debugInfo: err });
      setIsErrorModalOpen(true);
    }
  };

  // Validate form fields
  const validateForm = () => {
    let valid = true;
    
    // Check hostname
    if (!hostName.trim()) {
      setHostNameError("Host name is required");
      valid = false;
    } else {
      setHostNameError("");
    }
    
    // Check IP address
    if (!ipAddress.trim()) {
      setIpAddressError("IP address is required");
      valid = false;
    } else {
      setIpAddressError("");
    }
    
    // Check extra vars if present
    if (extraVars.trim()) {
      try {
        jsyaml.load(extraVars);
        setExtraVarsError("");
      } catch (error) {
        setExtraVarsError("Invalid YAML format: " + error.message);
        valid = false;
      }
    }
    
    // Check for template
    if (!pingTemplateId) {
      // setErrorMessage("The required ping template is not available"); // Removed
      setErrorModalDetails({ title: 'Configuration Error', message: 'The required ping template is not available. Please check AWX configuration.' });
      setIsErrorModalOpen(true);
      valid = false;
    }
    
    return valid;
  };

  // Removed renderAlert function as it's no longer used

  // Reset error state when modal closes
  const handleModalClose = () => {
    // setErrorMessage(''); // Removed
    setSuccessMessage('');
    setHostNameError('');
    setIpAddressError('');
    setExtraVarsError('');
    setProcessingStep('');
    onClose();
  };

  // Handle API errors consistently and trigger the error modal
  const handleApiError = (err, title = 'API Error') => {
    console.error("API Error:", err);
    
    let errorMsg = "An error occurred";
    let debugInfo = err; // Default to passing the full error object
    
    // Handle different error response formats
    if (err.response) {
      // Server responded with error status
      const status = err.response.status;
      const data = err.response.data;
      let detail = "Unknown error"; // Default detail

      // Extract the most specific error detail available
      if (data.__all__ && Array.isArray(data.__all__) && data.__all__.length > 0) {
        detail = data.__all__[0];
      } else if (data.error) {
        detail = data.error;
      } else if (data.detail) {
        detail = data.detail;
      } else if (typeof data === 'string') {
        // Sometimes the error might be a plain string in the data
        detail = data;
      } else {
         // Fallback if no specific detail is found but response exists
         // Use the extracted detail or construct a fallback
         detail = detail || `Status code ${status}`; 
      }

      // Check for specific "host already exists" error using the __all__ field
      let specificErrorFound = false;
      if (status === 400 && data?.__all__ && Array.isArray(data.__all__) && data.__all__.length > 0) {
        const specificDetail = data.__all__[0];
        // Use exact match for the known error string
        if (typeof specificDetail === 'string' && specificDetail === 'Host with this Name and Inventory already exists.') { 
          errorMsg = `Error: A host with the name "${hostName}" already exists in AWX.`;
          // Clear debug info for this known error to simplify the modal
          debugInfo = null;
          specificErrorFound = true;
        }
      } 
      
      // If the specific error wasn't found, use the general detail
      if (!specificErrorFound) {
         errorMsg = `Error: AWX API returned status: ${status} - ${detail}`;
      }

    } else if (err.request) {
      // Request was made but no response received
      errorMsg = "Error: No response from server. Please check your network connection.";
    } else if (err.message) {
      // Something else happened while setting up the request
      errorMsg = err.message;
    }

    // Ensure setErrorModalDetails is always called with a valid object
    setErrorModalDetails({
      title: title || 'Error', // Provide a default title
      message: errorMsg || 'An unexpected error occurred.', // Provide a default message
      debugInfo: debugInfo // Pass relevant debug info, which might be null
    });
    setIsErrorModalOpen(true); // Open the modal

    // Return the message for potential inline use if needed, though modal is primary
    return errorMsg; 
  };

  // Handle form submission with improved error handling
  const handleSubmit = async (event) => {
    if (event) {
      event.preventDefault();
    }
    
    // Clear any previous success messages, errors will be handled by the modal
    setSuccessMessage('');
    setIsErrorModalOpen(false); // Close any existing error modal
    
    // Validate form
    if (!validateForm()) {
      return;
    }
    
    setLoading(true);
    setProcessingStep('adding');
    
    try {
      // Step 1: Create host in AWX
      console.log("Creating host in AWX...");
      
      let createdHost;
      try {
        // Prepare request payload
        const payload = {
          name: hostName,
          ip_address: ipAddress,
          awx_only: true
        };
        
        // Add extra variables if provided
        if (extraVars.trim()) {
          try {
            const parsedVars = jsyaml.load(extraVars);
            if (parsedVars && typeof parsedVars === 'object') {
              payload.extra_vars = parsedVars;
            }
          } catch (err) {
            console.error("Failed to parse extra vars:", err);
            throw new Error("Invalid YAML in extra variables");
          }
        }
        
        console.log("Creating AWX host with payload:", payload);
        
        try {
          const createResponse = await axios.post('/api/v1/hosts/awx', payload, {
            headers: {
              'Authorization': localStorage.getItem('token'),
              'Content-Type': 'application/json'
            }
          });
          
          if (!createResponse.data || !createResponse.data.id) {
            throw new Error("Invalid response when creating host");
          }
          
          createdHost = createResponse.data;
          console.log("Host created successfully:", createdHost);
        } catch (axiosError) {
          // Handle specific Axios errors using the updated function
          handleApiError(axiosError, 'Failed to Create Host');
          // No need to throw new Error here, modal is opened by handleApiError
          return; // Stop execution after showing the error modal
        }
      } catch (err) {
        // Catch errors from parsing YAML or other setup issues before API call
        console.error("Failed during host preparation:", err);
        setErrorModalDetails({ title: 'Preparation Error', message: err.message || 'Failed to prepare host data.', debugInfo: err });
        setIsErrorModalOpen(true);
        setLoading(false); // Ensure loading stops
        return; // Stop execution
      }
      
      // Step 2: Run ping job
      console.log("Running ping job...");
      setProcessingStep('validating');
      
      let jobId;
      try {
        const jobResponse = await axios.post(`/api/v1/hosts/${hostName}/awx-jobs/execute`, {
          template_id: pingTemplateId,
          extra_vars: { limit: hostName }
        }, {
          headers: {
            'Authorization': localStorage.getItem('token'),
            'Content-Type': 'application/json'
          }
        });
        
        jobId = jobResponse.data.job_id || jobResponse.data.id;
        
        if (!jobId) {
          throw new Error("No job ID returned from AWX");
        }
        
        console.log("Ping job launched with ID:", jobId);
      } catch (err) {
        console.error("Failed to run ping job:", err);
        
        // Clean up by deleting the host
        if (createdHost && createdHost.id) {
          try {
            await axios.delete(`/api/v1/hosts/awx/${createdHost.id}`, {
              headers: { 'Authorization': localStorage.getItem('token') }
            });
            console.log("Deleted host due to job launch failure");
          } catch (deleteErr) {
            console.error("Failed to delete host after job launch failure:", deleteErr);
          }
        }
        
        if (err.response?.data?.error && typeof err.response.data.error === 'string') {
          throw new Error(err.response.data.error);
        } else if (err.message && typeof err.message === 'string') {
          throw new Error(err.message);
        } else {
          // Use handleApiError to show modal
          handleApiError(err, 'Failed to Launch Ping Job');
          // No need to throw, modal handles it. Ensure cleanup happens if needed.
          // The cleanup logic is already present before this point.
          setLoading(false); // Ensure loading stops
          return; // Stop execution
        }
      }
      
      // Step 3: Poll for job status
      console.log("Polling for job status...");
      let attempts = 0;
      const maxAttempts = 20;
      
      while (attempts < maxAttempts) {
        attempts++;
        console.log(`Checking job status (attempt ${attempts}/${maxAttempts})...`);
        
        // Wait 3 seconds between checks
        await new Promise(resolve => setTimeout(resolve, 3000));
        
        try {
          const statusResponse = await axios.get(`/api/v1/awx/jobs/${jobId}`, {
            headers: { 'Authorization': localStorage.getItem('token') }
          });
          
          const status = statusResponse.data.status;
          console.log(`Job status: ${status}`);
          
          if (status === 'successful') {
            // Ping test succeeded!
            
            // If setup toggle is enabled, run manual-setup-fresh-install
            if (runSetupAfterPing && setupTemplateId) {
              console.log("Ping successful! Now running workflow-manual-setup-fresh...");
              setProcessingStep('setup');
              setSuccessMessage(`Host "${hostName}" passed ping test, now running setup workflow...`);
              
              try {
                // Use the workflow-specific endpoint for this workflow template
                const setupResponse = await axios.post(`/api/v1/hosts/${hostName}/awx-workflow-jobs/execute`, {
                  workflow_template_id: setupTemplateId,
                  extra_vars: { limit: hostName },
                  format: "yaml"
                }, {
                  headers: {
                    'Authorization': localStorage.getItem('token'),
                    'Content-Type': 'application/json'
                  }
                });
                
                const setupJobId = setupResponse?.data?.job_id || setupResponse?.data?.id;
                
                if (!setupJobId) {
                  throw new Error("No job ID returned from AWX for setup job");
                }
                
                console.log("Setup job launched with ID:", setupJobId);
                
                // We won't wait for this job to complete
                setProcessingStep('success');
                setSuccessMessage(`Host "${hostName}" successfully added to AWX and setup job launched`);
              } catch (setupErr) {
                console.error("Failed to run setup job:", setupErr);
                // We still consider this a success since the ping passed
                setProcessingStep('success');
                setSuccessMessage(`Host "${hostName}" added to AWX, but setup job failed to launch: ${setupErr.message}`);
              }
            } else {
              // Just ping test was needed and it succeeded
              setProcessingStep('success');
              setSuccessMessage(`Host "${hostName}" was successfully added to AWX`);
            }
            
            // Create a host object but don't add it to the database
            const awxHost = {
              name: hostName,
              ipAddress: ipAddress,
              awxHostId: String(createdHost.id),
              awxHostUrl: createdHost.url || ''
            };
            
            console.log("Host created in AWX only (not added to local dashboard):", awxHost);
            
            // Safe call to the callback - make sure it exists and wrap in try/catch
            try {
              if (typeof onHostAdded === 'function') {
                onHostAdded({
                  awxOnly: true,
                  hostName: hostName,
                  ipAddress: ipAddress,
                  awxHostId: String(createdHost.id)
                });
              }
            } catch (callbackError) {
              console.error("Error in onHostAdded callback:", callbackError);
              // Don't let callback errors affect the UI
            }
            
            break;
          } else if (['failed', 'error', 'canceled'].includes(status)) {
            // Job failed, delete the host
            console.log("Ping check failed, deleting host");
            
            if (createdHost && createdHost.id) {
              try {
                await axios.delete(`/api/v1/hosts/awx/${createdHost.id}`, {
                  headers: { 'Authorization': localStorage.getItem('token') }
                });
                console.log("Deleted host due to failed ping check");
              } catch (deleteErr) {
                console.error("Failed to delete host after failed ping check:", deleteErr);
              }
            }
            
            // Use handleApiError for job failure
            handleApiError(new Error(`Host validation failed. The host could not be reached. Job status: ${status}`), 'Host Validation Failed');
            setLoading(false);
            return; // Stop polling
          }
          
          // Otherwise continue polling
        } catch (pollErr) {
          console.error("Error checking job status:", pollErr);
          // Use handleApiError for polling errors
          handleApiError(pollErr, 'Error Checking Job Status');
          setLoading(false);
          return; // Stop polling
        }
      }
      
      if (attempts >= maxAttempts) {
        // Use handleApiError for timeout
        handleApiError(new Error("Validation timed out after multiple attempts."), 'Validation Timeout');
        setLoading(false);
        return; // Stop execution
      }
    } catch (err) {
      // This top-level catch should ideally not be reached if specific errors are handled
      // But keep it as a fallback
      console.error("Unhandled process error:", err);
      handleApiError(err, 'Unexpected Error');
      setProcessingStep(''); // Reset processing step on unexpected error
    } finally {
      // Ensure loading is always turned off, even if errors occurred early
      setLoading(false); 
    }
  };

  return (
    <Modal
      variant={ModalVariant.medium}
      title="Add Host to AWX"
      titleIconVariant={
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <CenteredIcon 
            icon={<PlusCircleIcon style={{ color: '#0066CC' }} />} 
            style={{ marginRight: '10px' }} 
          />
        </div>
      }
      isOpen={isOpen}
      onClose={handleModalClose}
      appendTo={document.body}
      showClose={true}
      hasNoBodyWrapper={false}
      style={{ borderRadius: '8px', overflow: 'hidden' }}
    >
      <ErrorBoundary
        // Update ErrorBoundary reset logic if needed, though ErrorModal handles display
        onReset={() => {
          // setErrorMessage(''); // Removed
          setProcessingStep('');
          setLoading(false);
          setIsErrorModalOpen(false); // Also close error modal on reset
        }}
        fallback={
          <div style={{ padding: '20px', textAlign: 'center' }}>
            <Button 
              variant="primary" 
              onClick={handleModalClose}
            >
              Close and Try Again
            </Button>
          </div>
        }
      >
        <div style={{ padding: '24px 32px' }}>
          {/* Removed inline error message rendering */}
          
          {/* Success message - Keep using Alert for success */}
          {successMessage && (
            <Alert 
              variant="success" 
              title="Success"
              isInline 
              style={{ marginBottom: '16px' }}
            >
              {successMessage}
            </Alert>
          )}
          
          {/* Status alerts - Keep using Alert for status updates */}
          {processingStep === 'validating' && 
            <Alert 
              variant="info" 
              title="Validating Host"
              isInline 
              style={{ marginBottom: '16px' }}
            >
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <Spinner size="md" style={{ marginRight: '10px' }} />
                <span>Running ping check on the new host. This may take a few moments...</span>
              </div>, 
            </Alert>
          }
          
          {processingStep === 'setup' && 
            <Alert 
              variant="info" 
              title="Running Setup Job"
              isInline 
              style={{ marginBottom: '16px' }}
            >
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <Spinner size="md" style={{ marginRight: '10px' }} />
                <span>Running workflow-manual-setup-fresh on the new host...</span>
              </div>, 
            </Alert>
          }
          
          <Form onSubmit={handleSubmit}>
            {/* Host Name field */}
            <FormGroup 
              label="Host Name" 
              isRequired 
              fieldId="hostname"
            >
              <TextInput
                isRequired
                type="text"
                id="hostname"
                name="hostname"
                value={hostName}
                onChange={(event, value) => {
                  setHostName(value);
                  if (hostNameError) setHostNameError('');
                }}
                placeholder="Enter host name"
                isDisabled={loading || processingStep === 'success'}
                validated={hostNameError ? 'error' : 'default'}
              />
              {hostNameError && (
                <div style={{ color: 'red', fontSize: '14px', marginTop: '4px' }}>
                  {hostNameError}
                </div>
              )}
            </FormGroup>
            
            {/* IP Address field */}
            <FormGroup 
              label="IP Address" 
              isRequired 
              fieldId="ipaddress"
            >
              <TextInput
                isRequired
                type="text"
                id="ipaddress"
                name="ipaddress"
                value={ipAddress}
                onChange={(event, value) => {
                  setIpAddress(value);
                  if (ipAddressError) setIpAddressError('');
                }}
                placeholder="Enter IP address"
                isDisabled={loading || processingStep === 'success'}
                validated={ipAddressError ? 'error' : 'default'}
              />
              {ipAddressError && (
                <div style={{ color: 'red', fontSize: '14px', marginTop: '4px' }}>
                  {ipAddressError}
                </div>
              )}
            </FormGroup>
            
            {/* Advanced Settings Button */}
            <Button 
              variant="link" 
              onClick={() => setShowAdvanced(!showAdvanced)}
              style={{ marginBottom: '16px' }}
            >
              {showAdvanced ? 'Hide Advanced Options' : 'Show Advanced Options'}
            </Button>
            
            {/* Extra Variables Editor - only visible when advanced is toggled */}
            {showAdvanced && (
              <FormGroup 
                label={
                  <div style={{ display: 'flex', alignItems: 'center' }}>
                    <span>Extra Variables (YAML format)</span>
                    <Popover
                      headerContent={<div>Extra Variables</div>}
                      bodyContent={
                        <div>
                          <p>Specify additional variables for the host in YAML format.</p>
                          <p>These variables will be added to the host in AWX and can be used in playbooks.</p>
                          <p><strong>Example:</strong></p>
                          <pre style={{ backgroundColor: '#f0f0f0', padding: '8px', borderRadius: '4px' }}>
                            {`ansible_ssh_user: admin\nansible_ssh_port: 22\ndescription: Production server`}
                          </pre>
                        </div>
                      }
                    >
                      <Button variant="plain" aria-label="Help" style={{ padding: '0 8px' }}>
                        <HelpIcon />
                      </Button>
                    </Popover>
                  </div>
                }
                fieldId="extra-vars"
                helperText={
                  extraVars.trim() ? 
                    "These variables will be combined with the basic host configuration." :
                    "Optional. Leave empty to use default host configuration."
                }
                helperTextInvalid={extraVarsError}
                validated={extraVarsError ? 'error' : 'default'}
              >
                <div style={{ borderRadius: '3px', overflow: 'hidden', border: '1px solid var(--pf-global--BorderColor--100, #d2d2d2)', marginBottom: '8px' }}>
                  <Editor
                    height="200px"
                    defaultLanguage="yaml"
                    language="yaml"
                    value={extraVars}
                    onChange={(value) => {
                      setExtraVars(value || '');
                      if (extraVarsError) {
                        // Clear error when user makes changes
                        setExtraVarsError('');
                      }
                    }}
                    options={{
                      minimap: { enabled: false },
                      scrollBeyondLastLine: false,
                      fontSize: 14,
                      readOnly: loading,
                      wordWrap: 'on',
                      lineNumbers: 'on',
                      folding: true,
                      automaticLayout: true,
                      tabSize: 2
                    }}
                    theme="vs-dark"
                  />
                </div>
                {extraVarsError && (
                  <div style={{ color: 'red', fontSize: '14px', marginTop: '4px' }}>
                    {extraVarsError}
                  </div>
                )}
              </FormGroup>
            )}
            
            {/* Run Setup Toggle */}
            <div style={{ margin: '16px 0 24px 0', padding: '16px', backgroundColor: '#252525', borderRadius: '4px', color: '#f0f0f0' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div>
                  <Title headingLevel="h4" size="md" style={{ marginBottom: '8px', color: '#f8f8f8' }}>Run Fresh Install Setup</Title>
                  <p style={{ fontSize: '14px', color: '#d0d0d0' }}>
                    After ping test succeeds, run the <strong>workflow-manual-setup-fresh</strong> job template on this host
                  </p>
                </div>
                <Tooltip 
                  content={
                    setupTemplateId ? 
                    "Run 'workflow-manual-setup-fresh' job template on this host after ping verification" :
                    "Job template 'workflow-manual-setup-fresh' not found in AWX"
                  }
                >
                  <div>
                    <style>
                      {`
                      /* Custom dark styling for the switch */
                      .pf-c-switch__toggle {
                        background-color: #3c3c3c !important;
                        border-color: #5a5a5a !important;
                      }
                      .pf-c-switch__toggle::before {
                        background-color: #dedede !important;
                      }
                      .pf-c-switch__input:checked ~ .pf-c-switch__toggle {
                        background-color: #0066cc !important;
                      }
                      `}
                    </style>
                    <Switch
                      id="run-setup"
                      aria-label="Run setup after ping"
                      isChecked={runSetupAfterPing}
                      onChange={() => setRunSetupAfterPing(!runSetupAfterPing)}
                      isDisabled={!setupTemplateId || loading || processingStep === 'success'}
                    />
                  </div>
                </Tooltip>
              </div>
              {!setupTemplateId && (
                <div style={{ marginTop: '8px', color: '#ff6b6b', fontSize: '14px' }}>
                  Warning: The "workflow-manual-setup-fresh" job template was not found in AWX
                </div>
              )}
            </div>
            
            {/* Action buttons */}
            <ActionGroup style={{ marginTop: '16px' }}>
              <Button 
                variant="primary" 
                onClick={handleSubmit}
                isDisabled={loading || processingStep === 'success' || !pingTemplateId}
                isLoading={loading}
                type="submit"
              >
                {loading ? 'Processing...' : 'Add Host'}
              </Button>
              
              <Button 
                variant="link" 
                onClick={onClose}
                isDisabled={loading && processingStep !== 'success'}
              >
                {processingStep === 'success' ? 'Close' : 'Cancel'}
              </Button>
            </ActionGroup>
          </Form>
        </div>
      </ErrorBoundary>
      {/* Add bottom padding */}
      <div style={{ padding: '12px' }}></div>

      {/* Render the ErrorModal */}
      <ErrorModal
        isOpen={isErrorModalOpen}
        onClose={() => setIsErrorModalOpen(false)}
        title={errorModalDetails.title}
        message={errorModalDetails.message}
        debugInfo={errorModalDetails.debugInfo}
      />
    </Modal>
  );
};

export default AwxHostAdd;
