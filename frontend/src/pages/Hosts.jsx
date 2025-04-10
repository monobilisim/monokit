import React, { useState, useEffect } from 'react';
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Flex,
  FlexItem,
  PageSection,
  Pagination,
  SearchInput,
  Spinner,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Modal,
  ModalVariant,
  Checkbox,
  Alert,
  AlertActionCloseButton,
  AlertGroup,
  AlertVariant,
  Dropdown,
  DropdownItem,
  DropdownSeparator,
  KebabToggle,
  ButtonVariant,
  Tooltip,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td, ActionsColumn } from '@patternfly/react-table';
import { ExclamationTriangleIcon, PlusCircleIcon } from '@patternfly/react-icons';
import { useNavigate } from 'react-router-dom';
import { getHosts, forceDeleteHost } from '../utils/api';
import axios from 'axios';
import DeleteConfirmationModal from '../components/DeleteConfirmationModal';
import ForceDeleteConfirmationModal from '../components/ForceDeleteConfirmationModal';
import AwxHostAdd from '../components/AwxHostAdd';

const Hosts = ({ onAuthError }) => {
  const [hosts, setHosts] = useState([]);
  const [filteredHosts, setFilteredHosts] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchValue, setSearchValue] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [selectedHosts, setSelectedHosts] = useState([]);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isForceDeleteModalOpen, setIsForceDeleteModalOpen] = useState(false);
  const [isAwxHostAddModalOpen, setIsAwxHostAddModalOpen] = useState(false);
  const [hostToForceDelete, setHostToForceDelete] = useState(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [alerts, setAlerts] = useState([]);
  const [actionDropdownOpen, setActionDropdownOpen] = useState({});
  const navigate = useNavigate();

  const addAlert = (title, variant, description = '') => {
    const key = new Date().getTime();
    setAlerts([...alerts, { title, variant, key, description }]);

    // Automatically remove the alert after 5 seconds
    setTimeout(() => {
      setAlerts(currentAlerts => currentAlerts.filter(alert => alert.key !== key));
    }, 5000);
  };

  // Fetch hosts data
  useEffect(() => {
    const fetchHosts = async () => {
      try {
        setIsLoading(true);
        setError(null);
        
        console.log('Fetching hosts data...');
        const response = await getHosts();
        console.log('Hosts API response:', response);
        
        // Handle different response formats
        let hostsData = [];
        if (response && response.data) {
          if (Array.isArray(response.data)) {
            hostsData = response.data.map(host => ({
              ...host,
              name: host.name || host.hostname,
              address: host.ipAddress || host.IpAddress || host.ip_address || host.address || host.ip || 'Unknown',
              os: host.os || host.operating_system || 'Unknown',
              status: host.status || host.state || 'unknown'
            }));
          } else if (typeof response.data === 'object' && response.data.hosts) {
            hostsData = response.data.hosts.map(host => ({
              ...host,
              name: host.name || host.hostname,
              address: host.ipAddress || host.IpAddress || host.ip_address || host.address || host.ip || 'Unknown',
              os: host.os || host.operating_system || 'Unknown',
              status: host.status || host.state || 'unknown'
            }));
          }
        }
        
        // Verify we have valid data
        if (Array.isArray(hostsData)) {
          console.log('Processed hosts data:', hostsData);
          setHosts(hostsData);
          setFilteredHosts(hostsData);
        } else {
          console.error('Invalid hosts data format:', response.data);
          setError('Received invalid data format from server.');
        }
      } catch (err) {
        console.error('Failed to fetch hosts:', err);
        
        if (err.response && err.response.status === 401) {
          console.log('Authentication error in hosts component');
          if (typeof onAuthError === 'function') {
            onAuthError(err);
          }
        }
        
        setError('Failed to load hosts. Please try again later.');
      } finally {
        setIsLoading(false);
      }
    };

    fetchHosts();
  }, [onAuthError]);

  // Filter hosts when search value changes
  useEffect(() => {
    if (searchValue) {
      const filtered = hosts.filter(host => 
        host.name?.toLowerCase().includes(searchValue.toLowerCase()) ||
        host.address?.toLowerCase().includes(searchValue.toLowerCase()) ||
        host.os?.toLowerCase().includes(searchValue.toLowerCase())
      );
      setFilteredHosts(filtered);
      setPage(1);
    } else {
      setFilteredHosts(hosts);
    }
  }, [searchValue, hosts]);

  // Pagination
  const onSetPage = (_, newPage) => {
    setPage(newPage);
  };

  const onPerPageSelect = (_, newPerPage, newPage) => {
    setPerPage(newPerPage);
    setPage(newPage);
  };

  // Calculate pagination
  const paginatedHosts = filteredHosts.slice((page - 1) * perPage, page * perPage);

  // Handle view host details
  const handleViewHost = (hostname) => {
    navigate(`/hosts/${hostname}`);
  };

  // Handle checkbox selection
  const onSelectHost = (isSelected, hostName) => {
    if (isSelected) {
      setSelectedHosts([...selectedHosts, hostName]);
    } else {
      setSelectedHosts(selectedHosts.filter(name => name !== hostName));
    }
  };

  // Handle select all checkbox
  const onSelectAll = (isSelected) => {
    if (isSelected) {
      setSelectedHosts(paginatedHosts.map(host => host.name));
    } else {
      setSelectedHosts([]);
    }
  };

  // Handle force delete action for a single host
  const handleForceDelete = async () => {
    if (!hostToForceDelete) return;
    
    try {
      setIsDeleting(true);
      
      await forceDeleteHost(hostToForceDelete);
      
      // Refresh the hosts list
      const response = await getHosts();
      const updatedHosts = response.data;
      setHosts(updatedHosts);
      setFilteredHosts(updatedHosts);
      
      addAlert(`Successfully force deleted host ${hostToForceDelete}`, AlertVariant.success);
      setHostToForceDelete(null);
      setIsForceDeleteModalOpen(false);
      setIsDeleting(false);
    } catch (error) {
      console.error('Failed to force delete host:', error);
      addAlert('Failed to force delete host', AlertVariant.danger, error.message || 'Please try again later.');
      setIsDeleting(false);
    }
  };

  // Toggle action dropdown for a host
  const onToggleActionDropdown = (hostName, isOpen) => {
    setActionDropdownOpen({
      ...actionDropdownOpen,
      [hostName]: isOpen
    });
  };

  // Delete selected hosts
  const deleteSelectedHosts = async () => {
    try {
      const token = localStorage.getItem('token');
      const deletePromises = selectedHosts.map(hostName =>
        axios.delete(`/api/v1/admin/hosts/${hostName}`, {
          headers: { Authorization: token }
        })
      );

      await Promise.all(deletePromises);
      
      // Refresh the hosts list
      const response = await getHosts();
      const updatedHosts = response.data;
      setHosts(updatedHosts);
      setFilteredHosts(updatedHosts);
      setSelectedHosts([]);
      
      addAlert(`Successfully deleted ${selectedHosts.length} host(s)`, AlertVariant.success);
    } catch (error) {
      console.error('Failed to delete hosts:', error);
      addAlert('Failed to delete hosts', AlertVariant.danger, 'Please try again later.');
    }
    setIsDeleteModalOpen(false);
  };

  if (isLoading) {
    return (
      <PageSection>
        <Flex justifyContent={{ default: 'justifyContentCenter' }}>
          <FlexItem>
            <Spinner size="xl" />
          </FlexItem>
        </Flex>
      </PageSection>
    );
  }

  if (error) {
    return (
      <PageSection>
        <Card>
          <CardBody>
            <Flex justifyContent={{ default: 'justifyContentCenter' }}>
              <FlexItem>
                <div>
                  <Title headingLevel="h2">Error</Title>
                  <p style={{ fontSize: '16px', margin: '12px 0' }}>{error}</p>
                  <Button 
                    variant="primary" 
                    onClick={() => {
                      setIsLoading(true);
                      setError(null);
                      // Retry fetching the data
                      getHosts()
                        .then(response => {
                          if (response && response.data) {
                            setHosts(Array.isArray(response.data) ? response.data : []);
                            setFilteredHosts(Array.isArray(response.data) ? response.data : []);
                          }
                          setIsLoading(false);
                        })
                        .catch(() => {
                          setError('Failed to load hosts. Please try again later.');
                          setIsLoading(false);
                        });
                    }}
                  >
                    Retry
                  </Button>
                </div>
              </FlexItem>
            </Flex>
          </CardBody>
        </Card>
      </PageSection>
    );
  }

  return (
    <PageSection>
      <AlertGroup isToast>
        {alerts.map(({ key, variant, title, description }) => (
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
          >
            {description}
          </Alert>
        ))}
      </AlertGroup>

      <Card>
        <CardHeader>
          <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsMd' }}>
            <FlexItem>
              <Title headingLevel="h2" size="xl">Hosts</Title>
            </FlexItem>
            <FlexItem>
              <Toolbar>
                <ToolbarContent>
                  <ToolbarItem>
                    <SearchInput
                      placeholder="Filter hosts..."
                      value={searchValue}
                      onChange={(event, value) => setSearchValue(value)}
                      onClear={() => setSearchValue('')}
                    />
                  </ToolbarItem>
                  <ToolbarItem>
                    <Button
                      variant="danger"
                      isDisabled={selectedHosts.length === 0}
                      onClick={() => setIsDeleteModalOpen(true)}
                    >
                      Delete Selected ({selectedHosts.length})
                    </Button>
                  </ToolbarItem>
                  <ToolbarItem>
                    <Tooltip content="Add a new host to AWX and verify its connectivity">
                      <Button
                        variant="primary"
                        icon={<PlusCircleIcon />}
                        onClick={() => navigate('/hosts/awx/add')}
                      >
                        Add Host to AWX
                      </Button>
                    </Tooltip>
                  </ToolbarItem>
                </ToolbarContent>
              </Toolbar>
            </FlexItem>
          </Flex>
        </CardHeader>
        <CardBody>
          {filteredHosts.length === 0 ? (
            <Flex justifyContent={{ default: 'justifyContentCenter' }}>
              <FlexItem>
                <div>
                  <Title headingLevel="h3">No hosts found</Title>
                  {searchValue ? (
                    <p style={{ fontSize: '16px', margin: '12px 0' }}>
                      No hosts match your search criteria. Try adjusting your filter.
                    </p>
                  ) : (
                    <p style={{ fontSize: '16px', margin: '12px 0' }}>
                      No hosts are currently registered in the system.
                    </p>
                  )}
                </div>
              </FlexItem>
            </Flex>
          ) : (
            <>
              <Table aria-label="Hosts table">
                <Thead>
                  <Tr>
                    <Th>
                      <Checkbox
                        id="select-all"
                        isChecked={selectedHosts.length === paginatedHosts.length}
                        onChange={(_, isChecked) => onSelectAll(isChecked)}
                        aria-label="Select all hosts"
                      />
                    </Th>
                    <Th>Name</Th>
                    <Th>Address</Th>
                    <Th>OS</Th>
                    <Th>Status</Th>
                    <Th>Actions</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {paginatedHosts.map((host, index) => (
                    <Tr key={host.name || host.id || index}>
                      <Td>
                        <Checkbox
                          id={`select-host-${host.name}`}
                          isChecked={selectedHosts.includes(host.name)}
                          onChange={(_, isChecked) => onSelectHost(isChecked, host.name)}
                          aria-label={`Select host ${host.name}`}
                        />
                      </Td>
                      <Td>{host.name}</Td>
                      <Td>{host.address}</Td>
                      <Td>{host.os}</Td>
                      <Td>
                        {(() => {
                          const status = String(host.status).toLowerCase();
                          switch (status) {
                            case 'online':
                              return (
                                <>
                                  <span style={{
                                    display: 'inline-block',
                                    width: '12px',
                                    height: '12px',
                                    borderRadius: '50%',
                                    backgroundColor: '#3E8635',
                                    marginRight: '8px'
                                  }}></span>
                                  Online
                                </>
                              );
                            case 'offline':
                              return (
                                <>
                                  <span style={{
                                    display: 'inline-block',
                                    width: '12px',
                                    height: '12px',
                                    borderRadius: '50%',
                                    backgroundColor: '#C9190B',
                                    marginRight: '8px'
                                  }}></span>
                                  Offline
                                </>
                              );
                            case 'scheduled for deletion':
                              return (
                                <>
                                  <span style={{
                                    display: 'inline-block',
                                    width: '12px',
                                    height: '12px',
                                    borderRadius: '50%',
                                    backgroundColor: '#F0AB00',
                                    marginRight: '8px'
                                  }}></span>
                                  Scheduled for deletion
                                </>
                              );
                            default:
                              return (
                                <>
                                  <span style={{
                                    display: 'inline-block',
                                    width: '12px',
                                    height: '12px',
                                    borderRadius: '50%',
                                    backgroundColor: '#878787',
                                    marginRight: '8px'
                                  }}></span>
                                  Unknown
                                </>
                              );
                          }
                        })()}
                      </Td>
                      <Td>
                        <ActionsColumn
                          items={[
                            {
                              title: 'View',
                              onClick: () => handleViewHost(host.name)
                            },
                            {
                              title: 'Force Delete',
                              onClick: () => {
                                setHostToForceDelete(host.name);
                                setIsForceDeleteModalOpen(true);
                              },
                              tooltipContent: 'Immediately delete this host, bypassing all safety checks'
                            }
                          ]}
                        />
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
              
              <Pagination
                itemCount={filteredHosts.length}
                perPage={perPage}
                page={page}
                onSetPage={onSetPage}
                onPerPageSelect={onPerPageSelect}
                variant="bottom"
                isCompact
              />
            </>
          )}
        </CardBody>
      </Card>

      <DeleteConfirmationModal
        isOpen={isDeleteModalOpen}
        onClose={() => setIsDeleteModalOpen(false)}
        onDelete={deleteSelectedHosts}
        title={`Delete ${selectedHosts.length} selected host${selectedHosts.length !== 1 ? 's' : ''}?`}
        message={`Are you sure you want to delete ${selectedHosts.length} selected host${selectedHosts.length !== 1 ? 's' : ''}?`}
      />

      {/* Force Delete Confirmation Modal */}
      <ForceDeleteConfirmationModal
        isOpen={isForceDeleteModalOpen}
        onClose={() => {
          setIsForceDeleteModalOpen(false);
          setHostToForceDelete(null);
        }}
        onDelete={handleForceDelete}
        hostname={hostToForceDelete}
        isDeleting={isDeleting}
      />
      
      {/* AWX Host Add Modal */}
      <AwxHostAdd
        isOpen={isAwxHostAddModalOpen}
        onClose={() => setIsAwxHostAddModalOpen(false)}
        onHostAdded={(host) => {
          // Just show a success alert, don't refresh hosts list since we're not adding to dashboard
          if (host.awxOnly) {
            // For AWX-only hosts (not added to dashboard)
            addAlert(`Successfully added host "${host.hostName}" to AWX (not added to dashboard)`, AlertVariant.success);
          } else {
            // This branch shouldn't be reached anymore, but keeping for backward compatibility
            addAlert(`Successfully added host "${host.name}" to AWX`, AlertVariant.success);
            getHosts()
              .then(response => {
                if (response && response.data) {
                  setHosts(Array.isArray(response.data) ? response.data : []);
                  setFilteredHosts(Array.isArray(response.data) ? response.data : []);
                }
              })
              .catch(error => {
                console.error('Failed to refresh hosts list:', error);
              });
          }
        }}
      />
    </PageSection>
  );
};

export default Hosts;
