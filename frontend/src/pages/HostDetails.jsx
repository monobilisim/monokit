import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTheme } from '../ThemeContext.jsx';
import {
  PageSection,
  Title,
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Button,
  Spinner,
  Alert,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Breadcrumb,
  BreadcrumbItem,
  Label,
  Split,
  SplitItem,
  Stack,
  StackItem,
  Icon,
  Tab,
  Tabs, 
  TabTitleText,
  EmptyState
} from '@patternfly/react-core';
import { Table, Thead, Tbody, Tr, Th, Td, TableVariant } from '@patternfly/react-table';
import { ArrowLeftIcon, CheckCircleIcon, ExclamationCircleIcon, InfoCircleIcon, CubesIcon, CubeIcon, SearchIcon } from '@patternfly/react-icons';
import { format } from 'date-fns';
import axios from 'axios';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';
import EnabledComponents from '../components/EnabledComponents';
import AwxJobsTable from '../components/AwxJobsTable';

const HostDetails = () => {
  const { hostname } = useParams();
  const navigate = useNavigate();
  const { theme } = useTheme();
  const [host, setHost] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [isComponentsModalOpen, setIsComponentsModalOpen] = useState(false);
  const [activeTabKey, setActiveTabKey] = useState(0);
  const [awxJobs, setAwxJobs] = useState([]);
  const [awxLoading, setAwxLoading] = useState(true);
  const [awxError, setAwxError] = useState(null);
  // We'll use a different approach that doesn't make API calls

  useEffect(() => {
    fetchHostDetails();
  }, [hostname]);
  
  // Reset tab selection if AWX tab is selected but AWX Host ID is missing
  useEffect(() => {
    if (activeTabKey === 1 && !host?.awxHostId) {
      setActiveTabKey(0);
    }
  }, [activeTabKey, host]);

  // Function to fetch AWX jobs
  const fetchAwxJobs = async () => {
    if (!host || !host.awxHostId) return;
    
    try {
      setAwxLoading(true);
      console.log(`Directly fetching AWX jobs for host: ${hostname}`);
      
      const response = await axios.get(`/api/v1/hosts/${hostname}/awx-jobs`, {
        headers: {
          Authorization: localStorage.getItem('token')
        },
        timeout: 15000 // 15 second timeout
      });
      
      console.log('AWX jobs API response:', response.data);
      
      if (response.data && Array.isArray(response.data)) {
        setAwxJobs(response.data);
      } else {
        setAwxJobs([]);
        console.warn('Unexpected response format from AWX jobs API:', response.data);
      }
      setAwxError(null);
    } catch (err) {
      console.error('Error fetching AWX jobs:', err);
      setAwxError(err.message || 'Failed to fetch AWX jobs');
      setAwxJobs([]);
    } finally {
      setAwxLoading(false);
    }
  };

  // Fetch AWX jobs when the host data is loaded
  useEffect(() => {
    if (host && host.awxHostId) {
      fetchAwxJobs();
    }
  }, [host]);

  const fetchHostDetails = async () => {
    try {
      const response = await axios.get(`/api/v1/hosts/${hostname}`, {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      setHost(response.data);
      setLoading(false);
    } catch (err) {
      setError('Failed to fetch host details');
      setLoading(false);
    }
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'Online':
        return <Icon status="success"><CheckCircleIcon /></Icon>;
      case 'Offline':
        return <Icon status="danger"><ExclamationCircleIcon /></Icon>;
      default:
        return <Icon status="warning"><InfoCircleIcon /></Icon>;
    }
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'Online':
        return 'green';
      case 'Offline':
        return 'red';
      default:
        return 'orange';
    }
  };

  if (loading) {
    return (
      <PageSection>
        <Spinner />
      </PageSection>
    );
  }

  if (error) {
    return (
      <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
        <Alert variant="danger" title={error} />
      </PageSection>
    );
  }

  return (
    <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
      <Stack hasGutter>
        <StackItem>
          <Breadcrumb>
            <BreadcrumbItem onClick={() => navigate('/hosts')}>Hosts</BreadcrumbItem>
            <BreadcrumbItem isActive>{hostname}</BreadcrumbItem>
          </Breadcrumb>
        </StackItem>

        <StackItem>
          <Split>
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
              <Title headingLevel="h1" size="xl">{hostname}</Title>
            </SplitItem>
            <SplitItem>
              <Label 
                color={getStatusColor(host.status)} 
                icon={getStatusIcon(host.status)}
              >
                {host.status}
              </Label>
            </SplitItem>
          </Split>
        </StackItem>

        <StackItem>
          {/* Build tabs array based on available features */}
          {(() => {
            const tabs = [
              {
                eventKey: 0,
                title: <TabTitleText><CubeIcon /> Overview</TabTitleText>,
                content: (
                  <Grid hasGutter style={{marginTop: "20px"}}>
                    <GridItem span={6}>
                      <Card>
                        <CardTitle>
                          <Title headingLevel="h2" size="lg">System Information</Title>
                        </CardTitle>
                        <CardBody>
                          <DescriptionList>
                            <DescriptionListGroup>
                              <DescriptionListTerm>AWX Host ID</DescriptionListTerm>
                              <DescriptionListDescription>
                                {host.awxHostId && host.awxHostUrl ? (
                                  <Button variant="link" component="a" href={host.awxHostUrl} target="_blank" rel="noopener noreferrer">
                                    <Label isCompact variant="outline" color="blue">
                                      {host.awxHostId}
                                    </Label>
                                  </Button>
                                ) : (
                                  <Label isCompact variant="outline" color="gray">
                                    {host.awxHostId ? host.awxHostId : "Not configured"}
                                  </Label>
                                )}
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>IP Address</DescriptionListTerm>
                              <DescriptionListDescription>
                                <Label isCompact>{host.ipAddress}</Label>
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Operating System</DescriptionListTerm>
                              <DescriptionListDescription>{host.os}</DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>CPU Cores</DescriptionListTerm>
                              <DescriptionListDescription>{host.cpuCores}</DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>RAM</DescriptionListTerm>
                              <DescriptionListDescription>{host.ram}</DescriptionListDescription>
                            </DescriptionListGroup>
                          </DescriptionList>
                        </CardBody>
                      </Card>
                    </GridItem>

                    <GridItem span={6}>
                      <Card>
                        <CardTitle>
                          <Title headingLevel="h2" size="lg">Monokit Information</Title>
                        </CardTitle>
                        <CardBody>
                          <DescriptionList>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Version</DescriptionListTerm>
                              <DescriptionListDescription>
                                <Label isCompact>{host.monokitVersion}</Label>
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Wants Update To Monokit version</DescriptionListTerm>
                              <DescriptionListDescription>
                                {host.wantsUpdateTo ? (
                                  <Label color="blue" isCompact>{host.wantsUpdateTo}</Label>
                                ) : (
                                  'No update pending'
                                )}
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Groups</DescriptionListTerm>
                              <DescriptionListDescription>
                                {host.groups === 'nil' ? (
                                  'None'
                                ) : (
                                  <Split hasGutter>
                                    {host.groups.split(',').map(group => (
                                      <SplitItem key={group}>
                                        <Label isCompact>{group.trim()}</Label>
                                      </SplitItem>
                                    ))}
                                  </Split>
                                )}
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Inventory</DescriptionListTerm>
                              <DescriptionListDescription>
                                <Label isCompact>{host.inventory}</Label>
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Components</DescriptionListTerm>
                              <DescriptionListDescription>
                                {host.installedComponents ? (
                                  <Split hasGutter>
                                    {host.installedComponents.split('::').map(component => (
                                      <SplitItem key={component}>
                                        <Label isCompact>{component}</Label>
                                      </SplitItem>
                                    ))}
                                  </Split>
                                ) : (
                                  'None'
                                )}
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Disabled Components</DescriptionListTerm>
                              <DescriptionListDescription>
                                {host.disabledComponents ? (
                                  <Split hasGutter>
                                    {host.disabledComponents.split('::').map(component => (
                                      <SplitItem key={component}>
                                        <Label color="red" isCompact>{component}</Label>
                                      </SplitItem>
                                    ))}
                                  </Split>
                                ) : (
                                  'None'
                                )}
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                            <DescriptionListGroup>
                              <DescriptionListTerm>Configuration</DescriptionListTerm>
                              <DescriptionListDescription>
                                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                                  <Button 
                                    variant="secondary" 
                                    onClick={() => navigate(`/hosts/${hostname}/config`)}
                                  >
                                    Edit Monokit Configuration
                                  </Button>
                                  <Button
                                    variant="primary"
                                    onClick={() => setIsComponentsModalOpen(true)}
                                  >
                                    <CubesIcon style={{ marginRight: '8px' }} />
                                    Manage Components
                                  </Button>
                                </div>
                              </DescriptionListDescription>
                            </DescriptionListGroup>
                          </DescriptionList>
                        </CardBody>
                      </Card>
                    </GridItem>
                  </Grid>
                )
              }
            ];
            
            // Helper function for AWX status label

            // Get status label component with appropriate color
            const getAwxStatusLabel = (status) => {
              const statusMap = {
                successful: 'success',
                failed: 'danger',
                running: 'info',
                pending: 'warning',
                canceled: 'warning'
              };
              return <Label color={statusMap[status?.toLowerCase()] || 'default'}>{status}</Label>;
            };

            // Show AWX Jobs tab if the host has an AWX Host ID
            if (host.awxHostId) {
              tabs.push({
                eventKey: 1,
                title: <TabTitleText><CubeIcon /> AWX Jobs</TabTitleText>,
                content: (
                  <Card style={{marginTop: "20px"}}>
                    <CardTitle>
                      <Title headingLevel="h2" size="lg">Recent AWX Jobs</Title>
                    </CardTitle>
                    <CardBody>
                      {awxLoading ? (
                        <Spinner />
                      ) : awxError ? (
                        <Alert 
                          variant="danger" 
                          title={`Error loading AWX jobs: ${awxError}`}
                          actionLinks={
                            <Button variant="link" onClick={fetchAwxJobs}>Retry</Button>
                          }
                        />
                      ) : !awxJobs.length ? (
                        <EmptyState>
                          <Title headingLevel="h3" size="md">
                            <SearchIcon /> No AWX jobs found
                          </Title>
                          <p>No job records found for this host in the AWX system.</p>
                        </EmptyState>
                      ) : (
                        <AwxJobsTable hostName={hostname} />
                      )}
                    </CardBody>
                  </Card>
                )
              });
            }

            return (
              <Tabs activeKey={activeTabKey} onSelect={(_, key) => setActiveTabKey(key)}>
                {tabs.map(tab => (
                  <Tab 
                    key={tab.eventKey} 
                    eventKey={tab.eventKey} 
                    title={tab.title}
                  >
                    {tab.content}
                  </Tab>
                ))}
              </Tabs>
            );
          })()}
        </StackItem>
      </Stack>

      <EnabledComponents 
        isOpen={isComponentsModalOpen} 
        onClose={() => {
          setIsComponentsModalOpen(false);
          // Refresh host details when modal is closed to get updated component state
          fetchHostDetails();
        }}
        hostname={hostname}
        hostData={host}
      />
    </PageSection>
  );
};

export default HostDetails;
