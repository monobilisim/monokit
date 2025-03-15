import React, { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { useNavigate } from 'react-router-dom';
import { useTheme } from '../ThemeContext.jsx';
import {
  PageSection,
  Title,
  Button,
  Card,
  CardBody,
  CardHeader,
  Spinner,
  Breadcrumb,
  BreadcrumbItem,
  Alert,
  AlertVariant,
  Stack,
  StackItem,
  Split,
  SplitItem,
  AlertGroup,
  AlertActionCloseButton,
  FormGroup,
  Grid,
  GridItem,
  TextArea,
} from '@patternfly/react-core';
import { ArrowLeftIcon, TrashIcon, SaveIcon, PlusCircleIcon } from '@patternfly/react-icons';
import { getConfig, updateConfig, deleteConfig } from '../utils/api';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';
import DeleteConfirmationModal from '../components/DeleteConfirmationModal';

// Import Monaco Editor with loading indicator
import Editor from '@monaco-editor/react';

const MonokitConfig = ({ onAuthError }) => {
  const { hostname } = useParams();
  const navigate = useNavigate();
  const { theme } = useTheme();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [configFiles, setConfigFiles] = useState({});
  const [selectedFile, setSelectedFile] = useState('');
  const [fileContent, setFileContent] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [newFileName, setNewFileName] = useState('');
  const [newFileContent, setNewFileContent] = useState('');
  const [isAddingFile, setIsAddingFile] = useState(false);
  const [alerts, setAlerts] = useState([]);
  
  // Add state for delete confirmation modal
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [fileToDelete, setFileToDelete] = useState('');
  const [isDeleting, setIsDeleting] = useState(false);

  const [isEditorReady, setIsEditorReady] = useState(false);
  const [monacoError, setMonacoError] = useState(null);

  // Add refs for editor instances
  const mainEditorRef = useRef(null);
  const newFileEditorRef = useRef(null);

  // Function to determine language - simplified to only detect YAML
  const detectLanguage = (fileName) => {
    if (!fileName) return 'plaintext';
    
    const extension = fileName.split('.').pop().toLowerCase();
    
    // Only detect YAML files, use plaintext for everything else
    if (extension === 'yml' || extension === 'yaml') {
      return 'yaml';
    }
    
    return 'plaintext';
  };

  useEffect(() => {
    fetchConfigFiles();
  }, [hostname]);

  const fetchConfigFiles = async () => {
    try {
      setIsLoading(true);
      const response = await getConfig(hostname);
      setConfigFiles(response.data);
      
      // Select the first file by default if any exists
      const fileNames = Object.keys(response.data);
      if (fileNames.length > 0) {
        setSelectedFile(fileNames[0]);
        setFileContent(response.data[fileNames[0]]);
      }
      
      setError('');
    } catch (err) {
      console.error('Failed to fetch configuration files:', err);
      setError('Failed to load configuration files. Please try again.');
      
      if (err.response && err.response.status === 401) {
        onAuthError(err);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleSaveConfig = async () => {
    if (!selectedFile) return;
    
    try {
      setIsSaving(true);
      const configData = { ...configFiles };
      configData[selectedFile] = fileContent;
      
      await updateConfig(hostname, configData);
      
      // Show success alert
      addAlert('Configuration saved successfully', 'success');
      
      // Refresh config files
      fetchConfigFiles();
    } catch (err) {
      console.error('Failed to save configuration:', err);
      addAlert('Failed to save configuration', 'danger');
      
      if (err.response && err.response.status === 401) {
        onAuthError(err);
      }
    } finally {
      setIsSaving(false);
    }
  };

  const handleAddNewFile = async () => {
    if (!newFileName.trim()) {
      addAlert('File name cannot be empty', 'warning');
      return;
    }
    
    try {
      setIsSaving(true);
      const configData = { ...configFiles };
      configData[newFileName] = newFileContent;
      
      await updateConfig(hostname, configData);
      
      // Reset form
      setNewFileName('');
      setNewFileContent('');
      setIsAddingFile(false);
      
      // Show success alert
      addAlert(`Added new configuration file: ${newFileName}`, 'success');
      
      // Refresh config files and select the new file
      await fetchConfigFiles();
      setSelectedFile(newFileName);
      setFileContent(newFileContent);
    } catch (err) {
      console.error('Failed to add new configuration file:', err);
      addAlert('Failed to add new configuration file', 'danger');
      
      if (err.response && err.response.status === 401) {
        onAuthError(err);
      }
    } finally {
      setIsSaving(false);
    }
  };

  // Open the delete confirmation modal
  const openDeleteModal = (fileName) => {
    setFileToDelete(fileName);
    setIsDeleteModalOpen(true);
  };

  // Handle file deletion
  const handleDeleteFile = async () => {
    if (!fileToDelete) return;
    
    try {
      setIsDeleting(true);
      
      // Call the deleteConfig API function
      await deleteConfig(hostname, fileToDelete);
      
      // Remove file from local state
      const updatedFiles = { ...configFiles };
      delete updatedFiles[fileToDelete];
      setConfigFiles(updatedFiles);
      
      // Select another file if available, otherwise reset selection
      const fileNames = Object.keys(updatedFiles);
      if (fileNames.length > 0) {
        setSelectedFile(fileNames[0]);
        setFileContent(updatedFiles[fileNames[0]]);
      } else {
        setSelectedFile('');
        setFileContent('');
      }
      
      // Show success alert
      addAlert(`Deleted configuration file: ${fileToDelete}`, 'success');
      
      // Close the modal
      setIsDeleteModalOpen(false);
      setFileToDelete('');
    } catch (err) {
      console.error(`Failed to delete configuration file: ${fileToDelete}`, err);
      addAlert(`Failed to delete configuration file: ${fileToDelete}`, 'danger');
      
      if (err.response && err.response.status === 401) {
        onAuthError(err);
      }
    } finally {
      setIsDeleting(false);
    }
  };

  const addAlert = (title, variant, description = '') => {
    const key = new Date().getTime();
    setAlerts([...alerts, { title, variant, key, description }]);

    // Automatically remove the alert after 5 seconds
    setTimeout(() => {
      setAlerts(currentAlerts => currentAlerts.filter(alert => alert.key !== key));
    }, 5000);
  };

  // Handle Monaco Editor change for selected file
  const handleEditorChange = (value) => {
    setFileContent(value || '');
  };

  // Handle Monaco Editor change for new file
  const handleNewFileEditorChange = (value) => {
    setNewFileContent(value || '');
  };

  // Handle Monaco Editor ready event - store the editor instance
  const handleEditorDidMount = (editor, monaco) => {
    setIsEditorReady(true);
    console.log("Monaco editor is ready", editor, monaco);
    mainEditorRef.current = editor;
  };

  // Handle new file editor mount
  const handleNewEditorDidMount = (editor, monaco) => {
    console.log("New file editor is ready", editor, monaco);
    newFileEditorRef.current = editor;
  };

  // Handle Monaco Editor loading error
  const handleEditorWillMount = monaco => {
    console.log("Monaco is being loaded", monaco);
  };

  // Handle Monaco Editor error
  const handleEditorError = error => {
    console.error("Monaco editor loading error:", error);
    setMonacoError(error);
  };

  if (isLoading) {
    return (
      <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
        <div style={{ display: 'flex', justifyContent: 'center', padding: '40px' }}>
          <Spinner size="xl" />
        </div>
      </PageSection>
    );
  }

  return (
    <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
      <AlertGroup isToast>
        {alerts.map(({ key, variant, title, description }) => (
          <Alert
            key={key}
            variant={AlertVariant[variant]}
            title={title}
            actionClose={
              <AlertActionCloseButton
                title={title}
                onClose={() => setAlerts(alerts.filter(alert => alert.key !== key))}
              />
            }
          >
            {description}
          </Alert>
        ))}
      </AlertGroup>

      <Stack hasGutter>
        <StackItem>
          <Breadcrumb>
            <BreadcrumbItem onClick={() => navigate('/hosts')}>Hosts</BreadcrumbItem>
            <BreadcrumbItem onClick={() => navigate(`/hosts/${hostname}`)}>
              {hostname}
            </BreadcrumbItem>
            <BreadcrumbItem isActive>Configuration</BreadcrumbItem>
          </Breadcrumb>
        </StackItem>

        <StackItem>
          <Split>
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
                {hostname} - Configuration Files
              </Title>
            </SplitItem>
          </Split>
        </StackItem>

        {error && (
          <StackItem>
            <Alert variant="danger" title={error} />
          </StackItem>
        )}

        {!isAddingFile ? (
          <StackItem>
            <Button
              variant="primary"
              onClick={() => setIsAddingFile(true)}
              style={{ marginBottom: '16px' }}
            >
              <PlusCircleIcon style={{ marginRight: '8px' }} />
              Add New Configuration File
            </Button>
          </StackItem>
        ) : (
          <StackItem>
            <Card>
              <CardHeader>
                <Title headingLevel="h2" size="lg">
                  Add New Configuration File
                </Title>
              </CardHeader>
              <CardBody>
                <FormGroup label="File Name" isRequired>
                  <input
                    type="text"
                    value={newFileName}
                    onChange={(e) => setNewFileName(e.target.value)}
                    className="pf-c-form-control"
                    style={{ marginBottom: '16px', width: '100%' }}
                  />
                </FormGroup>
                <FormGroup label="File Content" isRequired>
                  {/* Monaco Editor with fallback to TextArea in case of loading issues */}
                  {monacoError ? (
                    <TextArea
                      id="new-file-content"
                      aria-label="New file content"
                      value={newFileContent}
                      onChange={(e) => setNewFileContent(e.target.value)}
                      style={{ height: '300px', fontFamily: 'monospace', width: '100%' }}
                      className="pf-c-form-control"
                    />
                  ) : (
                    <div style={{ border: '1px solid #ccc', borderRadius: '3px', height: '300px' }}>
                      <Editor
                        height="300px"
                        language={detectLanguage(newFileName)}
                        value={newFileContent}
                        onChange={handleNewFileEditorChange}
                        theme={theme === 'dark' ? 'vs-dark' : 'light'}
                        onMount={handleNewEditorDidMount}
                        beforeMount={handleEditorWillMount}
                        loading={<Spinner size="lg" />}
                        options={{
                          minimap: { enabled: false },
                          scrollBeyondLastLine: false,
                          automaticLayout: true, 
                          wordWrap: 'on',
                          tabSize: 2,
                        }}
                        onError={handleEditorError}
                      />
                    </div>
                  )}
                </FormGroup>
                <Split hasGutter style={{ marginTop: '16px' }}>
                  <SplitItem>
                    <Button
                      variant="primary"
                      onClick={handleAddNewFile}
                      isLoading={isSaving}
                      isDisabled={isSaving || !newFileName.trim()}
                    >
                      Add File
                    </Button>
                  </SplitItem>
                  <SplitItem>
                    <Button
                      variant="link"
                      onClick={() => setIsAddingFile(false)}
                      isDisabled={isSaving}
                    >
                      Cancel
                    </Button>
                  </SplitItem>
                </Split>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {!isAddingFile && Object.keys(configFiles).length > 0 && (
          <StackItem>
            <Grid hasGutter>
              <GridItem span={3}>
                <Card>
                  <CardHeader>
                    <Title headingLevel="h2" size="md">
                      Configuration Files
                    </Title>
                  </CardHeader>
                  <CardBody>
                    <div style={{ maxHeight: '500px', overflow: 'auto' }}>
                      {Object.keys(configFiles).map((fileName) => (
                        <div
                          key={fileName}
                          style={{
                            padding: '8px 16px',
                            cursor: 'pointer',
                            backgroundColor: selectedFile === fileName
                              ? (theme === 'dark' ? '#3c3836' : '#f0f0f0')
                              : 'transparent',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            borderBottom: '1px solid #ddd',
                            borderRadius: '4px',
                            margin: '4px 0'
                          }}
                        >
                          <span
                            onClick={() => {
                              setSelectedFile(fileName);
                              setFileContent(configFiles[fileName]);
                            }}
                            style={{ flexGrow: 1 }}
                          >
                            {fileName}
                          </span>
                          <Button
                            variant="plain"
                            onClick={() => openDeleteModal(fileName)}
                            aria-label={`Delete ${fileName}`}
                            style={{ color: theme === 'dark' ? '#fb4934' : '#9d0006' }}
                          >
                            <TrashIcon />
                          </Button>
                        </div>
                      ))}
                    </div>
                  </CardBody>
                </Card>
              </GridItem>
              
              <GridItem span={9}>
                {selectedFile && (
                  <Card>
                    <CardHeader>
                      <Split>
                        <SplitItem isFilled>
                          <Title headingLevel="h2" size="md">
                            {selectedFile}
                          </Title>
                        </SplitItem>
                        <SplitItem>
                          <Button
                            variant="primary"
                            onClick={handleSaveConfig}
                            isLoading={isSaving}
                            isDisabled={isSaving}
                          >
                            <SaveIcon style={{ marginRight: '8px' }} />
                            Save
                          </Button>
                        </SplitItem>
                      </Split>
                    </CardHeader>
                    <CardBody>
                      {/* Monaco Editor with fallback to TextArea in case of loading issues */}
                      {monacoError ? (
                        <TextArea
                          id="file-content-editor"
                          aria-label="File content"
                          value={fileContent}
                          onChange={(e) => setFileContent(e.target.value)}
                          style={{ height: '500px', fontFamily: 'monospace', width: '100%' }}
                          className="pf-c-form-control"
                        />
                      ) : (
                        <div style={{ border: '1px solid #ccc', borderRadius: '3px', height: '500px' }}>
                          <Editor
                            height="500px"
                            width="100%"
                            language={detectLanguage(selectedFile)}
                            value={fileContent}
                            onChange={handleEditorChange}
                            theme={theme === 'dark' ? 'vs-dark' : 'light'}
                            onMount={handleEditorDidMount}
                            beforeMount={handleEditorWillMount}
                            loading={<Spinner size="lg" />}
                            options={{
                              minimap: { enabled: true },
                              scrollBeyondLastLine: false,
                              automaticLayout: true,
                              wordWrap: 'on',
                              lineNumbers: 'on',
                              renderLineHighlight: 'all',
                              tabSize: 2,
                            }}
                            onError={handleEditorError}
                          />
                        </div>
                      )}
                    </CardBody>
                  </Card>
                )}
              </GridItem>
            </Grid>
          </StackItem>
        )}

        {!isAddingFile && Object.keys(configFiles).length === 0 && (
          <StackItem>
            <Card>
              <CardBody>
                <div style={{ textAlign: 'center', padding: '40px 0' }}>
                  <Title headingLevel="h2" size="xl">
                    No Configuration Files
                  </Title>
                  <p style={{ marginTop: '16px', marginBottom: '24px' }}>
                    This host does not have any configuration files yet.
                  </p>
                  <Button
                    variant="primary"
                    onClick={() => setIsAddingFile(true)}
                  >
                    <PlusCircleIcon style={{ marginRight: '8px' }} />
                    Add New Configuration File
                  </Button>
                </div>
              </CardBody>
            </Card>
          </StackItem>
        )}
      </Stack>

      {/* Delete confirmation modal */}
      <DeleteConfirmationModal
        isOpen={isDeleteModalOpen}
        onClose={() => setIsDeleteModalOpen(false)}
        onDelete={handleDeleteFile}
        title={`Delete Configuration File`}
        message={`Are you sure you want to delete the configuration file "${fileToDelete}"? This action cannot be undone.`}
        isDeleting={isDeleting}
      />
    </PageSection>
  );
};

export default MonokitConfig;
