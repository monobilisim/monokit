import React, { useState, useEffect } from 'react';
import { useTheme } from '../ThemeContext.jsx';
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Flex,
  FlexItem,
  PageSection,
  Pagination,
  Spinner,
  TextInput,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  FormGroup,
  EmptyState,
  EmptyStateBody,
  EmptyStateVariant
} from '@patternfly/react-core';
import { PlusCircleIcon, UsersIcon, TrashIcon } from '@patternfly/react-icons';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useNavigate } from 'react-router-dom';
import { getGroups, createGroup, deleteGroup } from '../utils/api';
import ModalForm from '../components/ModalForm';
import DeleteConfirmationModal from '../components/DeleteConfirmationModal';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';
import CenteredIcon from '../components/CenteredIcon';

const Groups = ({ onAuthError }) => {
  const { theme } = useTheme();
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [groups, setGroups] = useState([]);
  const [filteredGroups, setFilteredGroups] = useState([]);
  
  // Selected items for deletion
  const [selectedItems, setSelectedItems] = useState([]);
  
  // State for modals
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  
  // Pagination state
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  
  // Calculate paginated groups
  const paginatedGroups = filteredGroups.slice((page - 1) * perPage, page * perPage);
  
  // Handle pagination
  const onSetPage = (_, newPage) => {
    setPage(newPage);
  };
  
  const onPerPageSelect = (_, newPerPage) => {
    setPerPage(newPerPage);
    setPage(1);
  };
  
  // Modal handlers
  const openCreateModal = () => {
    setNewGroupName('');
    setIsCreateModalOpen(true);
  };
  
  const closeCreateModal = () => {
    setIsCreateModalOpen(false);
    setNewGroupName('');
  };
  
  const openDeleteModal = () => {
    if (selectedItems.length > 0) {
      setIsDeleteModalOpen(true);
    } else {
      alert('Please select at least one group to delete.');
    }
  };
  
  const closeDeleteModal = () => {
    setIsDeleteModalOpen(false);
  };
  
  // Fetch groups data
  const fetchGroups = async () => {
    setIsLoading(true);
    try {
      const response = await getGroups();
      
      // Map the response data to our expected format
      const fetchedGroups = response.data.map(group => ({
        id: group.id,
        name: group.name,
        members: group.users ? group.users.length : 0,
        created_at: new Date(group.createdAt).toLocaleString()
      }));
      
      setGroups(fetchedGroups);
      setFilteredGroups(fetchedGroups);
      setError(null);
    } catch (error) {
      console.error('Error fetching groups:', error);
      setError('Failed to load groups. Please try again later.');
      
      if (error.response && error.response.status === 401) {
        onAuthError(error);
      }
    } finally {
      setIsLoading(false);
    }
  };
  
  // Handle form submission
  const handleCreateGroup = async () => {
    if (!newGroupName.trim()) return;
    
    setIsSubmitting(true);
    try {
      // Debug log the authentication status
      console.log('Attempting to create group with authorization:', localStorage.getItem('token'));
      
      // Make API call to create the group
      const response = await createGroup(newGroupName.trim());
      
      // If successful, refresh the groups list
      fetchGroups();
      
      // Show success message (if you have a notification system)
      console.log('Group created successfully:', response.data);
      
      // Close modal and reset form
      setNewGroupName('');
      closeCreateModal();
    } catch (error) {
      console.error('Error creating group:', error);
      
      // Check for specific error types
      if (error.response) {
        console.error('Error response:', error.response.data);
        console.error('Error status:', error.response.status);
        console.error('Error headers:', error.response.headers);
        
        if (error.response.status === 401) {
          // Handle authentication error
          alert('Authentication failed. You may need to log in again.');
          if (typeof onAuthError === 'function') {
            onAuthError(error);
          }
        } else if (error.response.status === 409) {
          // Handle conflict (group already exists)
          alert('A group with this name already exists.');
        } else {
          // Handle other API errors
          alert(`Failed to create group: ${error.response.data?.error || 'Unknown error'}`);
        }
      } else {
        // Handle network errors
        alert('Network error. Please check your connection and try again.');
      }
    } finally {
      setIsSubmitting(false);
    }
  };
  
  // Handle deletion of selected groups
  const handleDeleteSelected = async () => {
    if (selectedItems.length === 0) return;
    
    setIsDeleting(true);
    try {
      // Find the groups from the selected IDs/names
      const groupsToDelete = groups.filter(group => selectedItems.includes(group.id || group.name));
      
      // Delete each group one by one
      for (const group of groupsToDelete) {
        await deleteGroup(group.name);
        console.log(`Deleted group: ${group.name}`);
      }
      
      // Refresh the groups list
      await fetchGroups();
      
      // Show success message
      console.log(`Deleted ${selectedItems.length} groups`);
      
      // Reset selected items and close modal
      setSelectedItems([]);
      closeDeleteModal();
    } catch (error) {
      console.error('Error deleting groups:', error);
      
      if (error.response && error.response.status === 401) {
        onAuthError(error);
        alert('Authentication failed. You may need to log in again.');
      } else {
        alert('Failed to delete one or more groups. Please try again.');
      }
    } finally {
      setIsDeleting(false);
    }
  };
  
  // Handle selection
  const onSelect = (id) => {
    console.log('Selecting/deselecting item with ID:', id);
    
    // Create a new array to ensure state update triggers correctly
    if (selectedItems.includes(id)) {
      // Remove the ID if already selected
      setSelectedItems(selectedItems.filter(itemId => itemId !== id));
    } else {
      // Add the ID if not already selected
      setSelectedItems([...selectedItems, id]);
    }
    
    // Debug: Log the updated selection after a brief delay to ensure state has updated
    setTimeout(() => {
      console.log('Current selected items after update:', selectedItems);
    }, 100);
  };

  // Handle select all
  const onSelectAll = (isSelected) => {
    if (isSelected) {
      const allIds = paginatedGroups.map(group => group.id || group.name);
      setSelectedItems(allIds);
    } else {
      setSelectedItems([]);
    }
  };
  
  // Fetch data on component mount
  useEffect(() => {
    fetchGroups();
  }, []);
  
  if (isLoading) {
    return (
      <PageSection>
        <Flex justifyContent={{ default: 'justifyContentCenter' }}>
          <FlexItem>
            <Spinner size="lg" />
            <p>Loading groups...</p>
          </FlexItem>
        </Flex>
      </PageSection>
    );
  }

  if (error) {
    return (
      <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
        <Card>
          <CardBody>
            <Flex justifyContent={{ default: 'justifyContentCenter' }}>
              <FlexItem>
                <div>
                  <Title headingLevel="h3">Error loading groups</Title>
                  <p style={{ color: 'red' }}>{error}</p>
                  <Button variant="secondary" onClick={fetchGroups}>Try Again</Button>
                </div>
              </FlexItem>
            </Flex>
          </CardBody>
        </Card>
      </PageSection>
    );
  }

  return (
    <PageSection style={{ backgroundColor: theme === 'dark' ? '#212427' : '#ffffff' }}>
      <Card>
        <CardHeader>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Title headingLevel="h1">Groups</Title>
              </ToolbarItem>
              <ToolbarItem align={{ default: 'alignRight' }}>
                <ButtonWithCenteredIcon 
                  variant="primary" 
                  icon={<PlusCircleIcon />} 
                  onClick={openCreateModal}
                >
                  Create Group
                </ButtonWithCenteredIcon>
              </ToolbarItem>
              {selectedItems.length > 0 && (
                <ToolbarItem>
                  <ButtonWithCenteredIcon 
                    variant="danger" 
                    icon={<TrashIcon />}
                    onClick={openDeleteModal}
                  >
                    Delete Selected ({selectedItems.length})
                  </ButtonWithCenteredIcon>
                </ToolbarItem>
              )}
            </ToolbarContent>
          </Toolbar>
        </CardHeader>
        <CardBody>
          {filteredGroups.length === 0 ? (
            <EmptyState variant={EmptyStateVariant.large}>
              <div style={{ marginBottom: '16px' }}>
                <UsersIcon size="xl" />
              </div>
              <Title headingLevel="h3" size="lg">
                No Groups Found
              </Title>
              <EmptyStateBody>
                No groups have been created yet. Get started by creating your first group.
              </EmptyStateBody>
              <ButtonWithCenteredIcon 
                variant="primary" 
                onClick={openCreateModal}
                icon={<PlusCircleIcon />}
                style={{ marginTop: '16px' }}
              >
                Create Group
              </ButtonWithCenteredIcon>
            </EmptyState>
          ) : (
            <>
              <Table aria-label="Groups table">
                <Thead>
                  <Tr>
                    <Th screenReaderText="Select all rows">
                      <input
                        type="checkbox"
                        onChange={(e) => onSelectAll(e.target.checked)}
                        checked={selectedItems.length === paginatedGroups.length && paginatedGroups.length > 0}
                        aria-label="Select all groups"
                        style={{ width: '20px', height: '20px' }}
                      />
                    </Th>
                    <Th>Name</Th>
                    <Th>Members</Th>
                    <Th>Created At</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {paginatedGroups.map((group) => {
                    // Use name as ID if id is undefined
                    const groupId = group.id || group.name;
                    const isChecked = selectedItems.includes(groupId);
                    console.log(`Rendering row for ${group.name}, ID: ${groupId}, checked: ${isChecked}`);
                    
                    return (
                      <Tr key={groupId}>
                        <Td>
                          <input 
                            type="checkbox" 
                            checked={isChecked}
                            onChange={() => onSelect(groupId)}
                            aria-label={`Select ${group.name}`}
                            style={{ width: '20px', height: '20px' }}
                          />
                        </Td>
                        <Td>{group.name}</Td>
                        <Td>{group.members || 0}</Td>
                        <Td>{group.created_at || 'N/A'}</Td>
                      </Tr>
                    );
                  })}
                </Tbody>
              </Table>
              
              <Pagination
                itemCount={filteredGroups.length}
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
      
      {/* Create Group Modal */}
      <ModalForm
        title="Create Group"
        isOpen={isCreateModalOpen}
        onClose={closeCreateModal}
        onSubmit={handleCreateGroup}
        submitLabel="Create Group"
        isSubmitting={isSubmitting}
        width="medium"
      >
        <FormGroup 
          label="Name" 
          isRequired 
          fieldId="group-name"
        >
          <input
            type="text"
            id="group-name"
            name="name"
            value={newGroupName}
            onChange={(e) => setNewGroupName(e.target.value)}
            required
            className="pf-c-form-control"
            placeholder="Enter group name"
            style={{ width: '100%' }}
          />
        </FormGroup>
      </ModalForm>
      
      {/* Delete Confirmation Modal */}
      <DeleteConfirmationModal
        isOpen={isDeleteModalOpen}
        onClose={closeDeleteModal}
        onDelete={handleDeleteSelected}
        title="Delete Selected Groups"
        message={`Are you sure you want to delete ${selectedItems.length} selected ${selectedItems.length === 1 ? 'group' : 'groups'}? This action cannot be undone.`}
        isDeleting={isDeleting}
      />
    </PageSection>
  );
};

export default Groups;