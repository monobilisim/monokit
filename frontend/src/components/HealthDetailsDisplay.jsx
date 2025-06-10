import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import { 
  Spinner, 
  Alert, 
  Card, 
  CardBody, 
  CardTitle, 
  Title,
  Grid,
  GridItem,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Progress,
  ProgressMeasureLocation,
  Label,
  Stack,
  StackItem,
  Flex,
  FlexItem,
  Icon
} from '@patternfly/react-core';
import { 
  CheckCircleIcon, 
  ExclamationTriangleIcon, 
  ExclamationCircleIcon,
  ServerIcon,
  MemoryIcon,
  HddIcon,
  TachometerAltIcon,
  NetworkIcon,
  UsersIcon,
  UserIcon,
  BuildingIcon,
  LockIcon,
  ConnectedIcon,
  DisconnectedIcon,
  CubesIcon,
  DatabaseIcon
} from '@patternfly/react-icons';
import axios from 'axios';

const HealthDetailsDisplay = ({ hostname, toolName }) => {
  const [healthData, setHealthData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchSpecificHealthData = async () => {
      if (!hostname || !toolName) {
        setHealthData(null);
        setLoading(false);
        setError(null);
        return;
      }

      setLoading(true);
      setError(null);
      try {
        const response = await axios.get(`/api/v1/hosts/${hostname}/health/${toolName}`, {
          headers: {
            Authorization: localStorage.getItem('token'),
          },
        });
        setHealthData(response.data);
      } catch (err) {
        console.error(`Error fetching health data for ${toolName} on ${hostname}:`, err);
        setError(err.response?.data?.error || err.message || `Failed to fetch health data for ${toolName}`);
        setHealthData(null);
      } finally {
        setLoading(false);
      }
    };

    fetchSpecificHealthData();
  }, [hostname, toolName]);

  const getProgressVariant = (percentage) => {
    if (percentage >= 90) return 'danger';
    if (percentage >= 75) return 'warning';
    return 'success';
  };

  const getStatusIcon = (exceeded, percentage) => {
    if (exceeded || percentage >= 90) {
      return <Icon status="danger"><ExclamationCircleIcon /></Icon>;
    }
    if (percentage >= 75) {
      return <Icon status="warning"><ExclamationTriangleIcon /></Icon>;
    }
    return <Icon status="success"><CheckCircleIcon /></Icon>;
  };

  const formatBytes = (bytes, decimals = 2) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
  };

  const renderOSHealthData = (data) => {
    return (
      <Stack hasGutter>
        {/* System Information */}
        {data.System && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <ServerIcon style={{ marginRight: '8px' }} />
                  System Information
                </Title>
              </CardTitle>
              <CardBody>
                <DescriptionList isHorizontal>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Hostname</DescriptionListTerm>
                    <DescriptionListDescription>{data.System.Hostname}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Operating System</DescriptionListTerm>
                    <DescriptionListDescription>{data.System.OS}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Kernel Version</DescriptionListTerm>
                    <DescriptionListDescription>{data.System.KernelVer}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Last Checked</DescriptionListTerm>
                    <DescriptionListDescription>{data.System.LastChecked}</DescriptionListDescription>
                  </DescriptionListGroup>
                  {data.System.Uptime && (
                    <DescriptionListGroup>
                      <DescriptionListTerm>Uptime</DescriptionListTerm>
                      <DescriptionListDescription>{data.System.Uptime}</DescriptionListDescription>
                    </DescriptionListGroup>
                  )}
                </DescriptionList>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Memory Usage */}
        {data.Memory && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <MemoryIcon style={{ marginRight: '8px' }} />
                  Memory Usage
                </Title>
              </CardTitle>
              <CardBody>
                <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsSm' }}>
                  <FlexItem>
                    <Flex alignItems={{ default: 'alignItemsCenter' }}>
                      <FlexItem>
                        {getStatusIcon(data.Memory.Exceeded, data.Memory.UsedPct)}
                      </FlexItem>
                      <FlexItem>
                        <strong>{data.Memory.Used} / {data.Memory.Total}</strong>
                      </FlexItem>
                      <FlexItem>
                        <Label color={data.Memory.Exceeded ? 'red' : data.Memory.UsedPct >= 75 ? 'orange' : 'green'}>
                          {data.Memory.UsedPct?.toFixed(1)}% used
                        </Label>
                      </FlexItem>
                    </Flex>
                  </FlexItem>
                  <FlexItem>
                    <Progress
                      value={data.Memory.UsedPct}
                      title="Memory usage"
                      variant={getProgressVariant(data.Memory.UsedPct)}
                      measureLocation={ProgressMeasureLocation.outside}
                    />
                  </FlexItem>
                  {data.Memory.Limit && (
                    <FlexItem>
                      <DescriptionList isCompact isHorizontal>
                        <DescriptionListGroup>
                          <DescriptionListTerm>Warning Limit</DescriptionListTerm>
                          <DescriptionListDescription>{data.Memory.Limit}%</DescriptionListDescription>
                        </DescriptionListGroup>
                        <DescriptionListGroup>
                          <DescriptionListTerm>Status</DescriptionListTerm>
                          <DescriptionListDescription>
                            {data.Memory.Exceeded ? (
                              <Label color="red">Limit Exceeded</Label>
                            ) : (
                              <Label color="green">Within Limits</Label>
                            )}
                          </DescriptionListDescription>
                        </DescriptionListGroup>
                      </DescriptionList>
                    </FlexItem>
                  )}
                </Flex>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* System Load */}
        {data.SystemLoad && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <TachometerAltIcon style={{ marginRight: '8px' }} />
                  System Load
                </Title>
              </CardTitle>
              <CardBody>
                <Grid hasGutter>
                  <GridItem span={6}>
                    <DescriptionList isCompact>
                      <DescriptionListGroup>
                        <DescriptionListTerm>CPU Count</DescriptionListTerm>
                        <DescriptionListDescription>{data.SystemLoad.CPUCount} cores</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Load Multiplier</DescriptionListTerm>
                        <DescriptionListDescription>{data.SystemLoad.Multiplier}x</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Status</DescriptionListTerm>
                        <DescriptionListDescription>
                          {data.SystemLoad.Exceeded ? (
                            <Label color="red">High Load</Label>
                          ) : (
                            <Label color="green">Normal</Label>
                          )}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    </DescriptionList>
                  </GridItem>
                  <GridItem span={6}>
                    <DescriptionList isCompact>
                      <DescriptionListGroup>
                        <DescriptionListTerm>1-min Load</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={data.SystemLoad.Load1 > data.SystemLoad.CPUCount ? 'orange' : 'green'}>
                            {data.SystemLoad.Load1}
                          </Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>5-min Load</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={data.SystemLoad.Load5 > data.SystemLoad.CPUCount ? 'orange' : 'green'}>
                            {data.SystemLoad.Load5}
                          </Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>15-min Load</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={data.SystemLoad.Load15 > data.SystemLoad.CPUCount ? 'orange' : 'green'}>
                            {data.SystemLoad.Load15}
                          </Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    </DescriptionList>
                  </GridItem>
                </Grid>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Disk Usage */}
        {data.Disk && Array.isArray(data.Disk) && data.Disk.length > 0 && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <HddIcon style={{ marginRight: '8px' }} />
                  Disk Usage
                </Title>
              </CardTitle>
              <CardBody>
                <Stack hasGutter>
                  {data.Disk.map((disk, index) => (
                    <StackItem key={index}>
                      <Card isCompact>
                        <CardBody>
                          <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsSm' }}>
                            <FlexItem>
                              <Flex alignItems={{ default: 'alignItemsCenter' }}>
                                <FlexItem>
                                  {getStatusIcon(false, disk.UsedPct)}
                                </FlexItem>
                                <FlexItem>
                                  <strong>{disk.Device}</strong> ({disk.Fstype})
                                </FlexItem>
                                <FlexItem>
                                  <Label color={disk.UsedPct >= 90 ? 'red' : disk.UsedPct >= 75 ? 'orange' : 'green'}>
                                    {disk.UsedPct?.toFixed(1)}% used
                                  </Label>
                                </FlexItem>
                              </Flex>
                            </FlexItem>
                            <FlexItem>
                              <DescriptionList isCompact isHorizontal>
                                <DescriptionListGroup>
                                  <DescriptionListTerm>Mount Point</DescriptionListTerm>
                                  <DescriptionListDescription>{disk.Mountpoint}</DescriptionListDescription>
                                </DescriptionListGroup>
                                <DescriptionListGroup>
                                  <DescriptionListTerm>Used / Total</DescriptionListTerm>
                                  <DescriptionListDescription>{disk.Used} / {disk.Total}</DescriptionListDescription>
                                </DescriptionListGroup>
                              </DescriptionList>
                            </FlexItem>
                            <FlexItem>
                              <Progress
                                value={disk.UsedPct}
                                title={`${disk.Device} usage`}
                                variant={getProgressVariant(disk.UsedPct)}
                                measureLocation={ProgressMeasureLocation.outside}
                              />
                            </FlexItem>
                          </Flex>
                        </CardBody>
                      </Card>
                    </StackItem>
                  ))}
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Additional Sections for non-null data */}
        {data.SystemdUnits && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">Systemd Units</Title>
              </CardTitle>
              <CardBody>
                <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '3px' }}>
                  {JSON.stringify(data.SystemdUnits, null, 2)}
                </pre>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {data.ZFSPools && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">ZFS Pools</Title>
              </CardTitle>
              <CardBody>
                <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '3px' }}>
                  {JSON.stringify(data.ZFSPools, null, 2)}
                </pre>
              </CardBody>
            </Card>
          </StackItem>
        )}
      </Stack>
    );
  };

  const renderPritunlHealthData = (data) => {
    const formatTimestamp = (timestamp) => {
      if (!timestamp) return 'N/A';
      try {
        return new Date(timestamp).toLocaleString();
      } catch (e) {
        return timestamp; // Return as-is if it's not a valid date
      }
    };

    return (
      <Stack hasGutter>
        {/* Overall Status */}
        <StackItem>
          <Card>
            <CardTitle>
              <Title headingLevel="h5" size="md">
                <LockIcon style={{ marginRight: '8px' }} />
                Pritunl VPN Status
              </Title>
            </CardTitle>
            <CardBody>
              <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsSm' }}>
                <FlexItem>
                  <Flex alignItems={{ default: 'alignItemsCenter' }}>
                    <FlexItem>
                      {data.IsHealthy ? (
                        <Icon status="success"><CheckCircleIcon /></Icon>
                      ) : (
                        <Icon status="danger"><ExclamationCircleIcon /></Icon>
                      )}
                    </FlexItem>
                    <FlexItem>
                      <Label color={data.IsHealthy ? 'green' : 'red'}>
                        {data.IsHealthy ? 'Healthy' : 'Unhealthy'}
                      </Label>
                    </FlexItem>
                  </Flex>
                </FlexItem>
                <FlexItem>
                  <DescriptionList isHorizontal>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Version</DescriptionListTerm>
                      <DescriptionListDescription>{data.Version || 'N/A'}</DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Last Checked</DescriptionListTerm>
                      <DescriptionListDescription>{formatTimestamp(data.LastChecked)}</DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Servers</DescriptionListTerm>
                      <DescriptionListDescription>{data.Servers?.length || 0}</DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Users</DescriptionListTerm>
                      <DescriptionListDescription>{data.Users?.length || 0}</DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Organizations</DescriptionListTerm>
                      <DescriptionListDescription>{data.Organizations?.length || 0}</DescriptionListDescription>
                    </DescriptionListGroup>
                  </DescriptionList>
                </FlexItem>
              </Flex>
            </CardBody>
          </Card>
        </StackItem>

        {/* Servers Section */}
        {data.Servers && data.Servers.length > 0 && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <ServerIcon style={{ marginRight: '8px' }} />
                  VPN Servers
                </Title>
              </CardTitle>
              <CardBody>
                <Stack hasGutter>
                  {data.Servers.map((server, index) => (
                    <StackItem key={index}>
                      <Card isCompact>
                        <CardBody>
                          <Flex alignItems={{ default: 'alignItemsCenter' }}>
                            <FlexItem>
                              {server.IsHealthy ? (
                                <Icon status="success"><CheckCircleIcon /></Icon>
                              ) : (
                                <Icon status="danger"><ExclamationCircleIcon /></Icon>
                              )}
                            </FlexItem>
                            <FlexItem>
                              <strong>{server.Name}</strong>
                            </FlexItem>
                            <FlexItem>
                              <Label color={server.Status === 'online' ? 'green' : 'red'}>
                                {server.Status}
                              </Label>
                            </FlexItem>
                          </Flex>
                        </CardBody>
                      </Card>
                    </StackItem>
                  ))}
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Users Section */}
        {data.Users && data.Users.length > 0 && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <UsersIcon style={{ marginRight: '8px' }} />
                  VPN Users
                </Title>
              </CardTitle>
              <CardBody>
                <Stack hasGutter>
                  {data.Users.map((user, index) => (
                    <StackItem key={index}>
                      <Card isCompact>
                        <CardBody>
                          <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsSm' }}>
                            <FlexItem>
                              <Flex alignItems={{ default: 'alignItemsCenter' }}>
                                <FlexItem>
                                  {user.Status === 'online' ? (
                                    <Icon status="success"><ConnectedIcon /></Icon>
                                  ) : (
                                    <Icon status="warning"><DisconnectedIcon /></Icon>
                                  )}
                                </FlexItem>
                                <FlexItem>
                                  <UserIcon style={{ marginRight: '4px' }} />
                                  <strong>{user.Name}</strong>
                                </FlexItem>
                                <FlexItem>
                                  <Label color={user.Status === 'online' ? 'green' : 'grey'}>
                                    {user.Status}
                                  </Label>
                                </FlexItem>
                                {user.ConnectedClients && user.ConnectedClients.length > 0 && (
                                  <FlexItem>
                                    <Label color="blue">
                                      {user.ConnectedClients.length} client{user.ConnectedClients.length !== 1 ? 's' : ''}
                                    </Label>
                                  </FlexItem>
                                )}
                              </Flex>
                            </FlexItem>
                            <FlexItem>
                              <DescriptionList isCompact isHorizontal>
                                <DescriptionListGroup>
                                  <DescriptionListTerm>Organization</DescriptionListTerm>
                                  <DescriptionListDescription>{user.Organization}</DescriptionListDescription>
                                </DescriptionListGroup>
                                {user.ConnectedClients && user.ConnectedClients.length > 0 && (
                                  <DescriptionListGroup>
                                    <DescriptionListTerm>Client IPs</DescriptionListTerm>
                                    <DescriptionListDescription>
                                      {user.ConnectedClients.map(client => client.IPAddress).join(', ')}
                                    </DescriptionListDescription>
                                  </DescriptionListGroup>
                                )}
                              </DescriptionList>
                            </FlexItem>
                          </Flex>
                        </CardBody>
                      </Card>
                    </StackItem>
                  ))}
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Organizations Section */}
        {data.Organizations && data.Organizations.length > 0 && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <BuildingIcon style={{ marginRight: '8px' }} />
                  Organizations
                </Title>
              </CardTitle>
              <CardBody>
                <Stack hasGutter>
                  {data.Organizations.map((org, index) => (
                    <StackItem key={index}>
                      <Card isCompact>
                        <CardBody>
                          <Flex alignItems={{ default: 'alignItemsCenter' }}>
                            <FlexItem>
                              {org.IsActive ? (
                                <Icon status="success"><CheckCircleIcon /></Icon>
                              ) : (
                                <Icon status="warning"><ExclamationTriangleIcon /></Icon>
                              )}
                            </FlexItem>
                            <FlexItem>
                              <strong>{org.Name}</strong>
                            </FlexItem>
                            <FlexItem>
                              <Label color={org.IsActive ? 'green' : 'orange'}>
                                {org.IsActive ? 'Active' : 'Inactive'}
                              </Label>
                            </FlexItem>
                          </Flex>
                        </CardBody>
                      </Card>
                    </StackItem>
                  ))}
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}
      </Stack>
    );
  };

  const renderK8sHealthData = (data) => {
    return (
      <Stack hasGutter>
        {/* Errors Section */}
        {data.Errors && data.Errors.length > 0 && (
          <StackItem>
            <Alert variant="danger" title="Kubernetes Health Errors">
              <Stack hasGutter>
                {data.Errors.map((error, index) => (
                  <StackItem key={index}>- {error}</StackItem>
                ))}
              </Stack>
            </Alert>
          </StackItem>
        )}

        {/* Nodes Section */}
        {data.Nodes && data.Nodes.length > 0 && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <ServerIcon style={{ marginRight: '8px' }} />
                  Kubernetes Nodes
                </Title>
              </CardTitle>
              <CardBody>
                <Stack hasGutter>
                  {data.Nodes.map((node, index) => (
                    <StackItem key={index}>
                      <Card isCompact>
                        <CardBody>
                          <Flex alignItems={{ default: 'alignItemsCenter' }}>
                            <FlexItem>
                              {node.IsReady ? (
                                <Icon status="success"><CheckCircleIcon /></Icon>
                              ) : (
                                <Icon status="danger"><ExclamationCircleIcon /></Icon>
                              )}
                            </FlexItem>
                            <FlexItem>
                              <strong>{node.Name}</strong>
                            </FlexItem>
                            <FlexItem>
                              <Label color={node.Role === 'master' ? 'blue' : 'green'}>
                                {node.Role}
                              </Label>
                            </FlexItem>
                            <FlexItem>
                              <Label color={node.IsReady ? 'green' : 'red'}>
                                {node.Status}
                              </Label>
                            </FlexItem>
                            {node.Reason && (
                              <FlexItem>
                                <span style={{ fontSize: '0.875rem', color: '#6a6e73' }}>
                                  {node.Reason}
                                </span>
                              </FlexItem>
                            )}
                          </Flex>
                        </CardBody>
                      </Card>
                    </StackItem>
                  ))}
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Problematic Pods Section */}
        {data.Pods && data.Pods.filter(pod => pod.IsProblem).length > 0 && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <CubesIcon style={{ marginRight: '8px' }} />
                  Problematic Pods
                </Title>
              </CardTitle>
              <CardBody>
                <Stack hasGutter>
                  {data.Pods.filter(pod => pod.IsProblem).map((pod, index) => (
                    <StackItem key={index}>
                      <Card isCompact>
                        <CardBody>
                          <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsSm' }}>
                            <FlexItem>
                              <Flex alignItems={{ default: 'alignItemsCenter' }}>
                                <FlexItem>
                                  <Icon status="warning"><ExclamationTriangleIcon /></Icon>
                                </FlexItem>
                                <FlexItem>
                                  <strong>{pod.Namespace}/{pod.Name}</strong>
                                </FlexItem>
                                <FlexItem>
                                  <Label color="orange">{pod.Phase}</Label>
                                </FlexItem>
                              </Flex>
                            </FlexItem>
                            {pod.ContainerStates && pod.ContainerStates.length > 0 && (
                              <FlexItem>
                                <DescriptionList isCompact>
                                  {pod.ContainerStates.filter(container => !container.IsReady).map((container, idx) => (
                                    <DescriptionListGroup key={idx}>
                                      <DescriptionListTerm>{container.Name}</DescriptionListTerm>
                                      <DescriptionListDescription>
                                        {container.State} - {container.Reason}
                                        {container.Message && ` (${container.Message.slice(0, 100)}${container.Message.length > 100 ? '...' : ''})`}
                                      </DescriptionListDescription>
                                    </DescriptionListGroup>
                                  ))}
                                </DescriptionList>
                              </FlexItem>
                            )}
                          </Flex>
                        </CardBody>
                      </Card>
                    </StackItem>
                  ))}
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* RKE2 Ingress Nginx Section */}
        {data.Rke2IngressNginx && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <NetworkIcon style={{ marginRight: '8px' }} />
                  RKE2 Ingress Nginx
                </Title>
              </CardTitle>
              <CardBody>
                {data.Rke2IngressNginx.Error ? (
                  <Alert variant="danger" title="Error">
                    {data.Rke2IngressNginx.Error}
                  </Alert>
                ) : (
                  <DescriptionList isHorizontal>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Manifest Available</DescriptionListTerm>
                      <DescriptionListDescription>
                        <Label color={data.Rke2IngressNginx.ManifestAvailable ? 'green' : 'red'}>
                          {data.Rke2IngressNginx.ManifestAvailable ? 'Yes' : 'No'}
                        </Label>
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                    {data.Rke2IngressNginx.ManifestPath && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>Manifest Path</DescriptionListTerm>
                        <DescriptionListDescription>{data.Rke2IngressNginx.ManifestPath}</DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {data.Rke2IngressNginx.PublishServiceEnabled !== null && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>Publish Service</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={data.Rke2IngressNginx.PublishServiceEnabled ? 'green' : 'orange'}>
                            {data.Rke2IngressNginx.PublishServiceEnabled ? 'Enabled' : 'Disabled'}
                          </Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                  </DescriptionList>
                )}
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Cert Manager Section */}
        {data.CertManager && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <LockIcon style={{ marginRight: '8px' }} />
                  Cert Manager
                </Title>
              </CardTitle>
              <CardBody>
                {data.CertManager.Error ? (
                  <Alert variant="danger" title="Error">
                    {data.CertManager.Error}
                  </Alert>
                ) : (
                  <Stack hasGutter>
                    <StackItem>
                      <DescriptionList isCompact isHorizontal>
                        <DescriptionListGroup>
                          <DescriptionListTerm>Namespace Available</DescriptionListTerm>
                          <DescriptionListDescription>
                            <Label color={data.CertManager.NamespaceAvailable ? 'green' : 'red'}>
                              {data.CertManager.NamespaceAvailable ? 'Yes' : 'No'}
                            </Label>
                          </DescriptionListDescription>
                        </DescriptionListGroup>
                      </DescriptionList>
                    </StackItem>
                    {data.CertManager.Certificates && data.CertManager.Certificates.length > 0 && (
                      <StackItem>
                        <Stack hasGutter>
                          {data.CertManager.Certificates.map((cert, index) => (
                            <StackItem key={index}>
                              <Card isCompact>
                                <CardBody>
                                  <Flex alignItems={{ default: 'alignItemsCenter' }}>
                                    <FlexItem>
                                      {cert.IsReady ? (
                                        <Icon status="success"><CheckCircleIcon /></Icon>
                                      ) : (
                                        <Icon status="danger"><ExclamationCircleIcon /></Icon>
                                      )}
                                    </FlexItem>
                                    <FlexItem>
                                      <strong>{cert.Name}</strong>
                                    </FlexItem>
                                    <FlexItem>
                                      <Label color={cert.IsReady ? 'green' : 'red'}>
                                        {cert.IsReady ? 'Ready' : 'Not Ready'}
                                      </Label>
                                    </FlexItem>
                                    {cert.Reason && (
                                      <FlexItem>
                                        <span style={{ fontSize: '0.875rem', color: '#6a6e73' }}>
                                          {cert.Reason}
                                        </span>
                                      </FlexItem>
                                    )}
                                  </Flex>
                                </CardBody>
                              </Card>
                            </StackItem>
                          ))}
                        </Stack>
                      </StackItem>
                    )}
                  </Stack>
                )}
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Last Checked */}
        {data.LastChecked && (
          <StackItem>
            <Card isCompact>
              <CardBody>
                <DescriptionList isCompact isHorizontal>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Last Checked</DescriptionListTerm>
                    <DescriptionListDescription>{data.LastChecked}</DescriptionListDescription>
                  </DescriptionListGroup>
                </DescriptionList>
              </CardBody>
            </Card>
          </StackItem>
        )}
      </Stack>
    );
  };

  const renderRedisHealthData = (data) => {
    return (
      <Stack hasGutter>
        {/* Service Status Section */}
        <StackItem>
          <Card>
            <CardTitle>
              <Title headingLevel="h5" size="md">
                <DatabaseIcon style={{ marginRight: '8px' }} />
                Service Status
              </Title>
            </CardTitle>
            <CardBody>
              <Flex alignItems={{ default: 'alignItemsCenter' }}>
                <FlexItem>
                  {data.Service.Active ? (
                    <Icon status="success"><CheckCircleIcon /></Icon>
                  ) : (
                    <Icon status="danger"><ExclamationCircleIcon /></Icon>
                  )}
                </FlexItem>
                <FlexItem>
                  <strong>Redis Service</strong>
                </FlexItem>
                <FlexItem>
                  <Label color={data.Service.Active ? 'green' : 'red'}>
                    {data.Service.Active ? 'Active' : 'Inactive'}
                  </Label>
                </FlexItem>
              </Flex>
            </CardBody>
          </Card>
        </StackItem>

        {/* Connection Status Section */}
        <StackItem>
          <Card>
            <CardTitle>
              <Title headingLevel="h5" size="md">
                <ConnectedIcon style={{ marginRight: '8px' }} />
                Connection Status
              </Title>
            </CardTitle>
            <CardBody>
              <Stack hasGutter>
                <StackItem>
                  <Card isCompact>
                    <CardBody>
                      <Flex alignItems={{ default: 'alignItemsCenter' }}>
                        <FlexItem>
                          {data.Connection.Pingable ? (
                            <Icon status="success"><CheckCircleIcon /></Icon>
                          ) : (
                            <Icon status="danger"><ExclamationCircleIcon /></Icon>
                          )}
                        </FlexItem>
                        <FlexItem>
                          <strong>Connection</strong>
                        </FlexItem>
                        <FlexItem>
                          <Label color={data.Connection.Pingable ? 'green' : 'red'}>
                            {data.Connection.Pingable ? 'Connected' : 'Disconnected'}
                          </Label>
                        </FlexItem>
                      </Flex>
                    </CardBody>
                  </Card>
                </StackItem>
                <StackItem>
                  <Card isCompact>
                    <CardBody>
                      <Flex alignItems={{ default: 'alignItemsCenter' }}>
                        <FlexItem>
                          {data.Connection.Writeable ? (
                            <Icon status="success"><CheckCircleIcon /></Icon>
                          ) : (
                            <Icon status="danger"><ExclamationCircleIcon /></Icon>
                          )}
                        </FlexItem>
                        <FlexItem>
                          <strong>Write Access</strong>
                        </FlexItem>
                        <FlexItem>
                          <Label color={data.Connection.Writeable ? 'green' : 'red'}>
                            {data.Connection.Writeable ? 'Writeable' : 'Read-only'}
                          </Label>
                        </FlexItem>
                      </Flex>
                    </CardBody>
                  </Card>
                </StackItem>
                <StackItem>
                  <Card isCompact>
                    <CardBody>
                      <Flex alignItems={{ default: 'alignItemsCenter' }}>
                        <FlexItem>
                          {data.Connection.Readable ? (
                            <Icon status="success"><CheckCircleIcon /></Icon>
                          ) : (
                            <Icon status="danger"><ExclamationCircleIcon /></Icon>
                          )}
                        </FlexItem>
                        <FlexItem>
                          <strong>Read Access</strong>
                        </FlexItem>
                        <FlexItem>
                          <Label color={data.Connection.Readable ? 'green' : 'red'}>
                            {data.Connection.Readable ? 'Readable' : 'Not readable'}
                          </Label>
                        </FlexItem>
                      </Flex>
                    </CardBody>
                  </Card>
                </StackItem>
              </Stack>
            </CardBody>
          </Card>
        </StackItem>

        {/* Role Status Section */}
        <StackItem>
          <Card>
            <CardTitle>
              <Title headingLevel="h5" size="md">
                <ServerIcon style={{ marginRight: '8px' }} />
                Role Status
              </Title>
            </CardTitle>
            <CardBody>
              <Flex alignItems={{ default: 'alignItemsCenter' }}>
                <FlexItem>
                  <Icon status="info"><ServerIcon /></Icon>
                </FlexItem>
                <FlexItem>
                  <strong>Redis Role</strong>
                </FlexItem>
                <FlexItem>
                  <Label color={data.Role.IsMaster ? 'blue' : 'green'}>
                    {data.Role.IsMaster ? 'Master' : 'Slave'}
                  </Label>
                </FlexItem>
              </Flex>
            </CardBody>
          </Card>
        </StackItem>

        {/* Sentinel Status Section */}
        {data.Sentinel && (
          <StackItem>
            <Card>
              <CardTitle>
                <Title headingLevel="h5" size="md">
                  <NetworkIcon style={{ marginRight: '8px' }} />
                  Sentinel Status
                </Title>
              </CardTitle>
              <CardBody>
                <Stack hasGutter>
                  <StackItem>
                    <Card isCompact>
                      <CardBody>
                        <Flex alignItems={{ default: 'alignItemsCenter' }}>
                          <FlexItem>
                            {data.Sentinel.Active ? (
                              <Icon status="success"><CheckCircleIcon /></Icon>
                            ) : (
                              <Icon status="danger"><ExclamationCircleIcon /></Icon>
                            )}
                          </FlexItem>
                          <FlexItem>
                            <strong>Sentinel Service</strong>
                          </FlexItem>
                          <FlexItem>
                            <Label color={data.Sentinel.Active ? 'green' : 'red'}>
                              {data.Sentinel.Active ? 'Active' : 'Inactive'}
                            </Label>
                          </FlexItem>
                        </Flex>
                      </CardBody>
                    </Card>
                  </StackItem>
                  {data.Role.IsMaster && (
                    <StackItem>
                      <Card isCompact>
                        <CardBody>
                          <Flex alignItems={{ default: 'alignItemsCenter' }}>
                            <FlexItem>
                              {data.Sentinel.SlaveCount === data.Sentinel.ExpectedCount ? (
                                <Icon status="success"><CheckCircleIcon /></Icon>
                              ) : (
                                <Icon status="warning"><ExclamationTriangleIcon /></Icon>
                              )}
                            </FlexItem>
                            <FlexItem>
                              <strong>Slave Count</strong>
                            </FlexItem>
                            <FlexItem>
                              <Label color={data.Sentinel.SlaveCount === data.Sentinel.ExpectedCount ? 'green' : 'orange'}>
                                {data.Sentinel.SlaveCount} / {data.Sentinel.ExpectedCount}
                              </Label>
                            </FlexItem>
                          </Flex>
                        </CardBody>
                      </Card>
                    </StackItem>
                  )}
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}

        {/* Last Checked */}
        {data.LastChecked && (
          <StackItem>
            <Card isCompact>
              <CardBody>
                <DescriptionList isCompact isHorizontal>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Last Checked</DescriptionListTerm>
                    <DescriptionListDescription>{data.LastChecked}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Version</DescriptionListTerm>
                    <DescriptionListDescription>{data.Version}</DescriptionListDescription>
                  </DescriptionListGroup>
                </DescriptionList>
              </CardBody>
            </Card>
          </StackItem>
        )}
      </Stack>
    );
  };

  const renderGenericHealthData = (data, toolName) => {
    return (
      <Card>
        <CardTitle>
          <Title headingLevel="h5" size="md">Raw Data for {toolName}</Title>
        </CardTitle>
        <CardBody>
          <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '3px' }}>
            {JSON.stringify(data, null, 2)}
          </pre>
        </CardBody>
      </Card>
    );
  };

  if (!toolName) {
    return null; 
  }

  if (loading) {
    return <Spinner aria-label={`Loading ${toolName} data...`} />;
  }

  if (error) {
    return (
      <Alert variant="danger" title={`Error loading ${toolName} data`}>
        {error}
      </Alert>
    );
  }

  if (!healthData) {
    return <p>No data available for {toolName}.</p>;
  }

  return (
    <div>
      <Title headingLevel="h4" size="lg" style={{ marginBottom: '1rem' }}>
        Health Details: {toolName}
      </Title>
      {(() => {
        if (toolName === 'osHealth') {
          return renderOSHealthData(healthData);
        } else if (toolName === 'pritunlHealth') {
          return renderPritunlHealthData(healthData);
        } else if (toolName === 'k8sHealth') {
          return renderK8sHealthData(healthData);
        } else if (toolName === 'redisHealth') {
          return renderRedisHealthData(healthData);
        } else {
          return renderGenericHealthData(healthData, toolName);
        }
      })()}
    </div>
  );
};

HealthDetailsDisplay.propTypes = {
  hostname: PropTypes.string.isRequired,
  toolName: PropTypes.string,
};

HealthDetailsDisplay.defaultProps = {
  toolName: '',
};

export default HealthDetailsDisplay;