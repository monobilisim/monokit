import React from 'react';
import { Button } from '@patternfly/react-core';
import CenteredIcon from './CenteredIcon';

/**
 * A wrapper for PatternFly Button that ensures icons are properly centered
 * 
 * @param {Object} props Component properties
 * @param {React.ReactNode} props.icon The icon to display
 * @param {string} props.children Button text
 * @returns {React.ReactElement} Button with centered icon
 */
const ButtonWithCenteredIcon = ({ icon, children, ...props }) => {
  return (
    <Button
      {...props}
      icon={icon ? <CenteredIcon icon={icon} /> : undefined}
    >
      {children}
    </Button>
  );
};

export default ButtonWithCenteredIcon; 