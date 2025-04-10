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

/**
 * Modal component for adding a host to AWX and validating it
 */
const AwxHostAdd = ({ isOpen, onClose, onHostAdded = () => {} }) => {
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

  // Reset form when opening modal
  useEffect(() => {
    if (isOpen) {
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
      
      // Find the ping template
      findPingTemplate();
    }
  }, [isOpen]);

  // Find the required job templates
  const findPingTemplate = async () => {
    try {
      console.log("Finding required templates...");
      
      // First get regular job templates
      console.log("Fetching job templates...");
      const jobResponse = await getAwxTemplatesGlobal();
      
      if (!jobResponse.data) {
        console.log("No data in job templates response");
        setErrorMessage("Error fetching job templates from AWX");
        return;
      }
      
      // Then get workflow templates
      console.log("Fetching workflow templates...");
      let workflowTemplates = [];
      try {
        const workflowResponse = await getAwxWorkflowTemplatesGlobal();
        if (workflowResponse.data && (workflowResponse.data.results || workflowResponse.data)) {
          workflowTemplates = workflowResponse.data.results || workflowResponse.data;
          console.log("Workflow templates:", workflowTemplates);
        }
      } catch (err) {
        console.warn("Error fetching workflow templates (this might be expected):", err);
        // We'll continue even if this fails
      }
      
      // Combine both template types
      const jobTemplates = jobResponse.data.results || jobResponse.data;
      const templates = [...jobTemplates, ...workflowTemplates];
      
      console.log(`Found ${jobTemplates.length} job templates and ${workflowTemplates.length} workflow templates`);
      
      if (!Array.isArray(templates)) {
        console.log("Templates is not an array:", templates);
        setErrorMessage("Invalid response format from AWX");
        return;
      }
      
      // Log all template names to debug
      console.log('All template names:');
      templates.forEach(template => {
        if (template.name) {
          console.log(`- "${template.name}" (ID: ${template.id})`);
        }
      });
      
      // More flexible search with trimming and contains logic
      const findTemplateByNameContains = (searchStr) => {
        searchStr = searchStr.toLowerCase().trim();
        // First try exact match after lowercase and trim
        let template = templates.find(t => 
          t.name && t.name.toLowerCase().trim() === searchStr
        );
        
        // If not found, try contains
        if (!template) {
          template = templates.find(t => 
            t.name && t.name.toLowerCase().trim().includes(searchStr)
          );
        }
        
        return template;
      };

      // Find ping template
      const pingTemplate = findTemplateByNameContains('manual-check-ping');
      
      if (pingTemplate && pingTemplate.id) {
        console.log('Found ping template with ID:', pingTemplate.id);
        console.log('Ping template name:', pingTemplate.name);
        setPingTemplateId(pingTemplate.id);
      } else {
        console.log("Ping template not found");
        setErrorMessage('The "manual-check-ping" job template was not found in AWX');
      }

      // Find setup template - using both exact and fuzzy match
      const setupTemplateName = 'workflow-manual-setup-fresh';
      const setupTemplate = findTemplateByNameContains(setupTemplateName);
      
      if (setupTemplate && setupTemplate.id) {
        console.log('Found setup template with ID:', setupTemplate.id);
        console.log('Setup template name:', setupTemplate.name);
        setSetupTemplateId(setupTemplate.id);
      } else {
        console.log(`Setup template "${setupTemplateName}" not found`);
        // We don't show an error here as this template is optional
      }
    } catch (err) {
      console.error("Error finding ping template:", err);
      
      if (err.response?.status === 502) {
        setErrorMessage("Cannot connect to AWX (502 Bad Gateway)");
      } else if (err.message && typeof err.message === 'string') {
        setErrorMessage("Error connecting to AWX: " + err.message);
      } else {
        setErrorMessage("Error connecting to AWX server");
      }
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
          ip_address: ipAddress
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
            
            // If setup toggle is enabled, run manual-setup-fresh-install
            if (runSetupAfterPing && setupTemplateId) {
              console.log("Ping successful! Now running workflow-manual-setup-fresh...");
              setProcessingStep('setup');
              setSuccessMessage(`Host "${hostName}" passed ping test, now running setup workflow...`);
              
              try {
                const setupResponse = await axios.post(`/api/v1/hosts/${hostName}/awx-jobs/execute`, {
                  template_id: setupTemplateId,
                  extra_vars: { limit: hostName }
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
            
            // Call the callback with just the AWX information
            onHostAdded({
              awxOnly: true,
              hostName: hostName,
              ipAddress: ipAddress,
              awxHostId: String(createdHost.id)
            });
            
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
      
      setProcessingStep('');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      variant={ModalVariant.medium}
      title={
        <div style={{ display: 'flex', alignItems: 'center', padding: '16px 16px 0 16px' }}>
          <CenteredIcon 
            icon={<PlusCircleIcon style={{ color: '#0066CC' }} />} 
            style={{ marginRight: '10px' }} 
          />
          Add Host to AWX
        </div>
      }
      isOpen={isOpen}
      onClose={onClose}
      appendTo={document.body}
      showClose={true}
      hasNoBodyWrapper={false}
      style={{ borderRadius: '8px', overflow: 'hidden' }}
    >
      <div style={{ padding: '24px 32px' }}>
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
      {/* Add bottom padding */}
      <div style={{ padding: '12px' }}></div>
    </Modal>
  );
};

export default AwxHostAdd;