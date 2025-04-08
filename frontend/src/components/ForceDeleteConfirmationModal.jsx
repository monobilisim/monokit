import React from 'react';
import {
  Button,
  Modal,
  ModalVariant,
  Alert,
  AlertVariant
} from '@patternfly/react-core';
import { ExclamationTriangleIcon } from '@patternfly/react-icons';

/**
 * A force delete confirmation modal with strong warnings
 * 
 * @param {Object} props Component properties
 * @param {boolean} props.isOpen Whether the modal is open
 * @param {Function} props.onClose Function to call when modal is closed
 * @param {Function} props.onDelete Function to call when force delete is confirmed
 * @param {string} props.hostname Name of the host to delete
 * @param {boolean} props.isDeleting Whether the deletion is in progress
 * @param {string} props.error Error message if deletion failed
 */
const ForceDeleteConfirmationModal = ({
  isOpen,
  onClose,
  onDelete,
  hostname,
  isDeleting = false,
  error = null
}) => {
  return (
    <Modal
      isOpen={isOpen}
      title="Force Delete Host"
      variant={ModalVariant.large}
      onClose={onClose}
      appendTo={document.body}
      showClose={true}
      hasNoBodyWrapper={false}
      style={{ width: '800px', maxWidth: '95vw' }}
    >
      <div style={{ padding: '32px 24px' }}>
        <div style={{ display: 'flex', alignItems: 'flex-start' }}>
          <ExclamationTriangleIcon 
            color="#f0ab00" 
            size="lg" 
            style={{ 
              marginRight: '28px', 
              marginTop: '4px', 
              flexShrink: 0, 
              width: '24px', 
              height: '24px' 
            }} 
          />
          <div style={{ width: '100%' }}>
            <p style={{ fontSize: '18px', lineHeight: '1.5', marginBottom: '28px', fontWeight: 'bold' }}>
              WARNING: You are about to force delete the host "{hostname}".
            </p>
            <p style={{ fontSize: '16px', lineHeight: '1.5', marginBottom: '28px', whiteSpace: 'pre-line' }}>
              This will immediately and permanently delete the host from the system, bypassing all safety checks.
            </p>
            <p style={{ fontSize: '16px', color: '#C9190B', marginBottom: '28px', fontWeight: 'bold' }}>
              This should only be used as a last resort when normal deletion fails.
            </p>
            
            {error && (
              <Alert 
                variant={AlertVariant.danger} 
                title="Error" 
                isInline 
                style={{ marginBottom: '28px' }}
              >
                {error}
              </Alert>
            )}
            
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '20px', marginTop: '36px' }}>
              <Button
                variant="danger"
                onClick={onDelete}
                isLoading={isDeleting}
                isDisabled={isDeleting}
                style={{ padding: '10px 28px', fontSize: '16px', minWidth: '120px' }}
              >
                {isDeleting ? 'Deleting...' : 'Force Delete'}
              </Button>
              <Button 
                variant="link" 
                onClick={onClose} 
                isDisabled={isDeleting}
                style={{ padding: '10px 28px', fontSize: '16px' }}
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

export default ForceDeleteConfirmationModal;