import React, { useState, useEffect } from 'react';
import {
  PageSection,
  Title,
  Card,
  CardBody,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Button,
  EmptyState,
  EmptyStateVariant,
  EmptyStateBody,
  Spinner,
  Icon,
  Modal,
  Alert,
  Label,
} from '@patternfly/react-core';
import {
  Table,
  Thead,
  Tr,
  Th,
  Tbody,
  Td
} from '@patternfly/react-table';
import { UsersIcon, TrashIcon, PlusCircleIcon } from '@patternfly/react-icons';
import axios from 'axios';
import { useNavigate } from 'react-router-dom';
import { useTheme } from '../ThemeContext.jsx';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';

const Users = () => {
  const { theme } = useTheme();
  const navigate = useNavigate();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [selectedUsers, setSelectedUsers] = useState([]);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [error, setError] = useState('');
  const [confirmationChecked, setConfirmationChecked] = useState(false);

  // Styles for dark mode
  const cardStyles = {
    backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
    color: theme === 'dark' ? '#ffffff' : '#151515',
    boxShadow: theme === 'dark' ? '0 4px 8px rgba(0,0,0,0.3)' : '0 2px 4px rgba(0,0,0,0.1)',
  };

  const titleStyles = {
    color: theme === 'dark' ? '#ffffff' : '#151515',
  };

  // Updated table style with background color
  const tableStyles = {
    color: theme === 'dark' ? '#ffffff' : '#151515',
    backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
  };

  // Specific style for table headers
  const tableHeaderStyles = {
    ...tableStyles,
    backgroundColor: theme === 'dark' ? '#202020' : '#f8f8f8',
    color: theme === 'dark' ? '#ffffff' : '#151515',
    fontWeight: 600,
  };

  // Table row hover style - for applying in onMouseEnter/onMouseLeave
  const tableRowHoverStyle = theme === 'dark' ? '#333333' : '#f5f5f5';

  const linkStyles = {
    color: theme === 'dark' ? '#73bcf7' : '#0066cc',
  };

  const labelStyles = {
    backgroundColor: theme === 'dark' ? '#1b1b1b' : '#f0f0f0',
    color: theme === 'dark' ? '#ffffff' : '#151515',
    padding: '4px 8px',
    borderRadius: '30px',
    display: 'inline-block',
    fontSize: '12px',
    margin: '2px',
  };

  const checkboxStyles = {
    accentColor: theme === 'dark' ? '#73bcf7' : '#0066cc',
    width: '16px',
    height: '16px',
  };

  const spinnerContainerStyles = {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    height: '200px',
    backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
    color: theme === 'dark' ? '#ffffff' : '#151515',
    borderRadius: '5px',
  };

  const emptyStateStyles = {
    backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
    color: theme === 'dark' ? '#ffffff' : '#151515',
    padding: '40px 20px',
    borderRadius: '5px',
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  // Reset confirmation checkbox when opening/closing modal
  useEffect(() => {
    if (!isDeleteModalOpen) {
      setConfirmationChecked(false);
    }
  }, [isDeleteModalOpen]);

  const fetchUsers = async () => {
    try {
      setLoading(true);
      const response = await axios.get('/api/v1/admin/users', {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      setUsers(response.data);
      setSelectedUsers([]);
    } catch (err) {
      setError('Failed to fetch users');
      console.error('Error fetching users:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSelect = (user) => {
    setSelectedUsers(prev => {
      const isSelected = prev.includes(user.username);
      if (isSelected) {
        return prev.filter(username => username !== user.username);
      } else {
        return [...prev, user.username];
      }
    });
  };

  const handleSelectAll = (isSelected) => {
    setSelectedUsers(isSelected ? users.map(user => user.username) : []);
  };

  const handleDeleteUsers = async () => {
    try {
      await Promise.all(selectedUsers.map(username => 
        axios.delete(`/api/v1/admin/users/${username}`, {
          headers: {
            Authorization: localStorage.getItem('token')
          }
        })
      ));
      
      setIsDeleteModalOpen(false);
      await fetchUsers();
    } catch (err) {
      setError('Failed to delete selected users');
    }
  };

  // onMouseLeave handler for table rows
  const handleMouseLeave = (e) => {
    e.currentTarget.style.backgroundColor = theme === 'dark' ? '#292929' : '#ffffff';
  };

  if (loading) {
    return (
      <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff', padding: '24px' }}>
        <div style={spinnerContainerStyles}>
          <Spinner size="xl" style={{ color: theme === 'dark' ? '#73bcf7' : '#0066cc' }} />
        </div>
      </PageSection>
    );
  }

  if (users.length === 0) {
    return (
      <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff', padding: '24px' }}>
        <div style={emptyStateStyles}>
          <EmptyState variant={EmptyStateVariant.full}>
            <Icon icon={UsersIcon} style={{ color: theme === 'dark' ? '#73bcf7' : '#0066cc' }} />
            <Title headingLevel="h4" size="lg" style={titleStyles}>
              No users found
            </Title>
            <EmptyStateBody style={{ color: theme === 'dark' ? '#d2d2d2' : '#4d5258' }}>
              No users are currently registered in the system.
            </EmptyStateBody>
            <ButtonWithCenteredIcon
              variant="primary"
              icon={<PlusCircleIcon />}
              onClick={() => navigate('/users/new')}
              style={{ marginTop: '24px' }}
            >
              Add User
            </ButtonWithCenteredIcon>
          </EmptyState>
        </div>
      </PageSection>
    );
  }

  return (
    <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff', padding: '24px' }}>
      <Title headingLevel="h1" size="lg" style={{ ...titleStyles, marginBottom: '1rem' }}>Users</Title>
      
      {error && (
        <Alert 
          variant="danger" 
          title={error} 
          style={{ 
            marginBottom: '1rem',
            backgroundColor: theme === 'dark' ? '#471c1e' : undefined,
            color: theme === 'dark' ? '#ffffff' : undefined,
            borderColor: theme === 'dark' ? '#c9190b' : undefined
          }} 
        />
      )}
      
      <Card style={cardStyles}>
        <CardBody>
          <Toolbar style={{ backgroundColor: theme === 'dark' ? '#292929' : '#ffffff', marginBottom: '16px' }}>
            <ToolbarContent>
              <ToolbarItem>
                <ButtonWithCenteredIcon 
                  variant="primary" 
                  icon={<PlusCircleIcon />}
                  onClick={() => navigate('/users/new')}
                >
                  Add User
                </ButtonWithCenteredIcon>
              </ToolbarItem>
              <ToolbarItem>
                <ButtonWithCenteredIcon 
                  variant="danger" 
                  icon={<TrashIcon />}
                  isDisabled={selectedUsers.length === 0}
                  onClick={() => setIsDeleteModalOpen(true)}
                >
                  Delete Selected ({selectedUsers.length})
                </ButtonWithCenteredIcon>
              </ToolbarItem>
              <ToolbarItem>
                <Button 
                  variant="secondary" 
                  onClick={fetchUsers}
                  style={{
                    backgroundColor: theme === 'dark' ? '#383838' : undefined,
                    color: theme === 'dark' ? '#ffffff' : undefined,
                  }}
                >
                  Refresh
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>

          <div style={{ 
            backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
            borderRadius: '5px',
            overflow: 'hidden',
            border: theme === 'dark' ? '1px solid #333' : '1px solid #d2d2d2',
          }}>
            <Table aria-label="Users table">
              <Thead>
                <Tr>
                  <Th style={tableHeaderStyles}>
                    <input
                      type="checkbox"
                      checked={selectedUsers.length === users.length && users.length > 0}
                      onChange={(e) => handleSelectAll(e.target.checked)}
                      style={checkboxStyles}
                    />
                  </Th>
                  <Th style={tableHeaderStyles}>Username</Th>
                  <Th style={tableHeaderStyles}>Email</Th>
                  <Th style={tableHeaderStyles}>Role</Th>
                  <Th style={tableHeaderStyles}>Groups</Th>
                  <Th style={tableHeaderStyles}>Inventories</Th>
                </Tr>
              </Thead>
              <Tbody>
                {users.map(user => (
                  <Tr 
                    key={user.username}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.backgroundColor = tableRowHoverStyle;
                    }}
                    onMouseLeave={handleMouseLeave}
                  >
                    <Td style={tableStyles}>
                      <input
                        type="checkbox"
                        checked={selectedUsers.includes(user.username)}
                        onChange={() => handleSelect(user)}
                        style={checkboxStyles}
                      />
                    </Td>
                    <Td style={tableStyles}>
                      <Button 
                        variant="link" 
                        onClick={() => navigate(`/users/${user.username}`)}
                        style={linkStyles}
                      >
                        {user.username}
                      </Button>
                    </Td>
                    <Td style={tableStyles}>{user.email}</Td>
                    <Td style={tableStyles}>
                      <span style={{
                        ...labelStyles,
                        backgroundColor: user.role === 'admin' ? '#0066cc' : '#2da44e',
                        color: '#ffffff',
                        fontWeight: '500',
                        boxShadow: theme === 'dark' ? '0 2px 4px rgba(0,0,0,0.3)' : 'none',
                      }}>
                        {user.role}
                      </span>
                    </Td>
                    <Td style={tableStyles}>
                      {user.groups === 'nil' ? (
                        <span style={{ color: theme === 'dark' ? '#999' : '#666' }}>None</span>
                      ) : (
                        user.groups.split(',').map(group => (
                          <span key={group} style={{
                            ...labelStyles,
                            boxShadow: theme === 'dark' ? '0 2px 4px rgba(0,0,0,0.2)' : 'none',
                          }}>
                            {group.trim()}
                          </span>
                        ))
                      )}
                    </Td>
                    <Td style={tableStyles}>
                      {user.inventories === 'nil' ? (
                        <span style={{ color: theme === 'dark' ? '#999' : '#666' }}>None</span>
                      ) : (
                        user.inventories.split(',').map(inventory => (
                          <span key={inventory} style={{
                            ...labelStyles,
                            backgroundColor: theme === 'dark' ? '#492365' : '#e7d9f3',
                            color: theme === 'dark' ? '#ffffff' : '#492365',
                            boxShadow: theme === 'dark' ? '0 2px 4px rgba(0,0,0,0.2)' : 'none',
                          }}>
                            {inventory.trim()}
                          </span>
                        ))
                      )}
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </div>
        </CardBody>
      </Card>

      <Modal
        variant="large"
        title="Delete Users"
        isOpen={isDeleteModalOpen}
        onClose={() => setIsDeleteModalOpen(false)}
        style={{ 
          maxWidth: '700px',
          padding: '20px',
          backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
          color: theme === 'dark' ? '#ffffff' : '#151515'
        }}
        header={
          <Title 
            headingLevel="h1" 
            size="xl"
            style={{ 
              color: theme === 'dark' ? '#ffffff' : '#151515',
              padding: '16px 24px',
              borderBottom: theme === 'dark' ? '1px solid #333' : '1px solid #d2d2d2'
            }}
          >
            Delete Users
          </Title>
        }
      >
        <div style={{ 
          padding: '30px 24px',
          backgroundColor: theme === 'dark' ? '#292929' : '#ffffff',
         }}>
          <Alert 
            variant="warning" 
            isInline 
            title="This action cannot be undone" 
            style={{ 
              marginBottom: '30px',
              backgroundColor: theme === 'dark' ? '#332a04' : undefined,
              color: theme === 'dark' ? '#ffffff' : undefined,
              borderColor: theme === 'dark' ? '#f0ab00' : undefined,
              boxShadow: theme === 'dark' ? '0 2px 4px rgba(0,0,0,0.2)' : 'none',
            }}
            titleHeadingLevel="h4"
          >
            <span style={{ color: theme === 'dark' ? '#f0f0f0' : undefined }}>
              This is a destructive action that permanently removes users from the system.
            </span>
          </Alert>
          <p style={{ 
            fontSize: '16px', 
            marginBottom: '20px', 
            fontWeight: '500',
            color: theme === 'dark' ? '#ffffff' : '#151515'
          }}>
            Are you sure you want to delete the following {selectedUsers.length} selected user(s)?
          </p>
          {selectedUsers.length > 0 && (
            <div style={{ 
              margin: '20px 0 30px 0', 
              padding: '16px 24px', 
              backgroundColor: theme === 'dark' ? '#202020' : '#f5f5f5',
              borderRadius: '4px',
              maxHeight: '180px',
              overflow: 'auto',
              border: `1px solid ${theme === 'dark' ? '#333' : '#ddd'}`,
              boxShadow: theme === 'dark' ? 'inset 0 2px 4px rgba(0,0,0,0.2)' : 'none',
            }}>
              <ul style={{ margin: 0, paddingLeft: '20px' }}>
                {selectedUsers.map(username => (
                  <li key={username} style={{ 
                    margin: '10px 0', 
                    fontSize: '16px',
                    color: theme === 'dark' ? '#ffffff' : '#151515'
                  }}>
                    {username}
                  </li>
                ))}
              </ul>
            </div>
          )}
          <div style={{ 
            display: 'flex', 
            alignItems: 'center',
            marginTop: '24px',
            padding: '16px',
            backgroundColor: theme === 'dark' ? '#202020' : '#f9f9f9',
            borderRadius: '4px',
            border: `1px solid ${theme === 'dark' ? '#333' : '#ddd'}`,
            boxShadow: theme === 'dark' ? 'inset 0 1px 2px rgba(0,0,0,0.1)' : 'none',
          }}>
            <input 
              type="checkbox" 
              id="confirmDelete"
              checked={confirmationChecked}
              onChange={(e) => setConfirmationChecked(e.target.checked)}
              style={{ 
                marginRight: '12px', 
                width: '18px', 
                height: '18px',
                accentColor: theme === 'dark' ? '#73bcf7' : '#0066cc', 
              }}
            />
            <label htmlFor="confirmDelete" style={{ 
              fontSize: '16px', 
              fontWeight: '500',
              color: theme === 'dark' ? '#ffffff' : '#151515'
            }}>
              I understand that this action cannot be undone
            </label>
          </div>
          
          {/* Action buttons */}
          <div style={{ 
            display: 'flex',
            justifyContent: 'flex-end',
            marginTop: '40px',
            gap: '16px'
          }}>
            <Button 
              variant="secondary" 
              onClick={() => setIsDeleteModalOpen(false)} 
              style={{ 
                minWidth: '140px', 
                padding: '10px 20px', 
                fontSize: '16px',
                backgroundColor: theme === 'dark' ? '#383838' : undefined,
                color: theme === 'dark' ? '#ffffff' : undefined, 
              }}
            >
              Cancel
            </Button>
            <Button 
              variant="danger" 
              onClick={handleDeleteUsers}
              isDisabled={!confirmationChecked}
              style={{ minWidth: '140px', padding: '10px 20px', fontSize: '16px' }}
            >
              Delete Users
            </Button>
          </div>
        </div>
      </Modal>
    </PageSection>
  );
};

export default Users;