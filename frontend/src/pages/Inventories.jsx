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
  Spinner,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  FormGroup,
  EmptyState,
  EmptyStateBody,
  EmptyStateVariant
} from '@patternfly/react-core';
import { PlusCircleIcon, CubesIcon, TrashIcon } from '@patternfly/react-icons';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { getInventories, createInventory, deleteInventory } from '../utils/api';
import ModalForm from '../components/ModalForm';
import DeleteConfirmationModal from '../components/DeleteConfirmationModal';
import ButtonWithCenteredIcon from '../components/ButtonWithCenteredIcon';
import CenteredIcon from '../components/CenteredIcon';

const Inventories = ({ onAuthError }) => {
  const [inventories, setInventories] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);

  // Selected items for deletion
  const [selectedItems, setSelectedItems] = useState([]);
  
  // Modal states
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [newInventoryName, setNewInventoryName] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  // Fetch inventories data
  const fetchInventories = async () => {
    setIsLoading(true);
    try {
      const response = await getInventories();
      
      // Map the response data to our expected format with guaranteed unique IDs
      const fetchedInventories = response.data.map((inventory, index) => ({
        // Ensure each inventory has a truly unique ID
        id: inventory.name || `inventory-${index}-${Date.now()}`,
        name: inventory.name,
        hosts: Array.isArray(inventory.hosts) ? inventory.hosts.length : 0,
        created_at: inventory.createdAt 
          ? new Date(inventory.createdAt).toLocaleString() 
          : 'N/A'
      }));
      
      console.log('Fetched inventories with unique IDs:', fetchedInventories);
      setInventories(fetchedInventories);
      setError(null);
    } catch (error) {
      console.error('Error fetching inventories:', error);
      setError('Failed to load inventories. Please try again later.');
      
      if (error.response && error.response.status === 401) {
        onAuthError(error);
      }
    } finally {
      setIsLoading(false);
    }
  };
  
  // Fetch data on component mount
  useEffect(() => {
    fetchInventories();
  }, []);

  // Pagination
  const onSetPage = (_, newPage) => {
    setPage(newPage);
  };

  const onPerPageSelect = (_, newPerPage) => {
    setPerPage(newPerPage);
    setPage(1);
  };

  // Calculate pagination
  const paginatedInventories = inventories.slice((page - 1) * perPage, page * perPage);

  // Handle modal open/close
  const openCreateModal = () => {
    setNewInventoryName('');
    setIsCreateModalOpen(true);
  };

  const closeCreateModal = () => {
    setIsCreateModalOpen(false);
  };

  const openDeleteModal = () => {
    if (selectedItems.length > 0) {
      setIsDeleteModalOpen(true);
    } else {
      alert('Please select at least one item to delete.');
    }
  };

  const closeDeleteModal = () => {
    setIsDeleteModalOpen(false);
  };

  // Handle creation of new inventory
  const handleCreateInventory = async (e) => {
    // Add a check to handle undefined event
    if (e && e.preventDefault) {
      e.preventDefault();
    }
    
    // Proceed with inventory creation
    if (newInventoryName) {
      setIsSubmitting(true);
      try {
        await createInventory(newInventoryName);
        await fetchInventories();
        closeCreateModal();
        setNewInventoryName('');
      } catch (error) {
        console.error('Error creating inventory:', error);
        
        if (error.response && error.response.status === 401) {
          onAuthError(error);
        } else {
          alert('Failed to create inventory. Please try again.');
        }
      } finally {
        setIsSubmitting(false);
      }
    }
  };

  // Handle deletion of selected items
  const handleDeleteSelected = async () => {
    if (selectedItems.length === 0) return;
    
    setIsDeleting(true);
    try {
      // Find the inventory names from the selected IDs
      const inventoriesToDelete = inventories.filter(inventory => 
        selectedItems.includes(inventory.id)
      );
      
      console.log('Deleting these inventories:', inventoriesToDelete);
      
      // Delete each inventory one by one
      for (const inventory of inventoriesToDelete) {
        await deleteInventory(inventory.name);
        console.log(`Deleted inventory: ${inventory.name}`);
      }
      
      // Refresh the inventories list
      await fetchInventories();
      
      // Show success message
      console.log(`Deleted ${selectedItems.length} inventories`);
      
      // Reset selected items and close modal
      setSelectedItems([]);
      closeDeleteModal();
    } catch (error) {
      console.error('Error deleting inventories:', error);
      
      if (error.response && error.response.status === 401) {
        onAuthError(error);
        alert('Authentication failed. You may need to log in again.');
      } else {
        alert('Failed to delete one or more inventories. Please try again.');
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
                <div style={{ textAlign: 'center', marginTop: '20px' }}>
                  <CenteredIcon icon={<CubesIcon size="xl" />} style={{ marginBottom: '16px' }} />
                  <Title headingLevel="h3" size="lg">
                    Error Loading Inventories
                  </Title>
                  <p>{error}</p>
                  <Button
                    variant="primary"
                    onClick={fetchInventories}
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
      <Card>
        <CardHeader>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Title headingLevel="h1">Inventories</Title>
              </ToolbarItem>
              <ToolbarItem align={{ default: 'alignRight' }}>
                <ButtonWithCenteredIcon 
                  variant="primary" 
                  icon={<PlusCircleIcon />}
                  onClick={openCreateModal}
                >
                  Create Inventory
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
          {inventories.length === 0 ? (
            <EmptyState variant={EmptyStateVariant.large}>
              <div style={{ marginBottom: '16px' }}>
                <CubesIcon size="xl" />
              </div>
              <Title headingLevel="h3" size="lg">
                No Inventories Found
              </Title>
              <EmptyStateBody>
                No inventories have been created yet. Get started by creating your first inventory.
              </EmptyStateBody>
              <ButtonWithCenteredIcon 
                variant="primary" 
                onClick={openCreateModal}
                icon={<PlusCircleIcon />}
                style={{ marginTop: '16px' }}
              >
                Create Inventory
              </ButtonWithCenteredIcon>
            </EmptyState>
          ) : (
            <>
              <Table aria-label="Inventories table">
                <Thead>
                  <Tr>
                    <Th></Th>
                    <Th>Name</Th>
                    <Th>Hosts</Th>
                    <Th>Created At</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {paginatedInventories.map((inventory) => {
                    const isChecked = selectedItems.includes(inventory.id);
                    console.log(`Rendering row for ${inventory.name}, ID: ${inventory.id}, checked: ${isChecked}`);
                    
                    return (
                      <Tr key={inventory.id}>
                        <Td>
                          <input 
                            type="checkbox" 
                            checked={isChecked}
                            onChange={() => onSelect(inventory.id)}
                            aria-label={`Select ${inventory.name}`}
                            style={{ width: '20px', height: '20px' }}
                          />
                        </Td>
                        <Td>{inventory.name}</Td>
                        <Td>{inventory.hosts}</Td>
                        <Td>{inventory.created_at}</Td>
                      </Tr>
                    );
                  })}
                </Tbody>
              </Table>
              
              <Pagination
                itemCount={inventories.length}
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
      
      {/* Create Inventory Modal */}
      <ModalForm
        title="Create Inventory"
        isOpen={isCreateModalOpen}
        onClose={closeCreateModal}
        onSubmit={handleCreateInventory}
        submitLabel="Create Inventory"
        isSubmitting={isSubmitting}
        width="medium"
      >
        <FormGroup 
          label="Name" 
          isRequired 
          fieldId="inventory-name"
        >
          <input
            type="text"
            id="inventory-name"
            name="name"
            value={newInventoryName}
            onChange={(e) => setNewInventoryName(e.target.value)}
            required
            className="pf-c-form-control"
            placeholder="Enter inventory name"
            style={{ width: '100%' }}
          />
        </FormGroup>
      </ModalForm>
      
      {/* Delete Confirmation Modal */}
      <DeleteConfirmationModal
        isOpen={isDeleteModalOpen}
        onClose={closeDeleteModal}
        onDelete={handleDeleteSelected}
        title={`Delete ${selectedItems.length} ${selectedItems.length === 1 ? 'Inventory' : 'Inventories'}`}
        message={
          selectedItems.length > 0 
            ? `Are you sure you want to delete the following ${
                selectedItems.length === 1 ? 'inventory' : 'inventories'
              }?
              
              ${inventories
                .filter(inv => selectedItems.includes(inv.id))
                .map(inv => `• ${inv.name}`)
                .join('\n')}`
            : 'No inventories selected'
        }
        isDeleting={isDeleting}
      />
    </PageSection>
  );
};

export default Inventories; 