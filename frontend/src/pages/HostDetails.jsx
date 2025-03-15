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
  Icon
} from '@patternfly/react-core';
import { ArrowLeftIcon, CheckCircleIcon, ExclamationCircleIcon, InfoCircleIcon, CubesIcon } from '@patternfly/react-icons';
import axios from 'axios';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';
import EnabledComponents from '../components/EnabledComponents';

const HostDetails = () => {
  const { hostname } = useParams();
  const navigate = useNavigate();
  const { theme } = useTheme();
  const [host, setHost] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [isComponentsModalOpen, setIsComponentsModalOpen] = useState(false);

  useEffect(() => {
    fetchHostDetails();
  }, [hostname]);

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
          <Grid hasGutter>
            <GridItem span={6}>
              <Card>
                <CardTitle>
                  <Title headingLevel="h2" size="lg">System Information</Title>
                </CardTitle>
                <CardBody>
                  <DescriptionList>
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
                      <DescriptionListTerm>Wants Update To</DescriptionListTerm>
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
