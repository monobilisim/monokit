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
} from '@patternfly/react-core';
import { ArrowLeftIcon } from '@patternfly/react-icons';
import axios from 'axios';

const UserDetails = () => {
  const { username } = useParams();
  const navigate = useNavigate();
  const { theme } = useTheme();
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    fetchUserDetails();
  }, [username]);

  const fetchUserDetails = async () => {
    try {
      const response = await axios.get(`/api/v1/admin/users/${username}`, {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      setUser(response.data);
      setLoading(false);
    } catch (err) {
      setError('Failed to fetch user details');
      setLoading(false);
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
      <PageSection>
        <Alert variant="danger" isInline title={error} style={{ marginBottom: '1rem' }} />
      </PageSection>
    );
  }

  return (
    <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
      <Stack hasGutter>
        <StackItem>
          <Breadcrumb>
            <BreadcrumbItem onClick={() => navigate('/users')}>Users</BreadcrumbItem>
            <BreadcrumbItem isActive>{username}</BreadcrumbItem>
          </Breadcrumb>
        </StackItem>

        <StackItem>
          <Split>
            <SplitItem>
              <Button 
                variant="link" 
                icon={<ArrowLeftIcon />} 
                onClick={() => navigate('/users')}
              >
                Back to Users
              </Button>
            </SplitItem>
            <SplitItem isFilled>
              <Title headingLevel="h1" size="xl">{username}</Title>
            </SplitItem>
            <SplitItem>
              <Label 
                color={user.role === 'admin' ? 'blue' : 'green'}
                style={{ 
                  padding: '4px 12px',
                  fontSize: '14px',
                  borderRadius: '30px',
                }}
              >
                {user.role}
              </Label>
            </SplitItem>
          </Split>
        </StackItem>

        <StackItem>
          <Grid hasGutter>
            <GridItem span={6}>
              <Card>
                <CardTitle>
                  <Title headingLevel="h2" size="lg">User Information</Title>
                </CardTitle>
                <CardBody>
                  <DescriptionList>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Email</DescriptionListTerm>
                      <DescriptionListDescription>
                        <Label isCompact>{user.email}</Label>
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Groups</DescriptionListTerm>
                      <DescriptionListDescription>
                        {user.groups === 'nil' ? (
                          'None'
                        ) : (
                          <Split hasGutter>
                            {user.groups.split(',').map(group => (
                              <SplitItem key={group}>
                                <Label isCompact>{group.trim()}</Label>
                              </SplitItem>
                            ))}
                          </Split>
                        )}
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Inventories</DescriptionListTerm>
                      <DescriptionListDescription>
                        {user.inventories === 'nil' ? (
                          'None'
                        ) : (
                          <Split hasGutter>
                            {user.inventories.split(',').map(inv => (
                              <SplitItem key={inv}>
                                <Label color="purple" isCompact>{inv.trim()}</Label>
                              </SplitItem>
                            ))}
                          </Split>
                        )}
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                    
                    {/* Add disabled components section here */}
                    {user.disabledComponents && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>Disabled Components</DescriptionListTerm>
                        <DescriptionListDescription>
                          {user.disabledComponents === 'nil' ? (
                            'None'
                          ) : (
                            <Split hasGutter>
                              {(() => {
                                // Get components list, removing 'nil' and empty strings
                                const components = user.disabledComponents
                                  .split('::')
                                  .filter(comp => comp !== 'nil' && comp.trim() !== '');
                                
                                // If filtered list is empty, show 'None' instead
                                return components.length > 0 ? (
                                  components.map(comp => (
                                    <SplitItem key={comp}>
                                      <Label color="red" isCompact>{comp.trim()}</Label>
                                    </SplitItem>
                                  ))
                                ) : 'None';
                              })()}
                            </Split>
                          )}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                  </DescriptionList>
                </CardBody>
              </Card>
            </GridItem>

            <GridItem span={6}>
              <Card>
                <CardTitle>
                  <Title headingLevel="h2" size="lg">Actions</Title>
                </CardTitle>
                <CardBody>
                  <Stack hasGutter>
                    <Button 
                      variant="primary"
                      onClick={() => navigate(`/users/${username}/edit`)}
                    >
                      Edit User
                    </Button>
                    <Button 
                      variant="danger"
                      onClick={() => {
                        // Add delete confirmation modal
                      }}
                    >
                      Delete User
                    </Button>
                  </Stack>
                </CardBody>
              </Card>
            </GridItem>
          </Grid>
        </StackItem>
      </Stack>
    </PageSection>
  );
};

export default UserDetails;