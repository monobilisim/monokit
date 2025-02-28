import React from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  Form,
  ActionGroup,
} from '@patternfly/react-core';

/**
 * A reusable modal form component with proper sizing
 * 
 * @param {Object} props Component properties
 * @param {string} props.title Modal title
 * @param {boolean} props.isOpen Whether the modal is open
 * @param {Function} props.onClose Function to call when modal is closed
 * @param {Function} props.onSubmit Function to call when form is submitted
 * @param {React.ReactNode} props.children Modal content (form fields)
 * @param {string} props.submitLabel Label for the submit button (default: 'Save')
 * @param {boolean} props.isSubmitting Whether the form is submitting (for loading state)
 * @param {string} props.width Modal width: 'small' (500px), 'medium' (700px), 'large' (1120px)
 */
const ModalForm = ({
  title,
  isOpen,
  onClose,
  onSubmit,
  children,
  submitLabel = 'Save',
  isSubmitting = false,
  width = 'medium' // Default to medium for better usability
}) => {
  // Default variant is ModalVariant.medium
  let variant;
  switch (width) {
    case 'small':
      variant = ModalVariant.small; // 500px
      break;
    case 'medium':
      variant = ModalVariant.medium; // 700px
      break;
    case 'large':
      variant = ModalVariant.large; // 1120px
      break;
    default:
      variant = ModalVariant.medium;
  }

  const handleSubmit = (e) => {
    if (e) {
      e.preventDefault();
    }
    
    if (onSubmit) {
      onSubmit(e);
    }
  };

  // Removed modal's built-in actions as they're not working correctly
  // Instead, we'll put buttons directly in the modal body

  return (
    <Modal
      variant={variant}
      title={title}
      isOpen={isOpen}
      onClose={onClose}
      hasNoBodyWrapper
    >
      <div style={{ padding: '24px' }}>
        <Form id="modal-form" onSubmit={handleSubmit} style={{ width: '100%' }}>
          {children}
          
          {/* Button group always at the bottom of the form */}
          <ActionGroup style={{ marginTop: '24px', display: 'flex', justifyContent: 'flex-end' }}>
            <Button 
              type="submit"
              variant="primary" 
              onClick={handleSubmit}
              isLoading={isSubmitting}
              isDisabled={isSubmitting}
              style={{ marginRight: '8px' }}
            >
              {submitLabel}
            </Button>
            <Button 
              variant="link" 
              onClick={onClose}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </div>
    </Modal>
  );
};

export default ModalForm; 