import React, { useState, useEffect } from 'react';
import DeleteConfirmationModal from '../components/DeleteConfirmationModal';
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  Flex,
  FlexItem,
  PageSection,
  Pagination,
  SearchInput,
  Spinner,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Select,
  SelectOption,
  Alert,
  AlertGroup,
  AlertVariant,
  MenuToggle,
  AlertActionCloseButton,
  EmptyState,
  EmptyStateVariant,
  EmptyStateBody,
  Grid,
  GridItem,
} from '@patternfly/react-core';
import { TrashIcon } from '@patternfly/react-icons';
import {
  Chart,
  ChartArea,
  ChartAxis,
  ChartGroup,
  ChartLegend,
  ChartThemeColor,
  ChartVoronoiContainer
} from '@patternfly/react-charts';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import axios from 'axios';
import api from '../utils/api';
import { useTheme } from '../ThemeContext.jsx';
import { GruvboxColors } from '../ThemeContext.jsx';

const Logs = ({ onAuthError }) => {
  const [chartData, setChartData] = useState([]);
  const [timeRange, setTimeRange] = useState('24h');
  const [logs, setLogs] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchValue, setSearchValue] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [totalItems, setTotalItems] = useState(0);
  const [alerts, setAlerts] = useState([]);
  const [levelFilter, setLevelFilter] = useState('');
  const [isLevelFilterOpen, setIsLevelFilterOpen] = useState(false);
  const [hostFilter, setHostFilter] = useState('');
  const [isHostFilterOpen, setIsHostFilterOpen] = useState(false);
  const [availableHosts, setAvailableHosts] = useState([]);
  const [selectedLogs, setSelectedLogs] = useState([]);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const { theme } = useTheme();
  
  // Get the appropriate color palette based on the current theme
  const colors = theme === 'dark' ? GruvboxColors.dark : GruvboxColors.light;

  const logLevels = ['info', 'warning', 'error', 'critical'];
  
  // Function to clear the level filter
  const clearLevelFilter = () => {
    setLevelFilter('');
    setIsLevelFilterOpen(false);
    setPage(1);
  };

  // Function to clear the host filter
  const clearHostFilter = () => {
    setHostFilter('');
    setIsHostFilterOpen(false);
    setPage(1);
  };

  const addAlert = (title, variant, description = '') => {
    const key = new Date().getTime();
    setAlerts([...alerts, { title, variant, key, description }]);
    setTimeout(() => {
      setAlerts(currentAlerts => currentAlerts.filter(alert => alert.key !== key));
    }, 5000);
  };

  const processLogsForChart = (logs) => {
    const timeMap = {};
    logs.forEach(log => {
      const timestamp = new Date(log.timestamp).getTime();
      if (!timeMap[timestamp]) {
        timeMap[timestamp] = { timestamp, info: 0, warning: 0, error: 0, critical: 0 };
      }
      timeMap[timestamp][log.level.toLowerCase()]++;
    });

    return Object.values(timeMap).sort((a, b) => a.timestamp - b.timestamp);
  };

  const handleSelectLog = (logId, isChecked) => {
    if (isChecked) {
      setSelectedLogs([...selectedLogs, logId]);
    } else {
      setSelectedLogs(selectedLogs.filter(id => id !== logId));
    }
  };

  const handleSelectAll = (isChecked) => {
    if (isChecked) {
      setSelectedLogs(logs.map(log => log.id));
    } else {
      setSelectedLogs([]);
    }
  };

  const openDeleteModal = () => {
    if (selectedLogs.length > 0) {
      setIsDeleteModalOpen(true);
    } else {
      addAlert('No logs selected', AlertVariant.info, 'Please select at least one log to delete.');
    }
  };

  const closeDeleteModal = () => {
    setIsDeleteModalOpen(false);
  };

  const deleteSelectedLogs = async () => {
    try {
      setIsLoading(true);
      
      // Delete each selected log
      const deletePromises = selectedLogs.map(logId => 
        api.delete(`/logs/${logId}`)
      );
      
      await Promise.all(deletePromises);
      
      // Show success message
      addAlert(
        `Successfully deleted ${selectedLogs.length} log${selectedLogs.length > 1 ? 's' : ''}`, 
        AlertVariant.success
      );
      
      // Clear selection and close modal
      setSelectedLogs([]);
      setIsDeleteModalOpen(false);
      
      // Refresh logs
      fetchLogs();
    } catch (err) {
      console.error('Failed to delete logs:', err);
      if (err.response?.status === 401) {
        onAuthError(err);
      } else {
        setError('Failed to delete logs. Please try again.');
        addAlert('Error deleting logs', AlertVariant.danger, err.message);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const fetchLogs = async () => {
    try {
      setIsLoading(true);
      setError(null);

      const searchParams = {
        page,
        page_size: perPage,
        level: levelFilter,
        host: hostFilter,
        message_text: searchValue
      };

      const response = await api.post('/logs/search', searchParams);
      
      // Check if response data has the expected structure
      if (response.data && typeof response.data === 'object') {
        // Handle case where logs might be null or undefined
        const logsData = response.data.logs || [];
        const pagination = response.data.pagination || { total: 0 };
        
        setLogs(logsData);
        setTotalItems(pagination.total);
        setChartData(processLogsForChart(logsData));

        // Extract unique host names for the host filter dropdown
        if (logsData.length > 0 && availableHosts.length === 0) {
          const hosts = [...new Set(logsData.map(log => log.host_name).filter(Boolean))];
          setAvailableHosts(hosts);
        }
      } else {
        // Handle unexpected response format
        console.error('Unexpected response format:', response.data);
        setLogs([]);
        setTotalItems(0);
      }
    } catch (err) {
      console.error('Failed to fetch logs:', err);
      if (err.response?.status === 401) {
        onAuthError(err);
      } else {
        // For non-auth errors, show empty logs instead of error when filtering
        if (levelFilter || hostFilter) {
          console.log(`No logs found for filters: level=${levelFilter}, host=${hostFilter}`);
          setLogs([]);
          setTotalItems(0);
        } else {
          setError('Failed to fetch logs. Please try again.');
          addAlert('Error fetching logs', AlertVariant.danger, err.message);
        }
      }
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchLogs();
  }, [page, perPage, levelFilter, hostFilter, searchValue]);

  const onSetPage = (_event, pageNumber) => {
    setPage(pageNumber);
  };

  const onPerPageSelect = (_event, perPage) => {
    setPerPage(perPage);
    setPage(1);
  };

  const onSearch = (_event, value) => {
    setSearchValue(value);
    setPage(1);
  };

  const onLevelSelect = (_event, selection) => {
    setLevelFilter(selection);
    setIsLevelFilterOpen(false);
    setPage(1);
  };

  const onHostSelect = (_event, selection) => {
    setHostFilter(selection);
    setIsHostFilterOpen(false);
    setPage(1);
  };

  return (
    <PageSection>
      <AlertGroup isToast>
        {alerts.map(({ key, variant, title, description }) => (
          <Alert
            key={key}
            variant={variant}
            title={title}
            actionClose={<AlertActionCloseButton title={title} onClose={() => setAlerts(alerts.filter(alert => alert.key !== key))} />}
          >
            {description}
          </Alert>
        ))}
      </AlertGroup>

      {/* Delete Confirmation Modal */}
      <DeleteConfirmationModal
        isOpen={isDeleteModalOpen}
        onClose={closeDeleteModal}
        onDelete={deleteSelectedLogs}
        title="Delete Logs"
        message={`Are you sure you want to delete ${selectedLogs.length} selected log${selectedLogs.length !== 1 ? 's' : ''}?`}
        isDeleting={isLoading}
      />

      <Card>
        <CardHeader>
          <Title headingLevel="h1">System Logs</Title>
        </CardHeader>
        <CardBody>
          <Card style={{ marginBottom: '24px' }}>
            <CardHeader>
              <Title headingLevel="h2" size="lg">Log Activity</Title>
            </CardHeader>
            <CardBody>
              <div style={{ height: '300px' }}>
                <Chart
                  ariaTitle="Log activity over time"
                  containerComponent={<ChartVoronoiContainer labels={({ datum }) => `${datum.name}: ${datum.y}`} />}
                  legendData={[
                    { name: 'Info', symbol: { fill: colors.blue } },
                    { name: 'Warning', symbol: { fill: colors.yellow } },
                    { name: 'Error', symbol: { fill: colors.red } },
                    { name: 'Critical', symbol: { fill: colors.purple } }
                  ]}
                  legendPosition="bottom-left"
                  height={300}
                  padding={{
                    bottom: 75,
                    left: 50,
                    right: 50,
                    top: 50
                  }}
                  width={800}
                  legendComponent={
                    <ChartLegend 
                      style={{
                        labels: { fill: colors.fg }
                      }}
                    />
                  }
                >
                  <ChartAxis tickFormat={x => new Date(x).toLocaleTimeString()} />
                  <ChartAxis dependentAxis />
                  <ChartGroup>
                    <ChartArea
                      data={chartData.map(d => ({ x: d.timestamp, y: d.info, name: 'Info' }))}
                      style={{ data: { fill: colors.blue, stroke: colors.blue } }}
                    />
                    <ChartArea
                      data={chartData.map(d => ({ x: d.timestamp, y: d.warning, name: 'Warning' }))}
                      style={{ data: { fill: colors.yellow, stroke: colors.yellow } }}
                    />
                    <ChartArea
                      data={chartData.map(d => ({ x: d.timestamp, y: d.error, name: 'Error' }))}
                      style={{ data: { fill: colors.red, stroke: colors.red } }}
                    />
                    <ChartArea
                      data={chartData.map(d => ({ x: d.timestamp, y: d.critical, name: 'Critical' }))}
                      style={{ data: { fill: colors.purple, stroke: colors.purple } }}
                    />
                  </ChartGroup>
                </Chart>
              </div>
            </CardBody>
          </Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <SearchInput
                  placeholder="Search logs..."
                  value={searchValue}
                  onChange={onSearch}
                  onClear={() => onSearch('')}
                />
              </ToolbarItem>
              <ToolbarItem>
                <Button 
                  variant="danger" 
                  icon={<TrashIcon />} 
                  onClick={openDeleteModal}
                  isDisabled={selectedLogs.length === 0}
                >
                  Delete Selected
                </Button>
              </ToolbarItem>
              <ToolbarItem>
                <Select
                  aria-label="Select Log Level"
                  isOpen={isLevelFilterOpen}
                  selected={levelFilter}
                  onToggle={() => setIsLevelFilterOpen(!isLevelFilterOpen)}
                  onSelect={onLevelSelect}
                  placeholderText="Filter by Level"
                  toggle={(toggleRef) => (
                    <MenuToggle
                      ref={toggleRef}
                      onClick={() => setIsLevelFilterOpen(!isLevelFilterOpen)}
                      isExpanded={isLevelFilterOpen}
                    >
                      {levelFilter ? levelFilter.charAt(0).toUpperCase() + levelFilter.slice(1) : "Filter by Level"}
                    </MenuToggle>
                  )}
                >
                  {/* Add an option to clear the filter */}
                  {levelFilter && (
                    <SelectOption key="clear" value="" onClick={clearLevelFilter}>
                      All Levels (Clear Filter)
                    </SelectOption>
                  )}
                  {logLevels.map((level, index) => (
                    <SelectOption key={index} value={level}>
                      {level.charAt(0).toUpperCase() + level.slice(1)}
                    </SelectOption>
                  ))}
                </Select>
              </ToolbarItem>
              <ToolbarItem>
                <Select
                  aria-label="Select Host"
                  isOpen={isHostFilterOpen}
                  selected={hostFilter}
                  onToggle={() => setIsHostFilterOpen(!isHostFilterOpen)}
                  onSelect={onHostSelect}
                  placeholderText="Filter by Host"
                  hasInlineFilter
                  onFilter={(_, value) => {
                    return availableHosts.filter(host => 
                      host.toLowerCase().includes(value.toLowerCase())
                    );
                  }}
                  toggle={(toggleRef) => (
                    <MenuToggle
                      ref={toggleRef}
                      onClick={() => setIsHostFilterOpen(!isHostFilterOpen)}
                      isExpanded={isHostFilterOpen}
                    >
                      {hostFilter || "Filter by Host"}
                    </MenuToggle>
                  )}
                >
                  {/* Add an option to clear the filter */}
                  {hostFilter && (
                    <SelectOption key="clear" value="" onClick={clearHostFilter}>
                      All Hosts (Clear Filter)
                    </SelectOption>
                  )}
                  {availableHosts.map((host, index) => (
                    <SelectOption key={index} value={host}>
                      {host}
                    </SelectOption>
                  ))}
                </Select>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>

          {error && (
            <Alert variant="danger" title={error} />
          )}

          {isLoading ? (
            <Flex justifyContent={{ default: 'justifyContentCenter' }}>
              <FlexItem>
                <Spinner />
              </FlexItem>
            </Flex>
          ) : (
            <>
              {logs.length === 0 ? (
                <EmptyState variant={EmptyStateVariant.full}>
                  <Title headingLevel="h3" size="lg">
                    No logs found
                  </Title>
                  <EmptyStateBody>
                    {levelFilter || hostFilter ? 
                      `No logs found with the current filters. Try selecting different filters or adjusting your search criteria.` : 
                      'No logs found. Try adjusting your search criteria.'}
                  </EmptyStateBody>
                </EmptyState>
              ) : (
                <>
                  <Table aria-label="Logs table">
                    <Thead>
                      <Tr>
                        <Th width={10}>
                          <Checkbox
                            id="select-all-logs"
                            isChecked={selectedLogs.length > 0 && selectedLogs.length === logs.length}
                            onChange={(_, checked) => handleSelectAll(checked)}
                            aria-label="Select all logs"
                            isIndeterminate={selectedLogs.length > 0 && selectedLogs.length < logs.length}
                          />
                        </Th>
                        <Th>Timestamp</Th>
                        <Th>Level</Th>
                        <Th>Component</Th>
                        <Th>Host</Th>
                        <Th>Message</Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {logs.map((log) => (
                        <Tr key={log.id}>
                          <Td>
                            <Checkbox
                              id={`select-log-${log.id}`}
                              isChecked={selectedLogs.includes(log.id)}
                              onChange={(_, checked) => handleSelectLog(log.id, checked)}
                              aria-label={`Select log ${log.id}`}
                            />
                          </Td>
                          <Td>{new Date(log.timestamp).toLocaleString()}</Td>
                          <Td>{log.level}</Td>
                          <Td>{log.component}</Td>
                          <Td>{log.host_name}</Td>
                          <Td>{log.message}</Td>
                        </Tr>
                      ))}
                    </Tbody>
                  </Table>

                  <Pagination
                    itemCount={totalItems}
                    perPage={perPage}
                    page={page}
                    onSetPage={onSetPage}
                    onPerPageSelect={onPerPageSelect}
                    variant="bottom"
                  />
                </>
              )}
            </>
          )}
        </CardBody>
      </Card>
    </PageSection>
  );
};

export default Logs;
