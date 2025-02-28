import React, { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  PageSection,
  Title,
  Card,
  CardBody,
  Form,
  FormGroup,
  TextInput,
  ActionGroup,
  Button,
  Alert,
  Select,
  SelectOption,
  Stack,
  StackItem,
  Breadcrumb,
  BreadcrumbItem,
  MenuToggle,
} from '@patternfly/react-core';
import axios from 'axios';
import { useTheme } from '../ThemeContext.jsx';

const UserForm = () => {
  const navigate = useNavigate();
  const { username: editUsername } = useParams();
  const isEditing = !!editUsername;
  const { theme } = useTheme();

  const [formData, setFormData] = useState({
    username: '',
    email: '',
    password: '',
    role: 'user',
    groups: '',
    inventory: '',
  });
  const [error, setError] = useState('');
  const [isRoleOpen, setIsRoleOpen] = useState(false);
  const [availableGroups, setAvailableGroups] = useState([]);
  const [availableInventories, setAvailableInventories] = useState([]);
  const [isGroupsOpen, setIsGroupsOpen] = useState(false);
  const [isInventoriesOpen, setIsInventoriesOpen] = useState(false);

  useEffect(() => {
    if (isEditing) {
      fetchUserData();
    }
    fetchGroups();
    fetchInventories();
  }, [isEditing]);

  const fetchUserData = async () => {
    try {
      const response = await axios.get(`/api/v1/admin/users/${editUsername}`, {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      const userData = response.data;
      setFormData({
        username: userData.username,
        email: userData.email,
        password: '',
        role: userData.role,
        groups: userData.groups === 'nil' ? '' : userData.groups,
        inventory: userData.inventories === 'nil' ? '' : userData.inventories,
      });
    } catch (err) {
      setError('Failed to fetch user data');
    }
  };

  const fetchGroups = async () => {
    try {
      const response = await axios.get('/api/v1/admin/groups', {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      const groups = response.data.map(group => group.name || group);
      console.log('Fetched groups:', groups);
      setAvailableGroups(groups);
    } catch (err) {
      console.error('Failed to fetch groups:', err);
      setError('Failed to fetch available groups');
    }
  };

  const fetchInventories = async () => {
    try {
      const response = await axios.get('/api/v1/inventory', {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      const inventories = response.data.map(inv => inv.name || inv);
      console.log('Fetched inventories:', inventories);
      setAvailableInventories(inventories);
    } catch (err) {
      console.error('Failed to fetch inventories:', err);
      setError('Failed to fetch available inventories');
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');

    // Clean up and format the groups and inventory
    const groups = formData.groups ? formData.groups.split(',').filter(g => g.trim() !== '').join(',') : 'nil';
    const inventory = formData.inventory ? formData.inventory.split(',').filter(i => i.trim() !== '')[0] : 'nil';

    const submitData = {
      ...formData,
      groups,
      inventory
    };

    console.log('Submitting data:', submitData);

    try {
      if (isEditing) {
        await axios.put(`/api/v1/admin/users/${editUsername}`, submitData, {
          headers: {
            Authorization: localStorage.getItem('token')
          }
        });
      } else {
        await axios.post('/api/v1/auth/register', submitData, {
          headers: {
            Authorization: localStorage.getItem('token')
          }
        });
      }
      navigate('/users');
    } catch (err) {
      console.error('API Error:', err);
      setError(err.response?.data?.error || 'Failed to save user');
    }
  };

  console.log('Form Data:', formData);
  console.log('Available Groups:', availableGroups);
  console.log('Available Inventories:', availableInventories);

  return (
    <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
      <Stack hasGutter>
        <StackItem>
          <Breadcrumb>
            <BreadcrumbItem onClick={() => navigate('/users')}>Users</BreadcrumbItem>
            <BreadcrumbItem isActive>{isEditing ? 'Edit User' : 'New User'}</BreadcrumbItem>
          </Breadcrumb>
        </StackItem>

        <StackItem>
          <Title headingLevel="h1" size="xl">
            {isEditing ? `Edit User: ${editUsername}` : 'Create New User'}
          </Title>
        </StackItem>

        <StackItem>
          <Card>
            <CardBody>
              {error && (
                <Alert variant="danger" title={error} style={{ marginBottom: '1rem' }} />
              )}
              <Form onSubmit={handleSubmit} style={{ maxWidth: '600px' }}>
                <FormGroup label="Username" isRequired fieldId="username">
                  <TextInput
                    id="username"
                    value={formData.username}
                    onChange={(_, value) => setFormData(prev => ({ ...prev, username: value }))}
                    isDisabled={isEditing}
                  />
                </FormGroup>
                <FormGroup label="Email" isRequired fieldId="email">
                  <TextInput
                    id="email"
                    type="email"
                    value={formData.email}
                    onChange={(_, value) => setFormData(prev => ({ ...prev, email: value }))}
                  />
                </FormGroup>
                <FormGroup label="Password" isRequired={!isEditing} fieldId="password">
                  <TextInput
                    id="password"
                    type="password"
                    value={formData.password}
                    onChange={(_, value) => setFormData(prev => ({ ...prev, password: value }))}
                    placeholder={isEditing ? "Leave blank to keep current password" : ""}
                  />
                </FormGroup>
                <FormGroup label="Role" isRequired fieldId="role">
                  <Select
                    id="role"
                    isOpen={isRoleOpen}
                    selected={formData.role}
                    onToggle={setIsRoleOpen}
                    onSelect={(_, value) => {
                      setFormData(prev => ({ ...prev, role: value }));
                      setIsRoleOpen(false);
                    }}
                    toggle={(toggleRef) => (
                      <MenuToggle
                        ref={toggleRef}
                        onClick={() => setIsRoleOpen(!isRoleOpen)}
                        isExpanded={isRoleOpen}
                      >
                        {formData.role}
                      </MenuToggle>
                    )}
                  >
                    <SelectOption value="user">User</SelectOption>
                    <SelectOption value="admin">Admin</SelectOption>
                  </Select>
                </FormGroup>
                <FormGroup label="Groups" fieldId="groups">
                  <Select
                    id="groups"
                    isOpen={isGroupsOpen}
                    selected={formData.groups ? formData.groups.split(',').filter(g => g.trim() !== '') : []}
                    onToggle={() => setIsGroupsOpen(!isGroupsOpen)}
                    onSelect={(_, value) => {
                      const currentGroups = formData.groups ? formData.groups.split(',').filter(g => g.trim() !== '') : [];
                      let newGroups;
                      if (currentGroups.includes(value)) {
                        newGroups = currentGroups.filter(g => g !== value);
                      } else {
                        newGroups = [...currentGroups, value];
                      }
                      const newGroupsStr = newGroups.length > 0 ? newGroups.join(',') : '';
                      console.log('Setting groups:', newGroupsStr);
                      setFormData(prev => ({ ...prev, groups: newGroupsStr }));
                    }}
                    isMulti
                    hasInlineFilter
                    placeholderText="Select Groups"
                    toggle={(toggleRef) => (
                      <MenuToggle
                        ref={toggleRef}
                        onClick={() => setIsGroupsOpen(!isGroupsOpen)}
                        isExpanded={isGroupsOpen}
                      >
                        {formData.groups ? `${formData.groups.split(',').filter(g => g.trim() !== '').length} groups selected` : 'Select Groups'}
                      </MenuToggle>
                    )}
                  >
                    {availableGroups.length > 0 ? (
                      availableGroups.map(group => (
                        <SelectOption 
                          key={group} 
                          value={group}
                          isSelected={formData.groups ? formData.groups.split(',').map(g => g.trim()).includes(group) : false}
                        >
                          {group}
                        </SelectOption>
                      ))
                    ) : (
                      <SelectOption isDisabled value="no-groups">
                        No groups available
                      </SelectOption>
                    )}
                  </Select>
                </FormGroup>
                <FormGroup label="Inventory" fieldId="inventory">
                  <Select
                    id="inventory"
                    isOpen={isInventoriesOpen}
                    selected={formData.inventory ? formData.inventory.split(',').filter(i => i.trim() !== '') : []}
                    onToggle={() => setIsInventoriesOpen(!isInventoriesOpen)}
                    onSelect={(_, value) => {
                      const newValue = value;
                      console.log('Setting inventory:', newValue);
                      setFormData(prev => ({ ...prev, inventory: newValue }));
                      setIsInventoriesOpen(false);
                    }}
                    placeholderText="Select Inventory"
                    toggle={(toggleRef) => (
                      <MenuToggle
                        ref={toggleRef}
                        onClick={() => setIsInventoriesOpen(!isInventoriesOpen)}
                        isExpanded={isInventoriesOpen}
                      >
                        {formData.inventory || 'Select Inventory'}
                      </MenuToggle>
                    )}
                  >
                    {availableInventories.length > 0 ? (
                      availableInventories.map(inv => (
                        <SelectOption 
                          key={inv} 
                          value={inv}
                          isSelected={formData.inventory === inv}
                        >
                          {inv}
                        </SelectOption>
                      ))
                    ) : (
                      <SelectOption isDisabled value="no-inventories">
                        No inventories available
                      </SelectOption>
                    )}
                  </Select>
                </FormGroup>
                <ActionGroup>
                  <Button variant="primary" type="submit">
                    {isEditing ? 'Save Changes' : 'Create User'}
                  </Button>
                  <Button variant="link" onClick={() => navigate('/users')}>
                    Cancel
                  </Button>
                </ActionGroup>
              </Form>
            </CardBody>
          </Card>
        </StackItem>
      </Stack>
    </PageSection>
  );
};

export default UserForm; 