import React from 'react';
import { Button } from '@patternfly/react-core';
import ErrorModal from './ErrorModal'; // Import the ErrorModal

/**
 * Error Boundary component to catch JavaScript errors in child components
 * and display a fallback UI instead of crashing the whole application.
 */
class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { 
      hasError: false,
      error: null,
      errorInfo: null
    };
  }

  static getDerivedStateFromError(error) {
    // Update state so the next render will show the fallback UI
    return { hasError: true, error };
  }

  componentDidCatch(error, errorInfo) {
    // Log the error to the console
    console.error('ErrorBoundary caught an error:', error, errorInfo);
    this.setState({ errorInfo });
    
    // You can also log the error to an error reporting service
    // logErrorToService(error, errorInfo);
  }

  resetError = () => {
    this.setState({ hasError: false, error: null, errorInfo: null });
    
    // If a reset handler was provided, call it
    if (typeof this.props.onReset === 'function') {
      this.props.onReset();
    }
  }

  render() {
    if (this.state.hasError) {
      // Prepare debug info for the modal - always include details
      const debugInfo = this.state.error
        ? `Error: ${this.state.error.toString()}\n\nComponent Stack:\n${this.state.errorInfo?.componentStack}`
        : 'No detailed error information available.'; // Fallback if error object is somehow missing

      // Render the ErrorModal instead of the inline Alert
      return (
        <>
          <ErrorModal
            isOpen={true} // Modal is open when there's an error
            onClose={this.resetError} // Use resetError to close the modal and reset state
            title="Application Error"
            message="An unexpected error occurred. You can try again or contact support if the issue persists."
            debugInfo={debugInfo}
          />
          {/* Optionally render a fallback UI in place, or null */}
          {this.props.fallback || null} 
        </>
      );
    }

    // If there's no error, render children normally
    return this.props.children;
  }
}

export default ErrorBoundary;
