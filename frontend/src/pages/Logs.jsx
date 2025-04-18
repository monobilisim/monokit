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
  EmptyState,
  EmptyStateVariant,
  EmptyStateBody
} from '@patternfly/react-core';
import { AlertActionCloseButton } from '@patternfly/react-core/dist/js/components/Alert/AlertActionCloseButton';
import { TrashIcon } from '@patternfly/react-icons';
import { Chart, ChartAxis, ChartBar, ChartGroup, ChartVoronoiContainer } from '@patternfly/react-charts';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
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
  const [availableTypes, setAvailableTypes] = useState([]);
  const [typeFilter, setTypeFilter] = useState('');
  const [isTypeFilterOpen, setIsTypeFilterOpen] = useState(false);
  const [selectedLogs, setSelectedLogs] = useState([]);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const { theme } = useTheme();
  
  const colors = theme === 'dark' ? GruvboxColors.dark : GruvboxColors.light;
  const logLevels = ['info', 'warning', 'error', 'critical'];

  const clearLevelFilter = () => {
    setLevelFilter('');
    setIsLevelFilterOpen(false);
    setPage(1);
  };

  const clearHostFilter = () => {
    setHostFilter('');
    setIsHostFilterOpen(false);
    setPage(1);
  };

  const clearTypeFilter = () => {
    setTypeFilter('');
    setIsTypeFilterOpen(false);
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
    if (!logs.length) return [];

    // Find latest log timestamp and round up to next 10 minutes
    const latest = new Date(Math.max(...logs.map(l => new Date(l.timestamp).getTime())));
    latest.setMilliseconds(0);
    latest.setSeconds(0);
    latest.setMinutes(Math.ceil(latest.getMinutes() / 10) * 10);

    // Set start time to one hour before latest, rounded down to nearest 10 minutes
    const start = new Date(latest);
    start.setHours(latest.getHours() - 1);
    start.setMinutes(Math.floor(start.getMinutes() / 10) * 10);

    // Create time slots
    const timeMap = {};
    const currentTime = new Date(start);
    while (currentTime <= latest) {
      timeMap[currentTime.getTime()] = {
        timestamp: currentTime.getTime(),
        info: 0,
        warning: 0,
        error: 0,
        critical: 0
      };
      currentTime.setMinutes(currentTime.getMinutes() + 10);
    }

    // Count logs in their time slots
    logs.forEach(log => {
      const logTime = new Date(log.timestamp);
      logTime.setMilliseconds(0);
      logTime.setSeconds(0);
      logTime.setMinutes(Math.floor(logTime.getMinutes() / 10) * 10);
      const slotTime = logTime.getTime();

      if (timeMap[slotTime]) {
        timeMap[slotTime][log.level.toLowerCase()]++;
      }
    });

    return Object.values(timeMap);
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
      
      const deletePromises = selectedLogs.map(logId => 
        api.delete(`/logs/${logId}`)
      );
      
      await Promise.all(deletePromises);
      
      addAlert(
        `Successfully deleted ${selectedLogs.length} log${selectedLogs.length > 1 ? 's' : ''}`, 
        AlertVariant.success
      );
      
      setSelectedLogs([]);
      setIsDeleteModalOpen(false);
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
        type: typeFilter,
        message_text: searchValue
      };

      const response = await api.post('/logs/search', searchParams);
      
      if (response.data && typeof response.data === 'object') {
        const logsData = response.data.logs || [];
        const pagination = response.data.pagination || { total: 0 };
        
        setLogs(logsData);
        setTotalItems(pagination.total);
        setChartData(processLogsForChart(logsData));

        if (logsData.length > 0) {
          if (availableHosts.length === 0) {
            const hosts = [...new Set(logsData.map(log => log.host_name).filter(Boolean))];
            setAvailableHosts(hosts);
          }
          if (availableTypes.length === 0) {
            const types = [...new Set(logsData.map(log => log.type).filter(Boolean))];
            setAvailableTypes(types);
          }
        }
      } else {
        console.error('Unexpected response format:', response.data);
        setLogs([]);
        setTotalItems(0);
      }
    } catch (err) {
      console.error('Failed to fetch logs:', err);
      if (err.response?.status === 401) {
        onAuthError(err);
      } else {
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
  }, [page, perPage, levelFilter, hostFilter, typeFilter, searchValue]);

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

  const onTypeSelect = (_event, selection) => {
    setTypeFilter(selection);
    setIsTypeFilterOpen(false);
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
              <div style={{ height: '250px', width: '600px', margin: '0 auto', background: colors.bg0, padding: '16px', borderRadius: '8px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Chart
                  ariaDesc="Log activity over last hour"
                  ariaTitle="Time series data of system logs"
                  name="log-activity"
                  height={250}
                  width={600}
                  containerComponent={
                    <ChartVoronoiContainer
                      constrainToVisibleArea
                      voronoiDimension="x"
                      labels={({ datum }) => (
                        `${datum.name}: ${datum.y} ${datum.y === 1 ? 'log' : 'logs'} at ${
                          new Date(datum.x).toLocaleTimeString('en-US', {
                            hour: 'numeric',
                            minute: '2-digit',
                            hour12: true
                          })
                        }`
                      )}
                    />
                  }
                  legendData={[
                    { name: 'Info', symbol: { fill: colors.blue } },
                    { name: 'Warning', symbol: { fill: colors.yellow } },
                    { name: 'Error', symbol: { fill: colors.red } },
                    { name: 'Critical', symbol: { fill: colors.purple } }
                  ]}
                  legendOrientation="vertical"
                  legendPosition="right"
                  padding={{
                    bottom: 50,
                    left: 50,
                    right: 150,
                    top: 20
                  }}
                  scale={{ x: "time", y: "linear" }}
                  domain={{
                    x: [
                      chartData.length ? new Date(chartData[0].timestamp).getTime() : new Date().getTime() - 3600000,
                      chartData.length ? new Date(chartData[chartData.length - 1].timestamp).getTime() + 600000 : new Date().getTime()
                    ],
                    y: [0, Math.max(5, Math.ceil(Math.max(...chartData.map(d => Math.max(d.info, d.warning, d.error, d.critical)))))]
                  }}
                >
                  <ChartAxis
                    fixLabelOverlap
                    tickFormat={x => {
                      const date = new Date(x);
                      return date.toLocaleTimeString('en-US', { 
                        hour: 'numeric',
                        minute: '2-digit',
                        hour12: true
                      });
                    }}
                    style={{
                      tickLabels: { 
                        fontSize: 12, 
                        angle: -45, 
                        textAnchor: 'end', 
                        fill: colors.fg 
                      },
                      axis: { stroke: colors.bg3 },
                      ticks: { stroke: colors.bg3, size: 5 }
                    }}
                    tickCount={7}
                  />
                  <ChartAxis
                    dependentAxis
                    showGrid
                    tickCount={6}
                    style={{
                      axis: { stroke: colors.bg3 },
                      grid: { stroke: colors.bg1, strokeDasharray: '3,3' },
                      tickLabels: { fontSize: 12, fill: colors.fg },
                      ticks: { stroke: colors.bg3, size: 5 }
                    }}
                  />
                  <ChartGroup offset={12}>
                    {['info', 'warning', 'error', 'critical'].map((level) => (
                      <ChartBar
                        key={level}
                        data={chartData.map(d => ({
                          x: d.timestamp,
                          y: d[level],
                          name: level.charAt(0).toUpperCase() + level.slice(1)
                        }))}
                        barWidth={12}
                        style={{
                          data: {
                            fill: colors[level === 'info' ? 'blue' : level === 'warning' ? 'yellow' : level === 'error' ? 'red' : 'purple']
                          }
                        }}
                      />
                    ))}
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
              <ToolbarItem>
                <Select
                  aria-label="Select Type"
                  isOpen={isTypeFilterOpen}
                  selected={typeFilter}
                  onToggle={() => setIsTypeFilterOpen(!isTypeFilterOpen)}
                  onSelect={onTypeSelect}
                  placeholderText="Filter by Type"
                  hasInlineFilter
                  onFilter={(_, value) => {
                    return availableTypes.filter(t => 
                      t.toLowerCase().includes(value.toLowerCase())
                    );
                  }}
                  toggle={(toggleRef) => (
                    <MenuToggle
                      ref={toggleRef}
                      onClick={() => setIsTypeFilterOpen(!isTypeFilterOpen)}
                      isExpanded={isTypeFilterOpen}
                    >
                      {typeFilter || "Filter by Type"}
                    </MenuToggle>
                  )}
                >
                  {typeFilter && (
                    <SelectOption key="clear" value="" onClick={clearTypeFilter}>
                      All Types (Clear Filter)
                    </SelectOption>
                  )}
                  {availableTypes.map((t, index) => (
                    <SelectOption key={index} value={t}>
                      {t}
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
                    {levelFilter || hostFilter || typeFilter ? 
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
