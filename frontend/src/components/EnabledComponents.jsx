import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  Spinner,
  Alert,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Split,
  SplitItem,
  Label,
  Switch,
  Bullseye,
  EmptyState,
  EmptyStateBody,
  Title
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import axios from 'axios';

const EnabledComponents = ({ isOpen, onClose, hostname, hostData }) => {
  const [components, setComponents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [updateStatus, setUpdateStatus] = useState({ isUpdating: false, component: null, action: null });

  useEffect(() => {
    console.log("EnabledComponents mounted");
    console.log("isOpen:", isOpen);
    console.log("hostname:", hostname);
    console.log("hostData:", hostData);
    
    if (isOpen) {
      console.log("Modal should be open, processing components");
      processComponents();
    }
  }, [isOpen, hostname, hostData]);

  // Process components from host data
  const processComponents = () => {
    try {
      setLoading(true);
      setError('');
      
      if (hostData?.installedComponents) {
        const installedComponentsList = hostData.installedComponents.split('::');
        const disabledComponentsList = hostData.disabledComponents ? hostData.disabledComponents.split('::') : [];
        
        // Create component status objects with proper naming
        const componentsData = [];
        
        // Add standard components from installed components
        installedComponentsList.forEach(component => {
          // Format component name to ensure it ends with Health for both display and API
          const formattedName = component.endsWith('Health') ? component : `${component}Health`;
          const isDisabled = disabledComponentsList.includes(component) || 
                             disabledComponentsList.includes(formattedName);
          
          componentsData.push({
            name: formattedName, // Use the Health suffix for API calls
            displayName: formattedName, // Same name for display
            enabled: !isDisabled,
            originalName: component // Keep original name for reference
          });
        });
        
        // Check if osHealth exists, if not add it
        const hasOs = installedComponentsList.some(comp => 
          comp === 'os' || comp === 'osHealth'
        );
        
        if (!hasOs) {
          componentsData.push({
            name: 'osHealth',
            displayName: 'osHealth',
            enabled: !disabledComponentsList.includes('os') && !disabledComponentsList.includes('osHealth'),
            originalName: 'os'
          });
        }
        
        // Sort components alphabetically
        componentsData.sort((a, b) => a.displayName.localeCompare(b.displayName));
        
        console.log("Processed components:", componentsData);
        setComponents(componentsData);
      } else {
        setComponents([]);
      }
    } catch (err) {
      console.error('Failed to process components:', err);
      setError('Failed to process component information');
    } finally {
      setLoading(false);
    }
  };

  // Toggle component status
  const toggleComponent = async (component, newEnabledState) => {
    try {
      setUpdateStatus({ 
        isUpdating: true, 
        component: component, 
        action: newEnabledState ? 'enabling' : 'disabling' 
      });
      
      const endpoint = newEnabledState 
        ? `/api/v1/hosts/${hostname}/enable/${component}`
        : `/api/v1/hosts/${hostname}/disable/${component}`;
      
      await axios.post(endpoint, {}, {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      
      // Update local state
      setComponents(prevComponents => 
        prevComponents.map(c => 
          c.name === component ? { ...c, enabled: newEnabledState } : c
        )
      );
      
      setError('');
    } catch (err) {
      console.error(`Failed to ${newEnabledState ? 'enable' : 'disable'} component:`, err);
      setError(`Failed to ${newEnabledState ? 'enable' : 'disable'} ${component}`);
    } finally {
      setUpdateStatus({ isUpdating: false, component: null, action: null });
    }
  };

  return (
    <Modal
      variant={ModalVariant.large}
      title={`Manage Components - ${hostname}`}
      isOpen={isOpen}
      onClose={onClose}
      actions={[
        <Button key="close" variant="primary" onClick={onClose}>
          Close
        </Button>
      ]}
      style={{ minHeight: '600px' }}
      width={'60%'}
    >
      {loading ? (
        <Bullseye>
          <Spinner size="xl" />
        </Bullseye>
      ) : error ? (
        <Alert variant="danger" title={error} />
      ) : components.length === 0 ? (
        <EmptyState>
          <CubesIcon size="lg" />
          <Title headingLevel="h4" size="lg">No components found</Title>
          <EmptyStateBody>
            There are no components installed on this host.
          </EmptyStateBody>
        </EmptyState>
      ) : (
        <div style={{ maxHeight: '500px', overflowY: 'auto' }}>
          <DescriptionList isCompact={false}>
            {components.map(component => (
              <DescriptionListGroup key={component.name} style={{ marginBottom: '20px', padding: '10px', borderBottom: '1px solid #ddd' }}>
                <DescriptionListTerm style={{ fontSize: '16px', fontWeight: 'bold' }}>
                  {component.displayName}
                </DescriptionListTerm>
                <DescriptionListDescription>
                  <Split hasGutter style={{ marginTop: '10px' }}>
                    <SplitItem>
                      <Switch
                        id={`switch-${component.name}`}
                        label="Enabled"
                        labelOff="Disabled"
                        isChecked={component.enabled}
                        onChange={() => toggleComponent(component.name, !component.enabled)}
                        isDisabled={updateStatus.isUpdating && updateStatus.component === component.name}
                      />
                    </SplitItem>
                    <SplitItem>
                      {updateStatus.isUpdating && updateStatus.component === component.name ? (
                        <Label color="blue">
                          {updateStatus.action === 'enabling' ? 'Enabling...' : 'Disabling...'}
                        </Label>
                      ) : (
                        <Label color={component.enabled ? 'green' : 'red'} style={{ fontSize: '14px' }}>
                          {component.enabled ? 'Active' : 'Inactive'}
                        </Label>
                      )}
                    </SplitItem>
                  </Split>
                </DescriptionListDescription>
              </DescriptionListGroup>
            ))}
          </DescriptionList>
        </div>
      )}
    </Modal>
  );
};

export default EnabledComponents;
