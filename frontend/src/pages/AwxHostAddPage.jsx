import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  PageSection,
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
  Tooltip,
  Breadcrumb,
  BreadcrumbItem,
  Card,
  CardBody,
  CardTitle,
  Stack,
  StackItem
} from '@patternfly/react-core';
import { AlertActionCloseButton } from '@patternfly/react-core/dist/js/components/Alert/AlertActionCloseButton';
import { PlusCircleIcon, InfoCircleIcon, HelpIcon, ArrowLeftIcon } from '@patternfly/react-icons';
import { getAwxTemplatesGlobal, getAwxWorkflowTemplatesGlobal, createAwxHost } from '../utils/api';
import axios from 'axios';
import CenteredIcon from '../components/CenteredIcon';
import Editor from '@monaco-editor/react';
import jsyaml from 'js-yaml';
import { useTheme } from '../ThemeContext.jsx';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';

/**
 * Page component for adding a host to AWX and validating it
 */
const AwxHostAddPage = ({ onAuthError }) => {
  const navigate = useNavigate();
  const { theme } = useTheme();
  
  // State management
  const [hostName, setHostName] = useState('');
  const [ipAddress, setIpAddress] = useState('');
  const [extraVars, setExtraVars] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const [successMessage, setSuccessMessage] = useState('');
  const [hostNameError, setHostNameError] = useState('');
  const [ipAddressError, setIpAddressError] = useState('');
  const [extraVarsError, setExtraVarsError] = useState('');
  const [processingStep, setProcessingStep] = useState(''); // '', 'adding', 'validating', 'success'
  const [pingTemplateId, setPingTemplateId] = useState(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [runSetupAfterPing, setRunSetupAfterPing] = useState(false);
  const [setupTemplateId, setSetupTemplateId] = useState(null);
  const [manualInstallMonokitClient, setManualInstallMonokitClient] = useState(false);
  const [manualInstallTemplateId, setManualInstallTemplateId] = useState(null);
  const [alerts, setAlerts] = useState([]);

  const addAlert = (title, variant, description = '') => {
    const key = new Date().getTime();
    setAlerts([...alerts, { title, variant, key, description }]);

    // Automatically remove the alert after 5 seconds
    setTimeout(() => {
      setAlerts(currentAlerts => currentAlerts.filter(alert => alert.key !== key));
    }, 5000);
  };

  // Initialize component on mount
  useEffect(() => {
    // Reset all state
    setHostName('');
    setIpAddress('');
    setExtraVars('');
    setErrorMessage('');
    setSuccessMessage('');
    setHostNameError('');
    setIpAddressError('');
    setExtraVarsError('');
    setProcessingStep('');
    setShowAdvanced(false);
    setRunSetupAfterPing(false);
    
    // Initialize templates with hardcoded IDs
    initializeTemplates();
  }, []);

  // Initialize with hardcoded template IDs
  const initializeTemplates = async () => {
    try {
      console.log("Initializing with hardcoded template IDs...");
      
      // Use the template IDs you provided
      const pingId = 107;  // manual-check-ping
      const setupId = 95;  // workflow-manual-setup-fresh
      const manualInstallId = 258;  // manual-install-monokit-client
      
      setPingTemplateId(pingId);
      setSetupTemplateId(setupId);
      setManualInstallTemplateId(manualInstallId);
      
      console.log(`Using hardcoded template IDs: ping=${pingId}, setup=${setupId}, manual-install=${manualInstallId}`);
    } catch (err) {
      console.error("Error initializing templates:", err);
      setErrorMessage("Error initializing templates");
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
      setErrorMessage("The required ping template is not available");
      valid = false;
    }
    
    return valid;
  };

  // Handle form submission
  const handleSubmit = async (event) => {
    if (event) {
      event.preventDefault();
    }
    
    // Clear any previous errors
    setErrorMessage('');
    setSuccessMessage('');
    
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
          awx_only: true // Always set to true - we want to add to AWX only, not to local dashboard
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
      } catch (err) {
        console.error("Failed to create host:", err);
        
        if (err.response?.status === 502) {
          throw new Error("Cannot connect to AWX (502 Bad Gateway)");
        } else if (err.response?.data?.error && typeof err.response.data.error === 'string') {
          throw new Error(err.response.data.error);
        } else if (err.message && typeof err.message === 'string') {
          throw new Error(err.message);
        } else {
          throw new Error("Failed to create host in AWX");
        }
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
          throw new Error("Failed to launch ping job");
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
            let setupSuccess = false;
            let manualInstallSuccess = false;
            let messages = [];
            
            // Track which jobs were launched to create an appropriate final success message
            const jobsLaunched = [];

            // Run setup workflow if enabled
            if (runSetupAfterPing && setupTemplateId) {
              console.log("Ping successful! Now running workflow-manual-setup-fresh...");
              setProcessingStep('setup');
              
              try {
                // Use the workflow-specific endpoint for this workflow template
                const setupResponse = await axios.post(`/api/v1/hosts/${hostName}/awx-workflow-jobs/execute`, {
                  workflow_template_id: setupTemplateId,
                  extra_vars: { limit: hostName },
                  format: "yaml",
                  is_workflow: true
                }, {
                  headers: {
                    'Authorization': localStorage.getItem('token'),
                    'Content-Type': 'application/json'
                  }
                });
                
                const setupJobId = setupResponse.data.job_id || setupResponse.data.id;
                
                if (!setupJobId) {
                  throw new Error("No job ID returned from AWX for setup job");
                }
                
                console.log("Setup job launched with ID:", setupJobId);
                jobsLaunched.push('setup');
                setupSuccess = true;
                addAlert(`Setup job launched for "${hostName}"`, 'success');
              } catch (setupErr) {
                console.error("Failed to run setup job:", setupErr);
                addAlert(`Setup job failed to launch for "${hostName}"`, 'warning', setupErr.message);
                messages.push(`Setup job failed to launch: ${setupErr.message}`);
              }
            }
            
            // Run manual install if enabled
            if (manualInstallMonokitClient && manualInstallTemplateId) {
              console.log("Now running manual-install-monokit-client...");
              setProcessingStep('setup');
              
              try {
                const manualInstallResponse = await axios.post(`/api/v1/hosts/${hostName}/awx-jobs/execute`, {
                  template_id: manualInstallTemplateId,
                  extra_vars: { limit: hostName }
                }, {
                  headers: {
                    'Authorization': localStorage.getItem('token'),
                    'Content-Type': 'application/json'
                  }
                });
                
                const manualInstallJobId = manualInstallResponse.data.job_id || manualInstallResponse.data.id;
                
                if (!manualInstallJobId) {
                  throw new Error("No job ID returned from AWX for manual install job");
                }
                
                console.log("Manual install job launched with ID:", manualInstallJobId);
                jobsLaunched.push('monokit client installation');
                manualInstallSuccess = true;
                addAlert(`Monokit client installation launched for "${hostName}"`, 'success');
              } catch (manualInstallErr) {
                console.error("Failed to run manual install job:", manualInstallErr);
                addAlert(`Monokit client installation failed to launch for "${hostName}"`, 'warning', manualInstallErr.message);
                messages.push(`Monokit client installation failed to launch: ${manualInstallErr.message}`);
              }
            }
            
            // Create final success message
            setProcessingStep('success');
            
            if (jobsLaunched.length === 0) {
              // No jobs were requested or launched
              setSuccessMessage(`Host "${hostName}" was successfully added to AWX`);
              addAlert(`Host "${hostName}" successfully added to AWX`, 'success');
            } else {
              // One or more jobs were launched
              const jobsText = jobsLaunched.join(' and ');
              setSuccessMessage(`Host "${hostName}" successfully added to AWX and ${jobsText} launched`);
              
              // Only add a summary alert if we haven't already sent individual alerts
              if (messages.length > 0) {
                addAlert(`Host "${hostName}" added with some issues`, 'warning', messages.join('; '));
              }
            }
            
            // Create a host object but don't add it to the database
            const awxHost = {
              name: hostName,
              ipAddress: ipAddress,
              awxHostId: String(createdHost.id),
              awxHostUrl: createdHost.url || ''
            };
            
            console.log("Host created in AWX only (not added to local dashboard):", awxHost);
            
            // After a short delay, redirect to hosts page
            setTimeout(() => {
              navigate('/hosts');
            }, 3000);
            
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
            
            throw new Error(`Host validation failed. The host could not be reached.`);
          }
          
          // Otherwise continue polling
        } catch (err) {
          console.error("Error checking job status:", err);
          
          if (err.response?.status === 502) {
            throw new Error("Lost connection to AWX server (502 Bad Gateway)");
          } else if (err.message && typeof err.message === 'string') {
            throw new Error(err.message);
          } else {
            throw new Error("Error checking job status");
          }
        }
      }
      
      if (attempts >= maxAttempts) {
        throw new Error("Validation timed out. Please check AWX for job status.");
      }
    } catch (err) {
      console.error("Overall process error:", err);
      
      // Ensure we have a string error message
      if (typeof err === 'string') {
        setErrorMessage(err);
      } else if (err.message && typeof err.message === 'string') {
        setErrorMessage(err.message);
      } else {
        setErrorMessage("An unexpected error occurred");
      }
      
      addAlert("Error adding host to AWX", 'danger', err.message || "An unexpected error occurred");
      setProcessingStep('');
    } finally {
      setLoading(false);
    }
  };

  // JSX styles for light/dark mode
  const containerStyles = {
    backgroundColor: theme === 'dark' ? '#212427' : '#ffffff'
  };

  const cardStyles = {
    backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
    border: theme === 'dark' ? '1px solid #444' : '1px solid #d2d2d2'
  };

  return (
    <PageSection style={containerStyles}>
      {alerts.map(({key, title, variant, description}) => (
        <Alert
          key={key}
          variant={variant}
          title={title}
          actionClose={
            <AlertActionCloseButton
              title={title}
              onClose={() => setAlerts(alerts.filter(alert => alert.key !== key))}
            />
          }
          style={{ marginBottom: '1rem' }}
        >
          {description}
        </Alert>
      ))}

      <Stack hasGutter>
        <StackItem>
          <Breadcrumb>
            <BreadcrumbItem onClick={() => navigate('/hosts')}>Hosts</BreadcrumbItem>
            <BreadcrumbItem isActive>Add Host to AWX</BreadcrumbItem>
          </Breadcrumb>
        </StackItem>

        <StackItem>
          <Split hasGutter>
            <SplitItem>
              <ButtonWithCenteredIcon
                variant="link"
                icon={<ArrowLeftIcon />}
                onClick={() => navigate('/hosts')}
              >
                Back to Hosts
              </ButtonWithCenteredIcon>
            </SplitItem>
            <SplitItem isFilled>
              <Title headingLevel="h1" size="xl">
                <CenteredIcon 
                  icon={<PlusCircleIcon style={{ color: '#0066CC' }} />} 
                  style={{ marginRight: '10px' }} 
                />
                Add Host to AWX
              </Title>
            </SplitItem>
          </Split>
        </StackItem>

        <StackItem>
          <Card style={cardStyles}>
            <CardBody>
              {/* Error message */}
              {errorMessage && (
                <Alert 
                  variant="danger" 
                  title="Error" 
                  isInline 
                  style={{ marginBottom: '16px' }}
                >
                  {errorMessage}
                  {errorMessage.includes("default inventory ID") && (
                    <div style={{ marginTop: '8px' }}>
                      <strong>Administrator action required:</strong> The server needs a default AWX inventory ID configured.
                      Please update the <code>default_inventory_id</code> in the server configuration file.
                    </div>
                  )}
                </Alert>
              )}
              
              {/* Success message */}
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
              
              {/* Status alerts */}
              {processingStep === 'validating' && (
                <Alert 
                  variant="info" 
                  title="Validating Host" 
                  isInline 
                  style={{ marginBottom: '16px' }}
                >
                  <div style={{ display: 'flex', alignItems: 'center' }}>
                    <Spinner size="md" style={{ marginRight: '10px' }} />
                    <span>Running ping check on the new host. This may take a few moments...</span>
                  </div>
                </Alert>
              )}
              
              {processingStep === 'setup' && (
                <Alert 
                  variant="info" 
                  title="Running Setup Job" 
                  isInline 
                  style={{ marginBottom: '16px' }}
                >
                  <div style={{ display: 'flex', alignItems: 'center' }}>
                    <Spinner size="md" style={{ marginRight: '10px' }} />
                    <span>Running workflow-manual-setup-fresh on the new host...</span>
                  </div>
                </Alert>
              )}
              
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
                          headerContent="Extra Variables"
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
                <div style={{ margin: '16px 0 24px 0', padding: '16px', backgroundColor: theme === 'dark' ? '#252525' : '#f5f5f5', borderRadius: '4px', color: theme === 'dark' ? '#f0f0f0' : '#333' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div>
                      <Title headingLevel="h4" size="md" style={{ marginBottom: '8px', color: theme === 'dark' ? '#f8f8f8' : '#333' }}>Run Fresh Install Setup</Title>
                      <p style={{ fontSize: '14px', color: theme === 'dark' ? '#d0d0d0' : '#666' }}>
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
                          /* Custom styling for the switch */
                          .pf-c-switch__toggle {
                            background-color: ${theme === 'dark' ? '#3c3c3c' : '#f0f0f0'} !important;
                            border-color: ${theme === 'dark' ? '#5a5a5a' : '#ccc'} !important;
                          }
                          .pf-c-switch__toggle::before {
                            background-color: ${theme === 'dark' ? '#dedede' : '#fff'} !important;
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
                    <div style={{ marginTop: '8px', color: theme === 'dark' ? '#ff6b6b' : '#c9190b', fontSize: '14px' }}>
                      Warning: The "workflow-manual-setup-fresh" job template was not found in AWX
                    </div>
                  )}
                </div>
                
                {/* Manual Install Monokit Client Toggle */}
                <div style={{ margin: '16px 0 24px 0', padding: '16px', backgroundColor: theme === 'dark' ? '#252525' : '#f5f5f5', borderRadius: '4px', color: theme === 'dark' ? '#f0f0f0' : '#333' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div>
                      <Title headingLevel="h4" size="md" style={{ marginBottom: '8px', color: theme === 'dark' ? '#f8f8f8' : '#333' }}>Manual Install Monokit Client</Title>
                      <p style={{ fontSize: '14px', color: theme === 'dark' ? '#d0d0d0' : '#666' }}>
                        After ping test succeeds, run the <strong>manual-install-monokit-client</strong> job template on this host
                      </p>
                    </div>
                    <Tooltip 
                      content={
                        manualInstallTemplateId ? 
                        "Run 'manual-install-monokit-client' job template on this host after ping verification" :
                        "Job template 'manual-install-monokit-client' not found in AWX"
                      }
                    >
                      <div>
                        <Switch
                          id="run-manual-install"
                          aria-label="Run manual install after ping"
                          isChecked={manualInstallMonokitClient}
                          onChange={() => setManualInstallMonokitClient(!manualInstallMonokitClient)}
                          isDisabled={!manualInstallTemplateId || loading || processingStep === 'success'}
                        />
                      </div>
                    </Tooltip>
                  </div>
                  {!manualInstallTemplateId && (
                    <div style={{ marginTop: '8px', color: theme === 'dark' ? '#ff6b6b' : '#c9190b', fontSize: '14px' }}>
                      Warning: The "manual-install-monokit-client" job template was not found in AWX
                    </div>
                  )}
                </div>
                
                {/* Action buttons */}
                <ActionGroup style={{ marginTop: '24px' }}>
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
                    onClick={() => navigate('/hosts')}
                    isDisabled={loading && processingStep !== 'success'}
                  >
                    Cancel
                  </Button>
                </ActionGroup>
              </Form>
            </CardBody>
          </Card>
        </StackItem>
      </Stack>
    </PageSection>
  );
};

export default AwxHostAddPage;