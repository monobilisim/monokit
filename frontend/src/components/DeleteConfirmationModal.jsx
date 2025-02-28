import React from 'react';
import {
  Button,
  Modal,
  ModalVariant
} from '@patternfly/react-core';
import { ExclamationTriangleIcon } from '@patternfly/react-icons';
import CenteredIcon from './CenteredIcon';

/**
 * A reusable delete confirmation modal
 * 
 * @param {Object} props Component properties
 * @param {boolean} props.isOpen Whether the modal is open
 * @param {Function} props.onClose Function to call when modal is closed
 * @param {Function} props.onDelete Function to call when delete is confirmed
 * @param {string} props.title Modal title
 * @param {string} props.message Confirmation message
 * @param {boolean} props.isDeleting Whether the deletion is in progress
 */
const DeleteConfirmationModal = ({
  isOpen,
  onClose,
  onDelete,
  title = 'Confirm Deletion',
  message = 'Are you sure you want to delete this item? This action cannot be undone.',
  isDeleting = false
}) => {
  return (
    <Modal
      isOpen={isOpen}
      title={title}
      variant={ModalVariant.large}
      onClose={onClose}
      appendTo={document.body}
      showClose={true}
      hasNoBodyWrapper={false}
      style={{ width: '800px', maxWidth: '95vw' }}
    >
      <div style={{ padding: '24px 0' }}>
        <div style={{ display: 'flex', alignItems: 'flex-start' }}>
          <CenteredIcon 
            icon={<ExclamationTriangleIcon color="#f0ab00" size="lg" />}
            style={{ 
              marginRight: '24px', 
              marginTop: '4px', 
              flexShrink: 0, 
              width: '24px', 
              height: '24px' 
            }} 
          />
          <div>
            <p style={{ fontSize: '18px', lineHeight: '1.5', marginBottom: '24px', whiteSpace: 'pre-line' }}>
              {message}
            </p>
            <p style={{ fontSize: '14px', color: '#666', marginBottom: '32px' }}>
              This action cannot be undone.
            </p>
            
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '16px', marginTop: '32px' }}>
              <Button
                variant="danger"
                onClick={onDelete}
                isLoading={isDeleting}
                isDisabled={isDeleting}
                style={{ padding: '10px 24px', fontSize: '16px', minWidth: '120px' }}
              >
                {isDeleting ? 'Deleting...' : 'Delete'}
              </Button>
              <Button 
                variant="link" 
                onClick={onClose} 
                isDisabled={isDeleting}
                style={{ padding: '10px 24px', fontSize: '16px' }}
              >
                Cancel
              </Button>
            </div>
          </div>
        </div>
      </div>
    </Modal>
  );
};

export default DeleteConfirmationModal; 