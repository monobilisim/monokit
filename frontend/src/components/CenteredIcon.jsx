import React from 'react';

/**
 * A wrapper component that ensures icons are properly centered
 * 
 * @param {Object} props Component properties
 * @param {React.ReactNode} props.icon The icon to center
 * @param {Object} props.style Additional styles to apply
 * @returns {React.ReactElement} The centered icon
 */
const CenteredIcon = ({ icon, style = {} }) => {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: '1em',
        height: '1em',
        verticalAlign: 'middle',
        ...style
      }}
    >
      {icon}
    </span>
  );
};

export default CenteredIcon; 