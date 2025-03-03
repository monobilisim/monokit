import React, { useState, useEffect } from 'react';
import {
  PageSection,
  Title,
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Spinner,
  Alert,
  Label,
  Split,
  SplitItem,
  Stack,
  StackItem,
  Tooltip,
  Popover,
} from '@patternfly/react-core';
import {
  Chart,
  ChartPie,
  ChartThemeColor,
  ChartTooltip,
  ChartContainer
} from '@patternfly/react-charts';
import axios from 'axios';
import { useTheme } from '../ThemeContext.jsx';
import { GruvboxColors } from '../ThemeContext.jsx';
import api from '../utils/api';

const Dashboard = () => {
  const [loading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [userRole, setUserRole] = useState('');
  const [stats, setStats] = useState({
    total: 0,
    online: 0,
    offline: 0,
    deletion: 0,
    unknown: 0
  });
  const [userInfo, setUserInfo] = useState(null);
  const [isChartHovered, setIsChartHovered] = useState(false);
  const [isLogChartHovered, setIsLogChartHovered] = useState(false);
  const [errorCount, setErrorCount] = useState(0);
  const [logStats, setLogStats] = useState({
    info: 0,
    warning: 0,
    error: 0,
    critical: 0
  });
  const { theme } = useTheme();
  
  // Get the appropriate color palette based on the current theme
  const colors = theme === 'dark' ? GruvboxColors.dark : GruvboxColors.light;

  const cardStyles = {
    backgroundColor: colors.bg0,
    color: colors.fg,
    border: `1px solid ${colors.bg3}`,
    boxShadow: `0 2px 4px ${theme === 'dark' ? 'rgba(0,0,0,0.3)' : 'rgba(0,0,0,0.1)'}`,
  };

  const titleStyles = {
    color: colors.fg0,
  };

  const labelStyles = {
    backgroundColor: colors.bg2,
    color: colors.fg1,
    padding: '8px 16px',
    borderRadius: '30px',
    border: `1px solid ${colors.bg3}`
  };

  const statusLabelStyles = (color) => {
    let bgColor, textColor;
    
    switch (color) {
      case 'blue':
        bgColor = colors.blue;
        textColor = theme === 'dark' ? colors.bg0 : colors.fg0;
        break;
      case 'green':
        bgColor = colors.green;
        textColor = theme === 'dark' ? colors.bg0 : colors.fg0;
        break;
      case 'red':
        bgColor = colors.red;
        textColor = theme === 'dark' ? colors.bg0 : colors.fg0;
        break;
      case 'yellow':
        bgColor = colors.yellow;
        textColor = theme === 'dark' ? colors.bg0 : colors.fg0;
        break;
      case 'purple':
        bgColor = colors.purple;
        textColor = theme === 'dark' ? colors.bg0 : colors.fg0;
        break;
      default:
        bgColor = colors.bg2;
        textColor = colors.fg1;
    }
    
    return {
      backgroundColor: bgColor,
      color: textColor,
      padding: '8px 16px',
      borderRadius: '30px',
      fontWeight: '500',
      boxShadow: theme === 'dark' ? `0 2px 4px rgba(0,0,0,0.2)` : 'none',
    };
  };

  useEffect(() => {
    fetchUserInfo();
    if (userRole === 'admin') {
      fetchErrorCount();
      fetchLogStats();
    }
  }, [userRole]);

  const fetchLogStats = async () => {
    try {
      const now = new Date();
      const lastWeek = new Date(now.getTime() - (7 * 24 * 60 * 60 * 1000));
      
      // Fetch counts for each log level
      const logLevels = ['info', 'warning', 'error', 'critical'];
      const counts = {};
      
      for (const level of logLevels) {
        const searchParams = {
          page: 1,
          page_size: 1,
          level: level,
          start_time: lastWeek.toISOString(),
          end_time: now.toISOString()
        };

        try {
          const response = await api.post('/logs/search', searchParams);
          if (response.data && typeof response.data === 'object') {
            const pagination = response.data.pagination || { total: 0 };
            counts[level] = pagination.total;
          }
        } catch (err) {
          console.error(`Failed to fetch ${level} logs:`, err);
          counts[level] = 0;
        }
      }
      
      setLogStats(counts);
    } catch (err) {
      console.error('Failed to fetch log statistics:', err);
    }
  };

  const fetchErrorCount = async () => {
    try {
      const now = new Date();
      const yesterday = new Date(now.getTime() - (24 * 60 * 60 * 1000));
      
      const searchParams = {
        page: 1,
        page_size: 1,
        level: 'error',
        start_time: yesterday.toISOString(),
        end_time: now.toISOString()
      };

      const response = await api.post('/logs/search', searchParams);
      
      if (response.data && typeof response.data === 'object') {
        const pagination = response.data.pagination || { total: 0 };
        setErrorCount(pagination.total);
      }
    } catch (err) {
      console.error('Failed to fetch error count:', err);
    }
  };

  const fetchUserInfo = async () => {
    try {
      const response = await axios.get('/api/v1/auth/me', {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      setUserInfo(response.data);
      setUserRole(response.data.role);
      
      if (response.data.role === 'admin') {
        fetchHostStats();
      } else {
        // For regular users, only fetch their assigned hosts
        fetchUserHosts();
      }
    } catch (err) {
      setError('Failed to fetch user information');
      setIsLoading(false);
    }
  };

  const fetchHostStats = async () => {
    try {
      const response = await axios.get('/api/v1/hosts', {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      const hosts = response.data;
      
      // Normalize status values to match the Hosts page
      const normalizedHosts = hosts.map(host => ({
        ...host,
        status: host.status?.toLowerCase() === 'scheduled for deletion' ? 'Scheduled for deletion' :
                host.status?.toLowerCase() === 'online' ? 'Online' :
                host.status?.toLowerCase() === 'offline' ? 'Offline' : 'Unknown'
      }));

      setStats({
        total: normalizedHosts.length,
        online: normalizedHosts.filter(h => h.status === 'Online').length,
        offline: normalizedHosts.filter(h => h.status === 'Offline').length,
        deletion: normalizedHosts.filter(h => h.status === 'Scheduled for deletion').length,
        unknown: normalizedHosts.filter(h => h.status === 'Unknown').length
      });
      setIsLoading(false);
    } catch (err) {
      setError('Failed to fetch host statistics');
      setIsLoading(false);
    }
  };

  const fetchUserHosts = async () => {
    try {
      const response = await axios.get('/api/v1/hosts/assigned', {
        headers: {
          Authorization: localStorage.getItem('token')
        }
      });
      const hosts = response.data;
      
      // Normalize status values to match the Hosts page
      const normalizedHosts = hosts.map(host => ({
        ...host,
        status: host.status?.toLowerCase() === 'online' ? 'Online' :
                host.status?.toLowerCase() === 'offline' ? 'Offline' : 'Unknown'
      }));

      setStats({
        total: normalizedHosts.length,
        online: normalizedHosts.filter(h => h.status === 'Online').length,
        offline: normalizedHosts.filter(h => h.status === 'Offline').length,
        deletion: 0, // Regular users don't see deletion status
        unknown: normalizedHosts.filter(h => h.status === 'Unknown').length
      });
      setIsLoading(false);
    } catch (err) {
      setError('Failed to fetch host statistics');
      setIsLoading(false);
    }
  };

  if (loading) {
    return (
      <PageSection style={{ backgroundColor: colors.bg }}>
        <Spinner />
      </PageSection>
    );
  }

  // Render the donut chart for host status
  const renderDonutChart = () => {
    const radius = 85;  // Slightly reduced radius
    const strokeWidth = 25;  // Slightly reduced stroke width
    const center = 100;
    const total = stats.total || 1;
    
    // Calculate the circumference
    const circumference = 2 * Math.PI * radius;
    
    // Calculate stroke-dasharray and stroke-dashoffset for each segment
    let currentOffset = 0;
    const segments = [];
    
    if (stats.online > 0) {
      const percentage = stats.online / total;
      segments.push({
        color: '#3E8635',
        dashArray: `${circumference * percentage} ${circumference * (1 - percentage)}`,
        dashOffset: -currentOffset * circumference,
        count: stats.online,
        label: 'Online'
      });
      currentOffset += percentage;
    }
    
    if (stats.offline > 0) {
      const percentage = stats.offline / total;
      segments.push({
        color: '#C9190B',
        dashArray: `${circumference * percentage} ${circumference * (1 - percentage)}`,
        dashOffset: -currentOffset * circumference,
        count: stats.offline,
        label: 'Offline'
      });
      currentOffset += percentage;
    }
    
    if (userRole === 'admin' && stats.deletion > 0) {
      const percentage = stats.deletion / total;
      segments.push({
        color: '#F0AB00',
        dashArray: `${circumference * percentage} ${circumference * (1 - percentage)}`,
        dashOffset: -currentOffset * circumference,
        count: stats.deletion,
        label: 'Scheduled for deletion'
      });
      currentOffset += percentage;
    }
    
    if (stats.unknown > 0) {
      const percentage = stats.unknown / total;
      segments.push({
        color: '#878787',
        dashArray: `${circumference * percentage} ${circumference * (1 - percentage)}`,
        dashOffset: -currentOffset * circumference,
        count: stats.unknown,
        label: 'Unknown'
      });
    }

    // Create status summary for popover content
    const statusSummary = (
      <div style={{ 
        padding: '12px', 
        backgroundColor: colors.bg1,
        color: colors.fg,
        borderRadius: '4px',
        border: `1px solid ${colors.bg3}`,
      }}>
        <div style={{ 
          marginBottom: '12px', 
          fontWeight: 'bold', 
          fontSize: '16px',
          borderBottom: `1px solid ${colors.bg3}`,
          paddingBottom: '8px',
          color: colors.fg0
        }}>
          Host Status Summary
        </div>
        {segments.map((segment, index) => (
          <div key={index} style={{ display: 'flex', alignItems: 'center', marginBottom: index < segments.length - 1 ? '8px' : 0 }}>
            <span style={{ 
              display: 'inline-block', 
              width: '12px', 
              height: '12px', 
              borderRadius: '50%', 
              backgroundColor: segment.color, 
              marginRight: '8px',
              boxShadow: theme === 'dark' ? `0 0 4px ${segment.color}` : 'none'
            }}></span>
            <span style={{ fontWeight: '500' }}>{segment.label}: {segment.count}</span>
          </div>
        ))}
      </div>
    );

    return (
      <div 
        style={{ 
          width: '200px', 
          height: '200px', 
          margin: '0 auto',
          position: 'relative',
          cursor: 'pointer',
        }}
      >
        <Popover
          position="top"
          bodyContent={statusSummary}
          aria-label="Host status popover"
          showClose={false}
          distance={16}
          appendTo={() => document.body}
          isVisible={isChartHovered}
        >
          <div
            style={{
              width: '200px',
              height: '200px',
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              transition: 'transform 0.2s ease-in-out',
              transform: isChartHovered ? 'scale(1.03)' : 'scale(1)',
            }}
            onMouseEnter={() => setIsChartHovered(true)}
            onMouseLeave={() => setIsChartHovered(false)}
          >
            <svg width="170" height="170" viewBox="0 0 200 200" style={{ backgroundColor: colors.bg0 }}>
              {segments.map((segment, index) => (
                <circle
                  key={index}
                  cx={center}
                  cy={center}
                  r={radius}
                  fill="none"
                  stroke={segment.color}
                  strokeWidth={strokeWidth}
                  strokeDasharray={segment.dashArray}
                  strokeDashoffset={segment.dashOffset}
                  transform="rotate(-90 100 100)"
                  style={{
                    transition: 'stroke-width 0.2s ease-in-out',
                    strokeWidth: isChartHovered ? strokeWidth + 2 : strokeWidth
                  }}
                />
              ))}
              <circle
                cx={center}
                cy={center}
                r={radius - strokeWidth / 2}
                fill={colors.bg0}
              />
              <text
                x={center}
                y={center - 5}
                textAnchor="middle"
                dominantBaseline="central"
                style={{
                  fill: colors.fg0,
                  fontSize: '38px',
                  fontWeight: 'bold',
                  fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif'
                }}
              >
                {stats.total}
              </text>
              <text
                x={center}
                y={center + 25}
                textAnchor="middle"
                style={{
                  fill: colors.fg1,
                  fontSize: '16px',
                  fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif'
                }}
              >
                hosts
              </text>
            </svg>
          </div>
        </Popover>
      </div>
    );
  };

  // Render the donut chart for log severity
  const renderLogSeverityChart = () => {
    const radius = 85;
    const strokeWidth = 25;
    const center = 100;
    
    // Calculate total logs
    const totalLogs = Object.values(logStats).reduce((sum, count) => sum + count, 0) || 1;
    
    // Calculate the circumference
    const circumference = 2 * Math.PI * radius;
    
    // Define colors for each log level
    const levelColors = {
      info: '#0066CC',     // Blue
      warning: '#F0AB00',  // Yellow/Orange
      error: '#C9190B',    // Red
      critical: '#A30000'  // Dark Red
    };
    
    // Calculate stroke-dasharray and stroke-dashoffset for each segment
    let currentOffset = 0;
    const segments = [];
    
    // Create segments for each log level that has data
    Object.entries(logStats).forEach(([level, count]) => {
      if (count > 0) {
        const percentage = count / totalLogs;
        segments.push({
          color: levelColors[level],
          dashArray: `${circumference * percentage} ${circumference * (1 - percentage)}`,
          dashOffset: -currentOffset * circumference,
          count,
          label: level.charAt(0).toUpperCase() + level.slice(1)
        });
        currentOffset += percentage;
      }
    });

    // Create log summary for popover content
    const logSummary = (
      <div style={{ 
        padding: '12px', 
        backgroundColor: colors.bg1,
        color: colors.fg,
        borderRadius: '4px',
        border: `1px solid ${colors.bg3}`,
      }}>
        <div style={{ 
          marginBottom: '12px', 
          fontWeight: 'bold', 
          fontSize: '16px',
          borderBottom: `1px solid ${colors.bg3}`,
          paddingBottom: '8px',
          color: colors.fg0
        }}>
          Log Severity Summary (7 days)
        </div>
        {segments.map((segment, index) => (
          <div key={index} style={{ display: 'flex', alignItems: 'center', marginBottom: index < segments.length - 1 ? '8px' : 0 }}>
            <span style={{ 
              display: 'inline-block', 
              width: '12px', 
              height: '12px', 
              borderRadius: '50%', 
              backgroundColor: segment.color, 
              marginRight: '8px',
              boxShadow: theme === 'dark' ? `0 0 4px ${segment.color}` : 'none'
            }}></span>
            <span style={{ fontWeight: '500' }}>{segment.label}: {segment.count}</span>
          </div>
        ))}
      </div>
    );

    return (
      <div 
        style={{ 
          width: '200px', 
          height: '200px', 
          margin: '0 auto',
          position: 'relative',
          cursor: 'pointer',
        }}
      >
        <Popover
          position="top"
          bodyContent={logSummary}
          aria-label="Log severity popover"
          showClose={false}
          distance={16}
          appendTo={() => document.body}
          isVisible={isLogChartHovered}
        >
          <div
            style={{
              width: '200px',
              height: '200px',
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              transition: 'transform 0.2s ease-in-out',
              transform: isLogChartHovered ? 'scale(1.03)' : 'scale(1)',
            }}
            onMouseEnter={() => setIsLogChartHovered(true)}
            onMouseLeave={() => setIsLogChartHovered(false)}
          >
            <svg width="170" height="170" viewBox="0 0 200 200" style={{ backgroundColor: colors.bg0 }}>
              {segments.map((segment, index) => (
                <circle
                  key={index}
                  cx={center}
                  cy={center}
                  r={radius}
                  fill="none"
                  stroke={segment.color}
                  strokeWidth={strokeWidth}
                  strokeDasharray={segment.dashArray}
                  strokeDashoffset={segment.dashOffset}
                  transform="rotate(-90 100 100)"
                  style={{
                    transition: 'stroke-width 0.2s ease-in-out',
                    strokeWidth: isLogChartHovered ? strokeWidth + 2 : strokeWidth
                  }}
                />
              ))}
              <circle
                cx={center}
                cy={center}
                r={radius - strokeWidth / 2}
                fill={colors.bg0}
              />
              <text
                x={center}
                y={center - 5}
                textAnchor="middle"
                dominantBaseline="central"
                style={{
                  fill: colors.fg0,
                  fontSize: '38px',
                  fontWeight: 'bold',
                  fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif'
                }}
              >
                {totalLogs}
              </text>
              <text
                x={center}
                y={center + 25}
                textAnchor="middle"
                style={{
                  fill: colors.fg1,
                  fontSize: '16px',
                  fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif'
                }}
              >
                logs
              </text>
            </svg>
          </div>
        </Popover>
      </div>
    );
  };

  return (
    <PageSection style={{ 
      minHeight: '100%',
      padding: '24px'
    }}>
      <Stack hasGutter>
        <StackItem>
          <Title headingLevel="h1" size="2xl" style={{ ...titleStyles, backgroundColor: 'transparent' }}>Dashboard</Title>
        </StackItem>

        {userInfo && (
          <StackItem>
            <Card style={cardStyles}>
              <CardBody>
                <Stack hasGutter>
                  <Title headingLevel="h2" size="xl" style={titleStyles}>
                    Welcome, {userInfo.username}
                  </Title>
                  <Split hasGutter>
                    <SplitItem>
                      <Label style={statusLabelStyles('blue')}>
                        {userInfo.role}
                      </Label>
                    </SplitItem>
                    {userInfo.groups !== 'nil' && (
                      <SplitItem>
                        <Label style={labelStyles}>Groups: {userInfo.groups}</Label>
                      </SplitItem>
                    )}
                    {userInfo.inventories !== 'nil' && (
                      <SplitItem>
                        <Label style={statusLabelStyles('purple')}>
                          Inventories: {userInfo.inventories}
                        </Label>
                      </SplitItem>
                    )}
                  </Split>
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
        )}

        <StackItem>
          <Grid hasGutter>
            <GridItem span={4}>
              <Card style={cardStyles}>
                <CardTitle>
                  <Title headingLevel="h2" size="xl" style={titleStyles}>Host Status Overview</Title>
                </CardTitle>
                <CardBody>
                  {error ? (
                    <Alert variant="danger" title={error} style={{ backgroundColor: colors.red, color: colors.bg0 }} />
                  ) : (
                    <div style={{ height: '250px', width: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center' }}>
                      {renderDonutChart()}
                    </div>
                  )}
                </CardBody>
              </Card>
            </GridItem>

            {userRole === 'admin' && (
              <GridItem span={4}>
                <Card style={cardStyles}>
                  <CardTitle>
                    <Title headingLevel="h2" size="xl" style={titleStyles}>Log Severity Overview</Title>
                  </CardTitle>
                  <CardBody>
                    <div style={{ height: '250px', width: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center' }}>
                      {renderLogSeverityChart()}
                    </div>
                  </CardBody>
                </Card>
              </GridItem>
            )}

            <GridItem span={userRole === 'admin' ? 4 : 8}>
              <Card style={cardStyles}>
                <CardTitle>
                  <Title headingLevel="h2" size="xl" style={titleStyles}>Host Statistics</Title>
                </CardTitle>
                <CardBody>
                  <Stack hasGutter>
                    <StackItem>
                      <Title headingLevel="h3" size="md" style={titleStyles}>
                        Total Hosts: {stats.total}
                      </Title>
                    </StackItem>
                    <StackItem>
                      <Stack hasGutter>
                        <Label style={statusLabelStyles('green')}>Online: {stats.online}</Label>
                        <Label style={statusLabelStyles('red')}>Offline: {stats.offline}</Label>
                        {userRole === 'admin' && (
                          <Label style={statusLabelStyles('yellow')}>
                            Pending Deletion: {stats.deletion}
                          </Label>
                        )}
                      </Stack>
                    </StackItem>
                  </Stack>
                </CardBody>
              </Card>
            </GridItem>

            {userRole === 'admin' && (
              <GridItem span={4}>
                <Card style={cardStyles}>
                  <CardTitle>
                    <Title headingLevel="h2" size="xl" style={titleStyles}>System Status</Title>
                  </CardTitle>
                  <CardBody>
                    <Stack hasGutter>
                      <Label style={statusLabelStyles('green')}>API Server: Running</Label>
                      <Label style={statusLabelStyles('green')}>Database: Connected</Label>
                      <Label style={statusLabelStyles('red')}>
                        Errors (24h): {errorCount}
                      </Label>
                      <Label style={labelStyles}>
                        Last Update: {new Date().toLocaleString()}
                      </Label>
                    </Stack>
                  </CardBody>
                </Card>
              </GridItem>
            )}
          </Grid>
        </StackItem>
      </Stack>
    </PageSection>
  );
};

export default Dashboard;