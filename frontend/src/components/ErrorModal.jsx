import React, { useState } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  Alert,
  AlertActionCloseButton,
  ExpandableSection,
  ClipboardCopy,
  ClipboardCopyVariant
} from '@patternfly/react-core';
import { ExclamationCircleIcon } from '@patternfly/react-icons';
import PropTypes from 'prop-types';

/**
 * A reusable modal component for displaying errors.
 */
const ErrorModal = ({
  isOpen,
  onClose,
  title = 'An Error Occurred', // Default title
  message = 'Something went wrong. Please try again.', // Default message
  debugInfo = null // Default debugInfo
}) => {
   // Ensure title and message are always strings
   const displayTitle = title || 'Error';
   const displayMessage = message || 'An unexpected error occurred.';
   const [isExpanded, setIsExpanded] = useState(false);
 
   // Renamed handler and parameter for clarity
   const handleToggle = (newIsExpandedState) => { 
     setIsExpanded(newIsExpandedState);
   };
 
   // Ensure onClose is always callable
  const handleClose = () => {
    if (typeof onClose === 'function') {
      onClose();
    }
    // Reset expansion state when closing
    setIsExpanded(false);
  };

  return (
    <Modal
      variant={ModalVariant.large} // Change variant to large
      title={displayTitle} // Use guaranteed string
      titleIconVariant={() => <ExclamationCircleIcon style={{ color: 'var(--pf-global--danger-color--100)' }} />}
      isOpen={isOpen}
      onClose={handleClose}
      appendTo={document.body} // Ensures modal is on top
      showClose={true} // Explicitly show the 'X' close button
      style={{ zIndex: 9999 }} // Add high z-index just in case
      actions={[
        <Button key="close" variant="primary" onClick={handleClose}>
          Close
        </Button>
      ]}
      // Add explicit z-index if needed, though appendTo body usually handles stacking
      // style={{ zIndex: 9999 }} // Example: Use if stacking issues occur
    >
      {/* Add padding similar to the example modal */}
      <div style={{ padding: '24px' }}> 
        <Alert
          variant="danger"
          isInline
        title={displayTitle} // Use guaranteed string
        style={{ marginBottom: '16px', borderLeft: '3px solid var(--pf-global--danger-color--100)' }}
      >
        {displayMessage} {/* Use guaranteed string */}
      </Alert>

       {/* Render ExpandableSection only if debugInfo is truthy and not the specific message */}
       {debugInfo && displayMessage !== 'An unexpected error occurred. You can try again or contact support if the issue persists.' && (
         <ExpandableSection
           toggleText={isExpanded ? 'Hide Details' : 'Show Details'}
           onToggle={handleToggle} // Use renamed handler
           isExpanded={isExpanded}
         >
           {/* Ensure debugInfo is handled safely */}
           <pre style={{ marginTop: '16px', whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
             {debugInfo ? (typeof debugInfo === 'object' ? JSON.stringify(debugInfo, null, 2) : String(debugInfo)) : 'No debug information available.'}
           </pre>
         </ExpandableSection>
       )}
      </div> {/* Close padding div */}
    </Modal>
  );
};

ErrorModal.propTypes = {
  isOpen: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  title: PropTypes.string,
  message: PropTypes.string,
  debugInfo: PropTypes.oneOfType([PropTypes.string, PropTypes.object]),
};

export default ErrorModal;
