import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  Form,
  FormGroup,
  Spinner,
  Alert,
  TextArea,
  ActionGroup,
  Tooltip,
  Split,
  SplitItem,
  Popover,
  Card,
  CardBody,
  Divider
} from '@patternfly/react-core';
import { PlayIcon, InfoCircleIcon, HelpIcon } from '@patternfly/react-icons';
import { executeAwxJob, getAwxJobTemplates, getAwxJobTemplateDetails } from '../utils/api';
import CenteredIcon from './CenteredIcon';
// Import Monaco Editor for YAML editing
import Editor from '@monaco-editor/react';
// Import js-yaml for YAML parsing
import jsyaml from 'js-yaml';

// Define custom styles for dark mode compatibility
const darkModeStyles = `
  @media (prefers-color-scheme: dark) {
    select, option {
      color: var(--pf-global--Color--light-100, white) !important;
    }
  }
`;

/**
 * Modal component for executing AWX jobs
 * 
 * @param {Object} props Component properties
 * @param {boolean} props.isOpen Whether the modal is open
 * @param {Function} props.onClose Function to call when modal is closed
 * @param {string} props.hostname Host name to execute job on
 * @param {Function} props.onJobLaunched Function to call when job is launched with job data
 */
const AwxJobExecute = ({ isOpen, onClose, hostname, onJobLaunched = () => {} }) => {
  const [loading, setLoading] = useState(false);
  const [templateLoading, setTemplateLoading] = useState(false);
  const [detailsLoading, setDetailsLoading] = useState(false);
  const [templates, setTemplates] = useState([]);
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [extraVars, setExtraVars] = useState('');
  const [templateVars, setTemplateVars] = useState(null);
  const [templateDetails, setTemplateDetails] = useState(null);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  
  // Fetch templates from AWX API when the modal opens
  useEffect(() => {
    if (isOpen && hostname) {
      fetchTemplates();
      // Reset form when reopening
      setSelectedTemplate('');
      setExtraVars('');
      setTemplateVars(null);
      setTemplateDetails(null);
      setError(null);
      setSuccess(null);
      console.log("Form reset, extraVars set to empty string");
    }
  }, [isOpen, hostname]);
  
  // Fetch template details when a template is selected
  useEffect(() => {
    if (selectedTemplate) {
      fetchTemplateDetails(selectedTemplate);
    } else {
      setTemplateVars(null);
      setTemplateDetails(null);
    }
  }, [selectedTemplate]);
  
  const fetchTemplates = async () => {
    setTemplateLoading(true);
    setError(null);
    
    try {
      const response = await getAwxJobTemplates(hostname);
      setTemplates(response.data || []);
    } catch (err) {
      console.error('Error fetching job templates:', err);
      setError(err.response?.data?.error || err.message || 'Failed to fetch job templates');
      setTemplates([]);
    } finally {
      setTemplateLoading(false);
    }
  };
  
  const fetchTemplateDetails = async (templateId) => {
    if (!templateId) {
      console.log("No template ID provided");
      return;
    }
    
    console.log("Fetching details for template ID:", templateId);
    setDetailsLoading(true);
    
    try {
      // Ensure we have a valid template ID
      const id = String(templateId).trim();
      if (!id) {
        throw new Error("Invalid template ID");
      }
      
      const response = await getAwxJobTemplateDetails(hostname, id);
      console.log("Template details response:", response.data);
      setTemplateDetails(response.data);
      
      // Extract and format variables
      const vars = response.data.variables || {};
      setTemplateVars(vars);
      
      // If no custom variables set yet, pre-populate with template defaults
      if ((!extraVars || !extraVars.trim()) && vars.extra_vars) {
        try {
          let formattedVars = '';
          if (typeof vars.extra_vars === 'string') {
            // Try to parse as JSON first (to handle legacy format)
            try {
              const parsed = JSON.parse(vars.extra_vars);
              formattedVars = jsyaml.dump(parsed);
            } catch (e) {
              // If it's not valid JSON, it might already be YAML
              formattedVars = vars.extra_vars;
            }
          } else if (typeof vars.extra_vars === 'object' && vars.extra_vars !== null) {
            // Convert object directly to YAML
            formattedVars = jsyaml.dump(vars.extra_vars);
          } else {
            // Fallback to string representation
            formattedVars = String(vars.extra_vars || '');
          }
          
          // Check if the result is valid before setting it
          if (formattedVars === '[object Object]') {
            console.error("Invalid formattedVars: object toString() resulted in [object Object]");
            formattedVars = '# Default variables could not be parsed\n';
          }
          
          console.log("Setting extraVars to:", formattedVars);
          setExtraVars(formattedVars);
        } catch (err) {
          console.error("Error formatting template variables:", err);
          setExtraVars('# Error loading default variables\n');
        }
      }
    } catch (err) {
      console.error('Error fetching template details:', err);
      // Don't set global error, just clear template vars
      setTemplateVars(null);
      setTemplateDetails(null);
    } finally {
      setDetailsLoading(false);
    }
  };
  
  const handleTemplateChange = (event) => {
    const templateId = event.target.value;
    console.log("Template selected:", templateId);
    setSelectedTemplate(templateId);
    
    // Immediately fetch details if a valid template is selected
    if (templateId) {
      fetchTemplateDetails(templateId);
    }
  };
  
  // The PatternFly TextArea component passes the event as the second parameter
  // and the value as the first parameter in the onChange handler
  const handleExtraVarsChange = (value, event) => {
    console.log("TextArea onChange called with value:", value);
    setExtraVars(value);
  };
  
  const handleSubmit = async (event) => {
    if (event) {
      event.preventDefault();
    }
    
    // Validate template selection
    if (!selectedTemplate) {
      setError("Please select a job template first");
      return;
    }
    
    console.log("Executing job with template ID:", selectedTemplate);
    setLoading(true);
    setError(null);
    setSuccess(null);
    
    try {
      // Parse extra_vars if provided
      let parsedExtraVars = {};
      if (extraVars.trim()) {
        try {
          // Parse the YAML to an object
          parsedExtraVars = jsyaml.load(extraVars);
          console.log("Parsed YAML:", parsedExtraVars);
          
          // Debug check to ensure it's not toString-ing to [object Object]
          if (typeof parsedExtraVars === 'object') {
            console.log("Object representation:", JSON.stringify(parsedExtraVars));
          }
        } catch (parseError) {
          setError(`Invalid YAML in extra_vars: ${parseError.message}`);
          setLoading(false);
          return;
        }
      }
      
      // Make sure we have a valid template ID as a number
      const templateId = parseInt(selectedTemplate, 10);
      if (isNaN(templateId)) {
        throw new Error("Invalid template ID");
      }
      
      console.log("Calling executeAwxJob with:", hostname, templateId);
      
      // Execute the job
      const response = await executeAwxJob(
        hostname, 
        templateId, 
        parsedExtraVars
      );
      
      console.log("Job execution response:", response.data);
      
      // Get job ID from response (could be in different formats)
      let jobId = response.data.job_id;
      if (!jobId && response.data.id) {
        jobId = response.data.id;
      }
      
      setSuccess(`Job launched successfully! Job ID: ${jobId}`);
      
      // Call the callback with the job details
      onJobLaunched(response.data);
      
      // Optionally close the modal on success
      // setTimeout(() => onClose(), 2000);
      
    } catch (err) {
      console.error('Error executing job:', err);
      setError(err.response?.data?.error || err.message || 'Failed to launch job');
    } finally {
      setLoading(false);
    }
  };
  
  const isFormValid = () => {
    return selectedTemplate !== '';
  };
  
  // Format template variables for display
  const formatTemplateVars = () => {
    if (!templateVars || !templateVars.extra_vars) {
      return 'No default variables defined for this template.';
    }
    
    if (typeof templateVars.extra_vars === 'string') {
      // Try to parse as JSON first to handle legacy data
      try {
        const parsed = JSON.parse(templateVars.extra_vars);
        return jsyaml.dump(parsed);
      } catch (e) {
        // If it's not valid JSON, return as is (might already be YAML)
        return templateVars.extra_vars;
      }
    } else {
      try {
        return jsyaml.dump(templateVars.extra_vars);
      } catch (error) {
        console.error("Error converting to YAML:", error);
        return JSON.stringify(templateVars.extra_vars, null, 2);
      }
    }
  };
  
  return (
    <Modal
      variant={ModalVariant.large}
      title={
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <CenteredIcon 
            icon={<PlayIcon style={{ color: '#0066CC' }} />} 
            style={{ marginRight: '10px' }} 
          />
          Execute AWX Job on {hostname}
        </div>
      }
      isOpen={isOpen}
      onClose={onClose}
      appendTo={document.body}
      hasNoBodyWrapper
      showClose={true}
      style={{ width: '850px', maxWidth: '95vw' }}
    >
      <style>{darkModeStyles}</style>
      <div style={{ padding: '24px' }}>
        {error && (
          <Alert 
            variant="danger" 
            title="Error" 
            isInline 
            style={{ marginBottom: '24px' }}
          >
            {error}
          </Alert>
        )}
        
        {success && (
          <Alert 
            variant="success" 
            title="Success" 
            isInline 
            style={{ marginBottom: '24px' }}
          >
            {success}
          </Alert>
        )}
        
        <Form onSubmit={handleSubmit}>
          <FormGroup 
            label={
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <span>Job Template</span>
                <Popover
                  headerContent={<div>About Job Templates</div>}
                  bodyContent={
                    <div>
                      <p>Select from available AWX job templates to execute on this host.</p>
                      <p>Templates define the playbook to run and default variables.</p>
                    </div>
                  }
                >
                  <Button variant="plain" aria-label="Help" style={{ padding: '0 8px' }}>
                    <HelpIcon />
                  </Button>
                </Popover>
              </div>
            }
            isRequired 
            fieldId="job-template"
            style={{ marginBottom: '20px' }}
          >
            {templateLoading ? (
              <div style={{ padding: '10px 0' }}>
                <Spinner size="md" /> <span style={{ marginLeft: '10px' }}>Loading templates...</span>
              </div>
            ) : (
              <div className="template-selector-wrapper">
                <select 
                  value={selectedTemplate}
                  onChange={handleTemplateChange}
                  id="job-template"
                  name="job-template"
                  disabled={loading || templateLoading}
                  style={{
                    padding: '8px 12px',
                    width: '100%',
                    border: '1px solid #d2d2d2',
                    borderRadius: '3px',
                    backgroundColor: 'transparent',
                    fontSize: '14px',
                    height: '36px',
                    color: 'inherit',
                    cursor: loading || templateLoading ? 'not-allowed' : 'pointer',
                    boxShadow: '0 1px 1px rgba(0,0,0,0.05)'
                  }}
                >
                  <option value="" style={{color: 'inherit'}}>Select a template</option>
                  {templates.map((template) => (
                    <option key={template.id} value={template.id.toString()} style={{color: 'inherit'}}>
                      {template.name}
                    </option>
                  ))}
                </select>
              </div>
            )}
            {templates.length === 0 && !templateLoading && (
              <div style={{ color: '#c9190b', marginTop: '8px', fontSize: '14px' }}>
                No job templates available. Please check AWX configuration.
              </div>
            )}
            
            {selectedTemplate && templateDetails && (
              <div style={{ marginTop: '10px' }}>
                {templateDetails.description && (
                  <p style={{ color: '#666', fontSize: '14px', margin: '4px 0' }}>
                    <strong>Description:</strong> {templateDetails.description}
                  </p>
                )}
                <p style={{ color: '#666', fontSize: '14px', margin: '4px 0' }}>
                  <strong>Job Type:</strong> {templateDetails.job_type || 'Standard'} 
                  {templateDetails.has_survey && ' • Includes Survey'}
                </p>
              </div>
            )}
          </FormGroup>
          
          <FormGroup 
            label={
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <span>Extra Variables (YAML format)</span>
                {detailsLoading && (
                  <Spinner size="sm" style={{ marginLeft: '8px' }} />
                )}
              </div>
            }
            fieldId="extra-vars"
            style={{ marginBottom: '20px' }}
          >
            <div style={{ borderRadius: '3px', overflow: 'hidden', border: '1px solid var(--pf-global--BorderColor--100, #d2d2d2)' }}>
              {/* Use Monaco Editor for better YAML editing experience */}
              <Editor
                height="250px"
                defaultLanguage="yaml"
                language="yaml"
                value={extraVars || ''}
                onChange={value => setExtraVars(value)}
                options={{
                  minimap: { enabled: false },
                  scrollBeyondLastLine: false,
                  fontSize: 14,
                  readOnly: loading,
                  wordWrap: 'on',
                  lineNumbers: 'on',
                  folding: true,
                  renderLineHighlight: 'all',
                  automaticLayout: true,
                  tabSize: 2,
                  formatOnPaste: true,
                  formatOnType: true
                }}
                theme="vs-dark"
                beforeMount={(monaco) => {
                  // Configure Monaco editor for better integration
                  monaco.editor.defineTheme('patternflyDark', {
                    base: 'vs-dark',
                    inherit: true,
                    rules: [],
                    colors: {
                      'editor.background': '#1b1d21',
                      'editor.foreground': '#f0f0f0',
                      'editorCursor.foreground': '#f0f0f0',
                      'editor.lineHighlightBackground': '#2b2e33',
                      'editorLineNumber.foreground': '#c5c5c5',
                      'editor.selectionBackground': '#3e4451',
                      'editor.inactiveSelectionBackground': '#3a3d41'
                    }
                  });
                  monaco.editor.setTheme('patternflyDark');
                }}
                loading={<div style={{ padding: '10px' }}><Spinner size="md" /> Loading editor...</div>}
              />
            </div>
            
            <Split hasGutter style={{ marginTop: '10px' }}>
              <SplitItem>
                <div style={{ fontSize: '14px', color: '#666' }}>
                <InfoCircleIcon style={{ color: '#0066CC', marginRight: '5px' }} />
                  Provide variables in YAML format that will be passed to the job template.
                </div>
              </SplitItem>
              <SplitItem isFilled />
              {templateVars && templateVars.extra_vars && (
                <SplitItem>
                  <Popover
                    headerContent={<div>Default Template Variables</div>}
                    bodyContent={
                      <div style={{ maxWidth: '500px', maxHeight: '400px', overflow: 'auto' }}>
                        <Card isCompact>
                          <CardBody>
                            <pre style={{ 
                              whiteSpace: 'pre-wrap', 
                              fontSize: '13px',
                              fontFamily: 'monospace',
                              margin: 0
                            }}>
                              {formatTemplateVars()}
                            </pre>
                          </CardBody>
                        </Card>
                        <Divider style={{ margin: '10px 0' }} />
                        <p style={{ fontSize: '13px', margin: '5px 0' }}>
                          These are the default variables defined in the template.
                          You can override them in the extra variables field.
                        </p>
                      </div>
                    }
                  >
                    <Button 
                      variant="link" 
                      style={{ padding: '0', fontSize: '14px', display: 'inline-flex', alignItems: 'center' }}
                    >
                      View default variables <InfoCircleIcon style={{ marginLeft: '5px' }} />
                    </Button>
                  </Popover>
                </SplitItem>
              )}
            </Split>
          </FormGroup>
          
          <ActionGroup style={{ marginTop: '32px', display: 'flex', justifyContent: 'flex-end', gap: '16px' }}>
            <Button 
              variant="primary" 
              onClick={handleSubmit}
              isDisabled={!isFormValid() || loading}
              isLoading={loading}
              style={{ padding: '8px 16px', minWidth: '120px' }}
              type="submit"
            >
              {loading ? 'Executing...' : 'Execute Job'}
            </Button>
            <Button 
              variant="link" 
              onClick={onClose}
              isDisabled={loading}
              style={{ padding: '8px 16px' }}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </div>
    </Modal>
  );
};

export default AwxJobExecute;