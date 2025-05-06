package esHealth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/spf13/cobra"
)

// DetectElasticsearch checks if the Elasticsearch configuration file exists.
// It returns true if 'es.yaml' or 'es.yml' is found in /etc/mono/, false otherwise.
func DetectElasticsearch() bool {
	common.LogFunctionEntry()
	exists := common.ConfExists("es")
	if exists {
		common.LogDebug("esHealth auto-detected successfully (config file found).")
	} else {
		common.LogDebug("esHealth auto-detection failed (config file not found).")
	}
	return exists
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "esHealth",
		EntryPoint: Main,
		// Platform:   "", // Specify platform if needed, e.g., "linux"
		AutoDetect: DetectElasticsearch,
	})
}

// Structs ShardAllocationResponse and UnassignedInfo are now defined in types.go
// as ShardAllocationAPIResponse and UnassignedInfoAPI respectively.

func Main(cmd *cobra.Command, args []string) {
	version := "0.2.0" // Updated version due to refactor
	common.ScriptName = "esHealth"
	common.TmpDir = common.TmpDir + "esHealth"
	common.Init()
	common.ConfInit("es", &Config) // Config is from types.go

	api.WrapperGetServiceStatus("esHealth") // Keep this for service status reporting

	// Collect health data
	healthData := collectEsHealthData()

	// Display as a nice box UI
	displayBoxUI(healthData, version)
}

// collectEsHealthData gathers all Elasticsearch health information.
func collectEsHealthData() *EsHealthData {
	healthData := NewEsHealthData() // From ui.go

	if Config.Api_url == "" {
		healthData.Error = "API URL is not configured in es.yml"
		common.LogError(healthData.Error)
		// Alarm for configuration error
		common.AlarmCheckDown("elasticsearch_config", healthData.Error, false, "", "")
		return healthData
	}
	common.AlarmCheckUp("elasticsearch_config", "Elasticsearch API URL is configured.", false)

	// Check Elasticsearch cluster health
	clusterHealth, err := getClusterHealth()
	if err != nil {
		healthData.Error = fmt.Sprintf("Failed to get cluster health: %v", err)
		common.LogError(healthData.Error)
		// Alarm is handled by getClusterHealth for connection/request errors
		return healthData
	}

	healthData.ClusterName = clusterHealth.ClusterName
	healthData.Status = clusterHealth.Status
	healthData.NodeStats = NodeStatsInfo{
		TotalNodes:     clusterHealth.NumberOfNodes,
		TotalDataNodes: clusterHealth.NumberOfDataNodes,
	}
	healthData.ShardStats = ShardStatsInfo{
		ActivePrimary: clusterHealth.ActivePrimaryShards,
		Active:        clusterHealth.ActiveShards,
		Relocating:    clusterHealth.RelocatingShards,
		Initializing:  clusterHealth.InitializingShards,
		Unassigned:    clusterHealth.UnassignedShards,
		ActivePercent: clusterHealth.ActiveShardsPercentAsNumber,
	}

	// Alarm for cluster status
	if clusterHealth.Status == "red" {
		common.AlarmCheckDown("elasticsearch_health",
			fmt.Sprintf("Elasticsearch cluster status is RED. Name: %s", clusterHealth.ClusterName), false, "", "")
	} else if clusterHealth.Status == "yellow" {
		common.AlarmCheckDown("elasticsearch_health", // Use a different key or manage severity for yellow
			fmt.Sprintf("Elasticsearch cluster status is YELLOW. Name: %s. Unassigned Shards: %d", clusterHealth.ClusterName, clusterHealth.UnassignedShards), false, "", "")
	} else {
		common.AlarmCheckUp("elasticsearch_health",
			fmt.Sprintf("Elasticsearch cluster status is GREEN. Name: %s", clusterHealth.ClusterName), false)
	}

	// Check shard allocation only if there are unassigned shards or status is not green
	if clusterHealth.UnassignedShards > 0 || clusterHealth.Status != "green" {
		allocationInfo, allocErr := getShardAllocation()
		if allocErr != nil {
			allocationErrorMsg := fmt.Sprintf("Failed to get shard allocation details: %v", allocErr)
			common.LogError(allocationErrorMsg)
			if healthData.Error != "" {
				healthData.Error += "; " + allocationErrorMsg
			} else {
				healthData.Error = allocationErrorMsg
			}
			// healthData.Allocation will remain nil or as previously set if error occurs
		} else {
			healthData.Allocation = allocationInfo // Can be nil if no issues (e.g. 400 response)
		}
	} else {
		// If no unassigned shards and status is green, allocation is considered OK.
		healthData.Allocation = &AllocationInfo{
			CanAllocate:   "OK",
			IsProblematic: false,
			Explanation:   "All shards assigned, cluster is green.",
		}
		common.AlarmCheckUp("elasticsearch_shard_allocation", "All shards assigned and cluster is green.", false)
	}

	// Specific alarm for problematic allocation if details were fetched
	if healthData.Allocation != nil && healthData.Allocation.IsProblematic {
		message := fmt.Sprintf("Elasticsearch shard allocation issue. Index: %s, Shard: %d, Reason: %s, Explanation: %s",
			healthData.Allocation.Index, healthData.Allocation.Shard, healthData.Allocation.UnassignedReason, healthData.Allocation.Explanation)
		common.AlarmCheckDown("elasticsearch_shard_allocation", message, false, "", "")
	} else if healthData.Allocation != nil && !healthData.Allocation.IsProblematic && (clusterHealth.UnassignedShards == 0 && clusterHealth.Status == "green") {
		// This case is covered above, but ensures alarm is up if we did fetch allocation and it was fine.
		common.AlarmCheckUp("elasticsearch_shard_allocation", "Elasticsearch shard allocation is healthy.", false)
	}

	return healthData
}

// displayBoxUI displays the health data in a nice box UI.
func displayBoxUI(healthData *EsHealthData, version string) {
	title := fmt.Sprintf("monokit esHealth v%s", version)
	content := healthData.RenderAll() // From ui.go

	renderedBox := common.DisplayBox(title, content)
	fmt.Println(renderedBox)
}

// getClusterHealth fetches the Elasticsearch cluster health status.
// It returns a ClusterHealthAPIResponse struct or an error.
func getClusterHealth() (*ClusterHealthAPIResponse, error) {
	common.LogFunctionEntry()
	url := Config.Api_url

	// Make sure URL ends with "/"
	if url[len(url)-1:] != "/" {
		url = url + "/"
	}
	healthEndpoint := url + "_cluster/health?pretty"

	req, err := http.NewRequest("GET", healthEndpoint, nil)
	if err != nil {
		msg := fmt.Sprintf("Error creating request for cluster health: %v", err)
		common.AlarmCheckDown("elasticsearch_connection", msg, false, "", "")
		return nil, fmt.Errorf(msg)
	}

	if Config.User != "" && Config.Password != "" {
		req.SetBasicAuth(Config.User, Config.Password)
	}
	common.AddUserAgent(req)

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		msg := fmt.Sprintf("Error connecting to Elasticsearch for cluster health: %v", err)
		common.AlarmCheckDown("elasticsearch_connection", msg, false, "", "")
		return nil, fmt.Errorf(msg)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("Error reading cluster health response body: %v", err)
		common.AlarmCheckDown("elasticsearch_connection", msg, false, "", "")
		return nil, fmt.Errorf(msg)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Elasticsearch cluster health check failed with status code %d: %s", resp.StatusCode, string(body))
		// This specific alarm is handled by the caller based on status (red/yellow)
		// common.AlarmCheckDown("elasticsearch_health_api", msg, false, "", "")
		return nil, fmt.Errorf(msg)
	}

	common.AlarmCheckUp("elasticsearch_connection", "Successfully connected to Elasticsearch for cluster health.", false)

	var healthResponse ClusterHealthAPIResponse // From types.go
	if err := json.Unmarshal(body, &healthResponse); err != nil {
		msg := fmt.Sprintf("Error parsing cluster health JSON response: %v. Body: %s", err, string(body))
		// common.AlarmCheckDown("elasticsearch_health_api", msg, false, "", "") // Or a parsing specific alarm
		return nil, fmt.Errorf(msg)
	}
	return &healthResponse, nil
}

// getShardAllocation fetches shard allocation status using _cluster/allocation/explain API.
// It returns an AllocationInfo struct or an error.
func getShardAllocation() (*AllocationInfo, error) {
	common.LogFunctionEntry()
	url := Config.Api_url

	if url[len(url)-1:] != "/" {
		url = url + "/"
	}
	allocationEndpoint := url + "_cluster/allocation/explain?pretty"

	req, err := http.NewRequest("GET", allocationEndpoint, nil)
	if err != nil {
		msg := fmt.Sprintf("Error creating request for shard allocation: %v", err)
		common.AlarmCheckDown("elasticsearch_allocation_connection", msg, false, "", "")
		return nil, fmt.Errorf(msg)
	}

	if Config.User != "" && Config.Password != "" {
		req.SetBasicAuth(Config.User, Config.Password)
	}
	common.AddUserAgent(req)

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		msg := fmt.Sprintf("Error connecting to Elasticsearch for shard allocation: %v", err)
		common.AlarmCheckDown("elasticsearch_allocation_connection", msg, false, "", "")
		return nil, fmt.Errorf(msg)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("Error reading shard allocation response body: %v", err)
		common.AlarmCheckDown("elasticsearch_allocation_connection", msg, false, "", "")
		return nil, fmt.Errorf(msg)
	}

	// If the status code is 400, it typically means there are no unassigned shards to explain.
	// This is considered a "good" state for this specific endpoint call.
	if resp.StatusCode == http.StatusBadRequest {
		common.LogDebug("Shard allocation explain returned 400, likely no unassigned shards.")
		common.AlarmCheckUp("elasticsearch_allocation_connection", "Successfully connected for shard allocation (400 implies no unassigned).", false)
		return &AllocationInfo{
			CanAllocate:   "OK", // Representing no issues found by this endpoint
			IsProblematic: false,
			Explanation:   "No unassigned shards to explain (API returned 400).",
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Elasticsearch shard allocation check failed with status code %d: %s", resp.StatusCode, string(body))
		// Alarm for this is handled by the caller based on IsProblematic
		return nil, fmt.Errorf(msg)
	}
	common.AlarmCheckUp("elasticsearch_allocation_connection", "Successfully connected to Elasticsearch for shard allocation.", false)

	var apiResponse ShardAllocationAPIResponse // From types.go
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("error parsing shard allocation JSON response: %v. Body: %s", err, string(body))
	}

	allocInfo := &AllocationInfo{
		CanAllocate:   apiResponse.CanAllocate,
		Explanation:   apiResponse.AllocateExplanation,
		Index:         apiResponse.Index,
		Shard:         apiResponse.Shard,
		Primary:       apiResponse.Primary,
		CurrentState:  apiResponse.CurrentState,
		IsProblematic: apiResponse.CanAllocate == "no" || apiResponse.CanAllocate == "throttle",
	}
	if apiResponse.UnassignedInfo != nil {
		allocInfo.UnassignedReason = apiResponse.UnassignedInfo.Reason
		allocInfo.UnassignedAt = apiResponse.UnassignedInfo.At.Format(time.RFC3339)
		allocInfo.IsProblematic = true // If UnassignedInfo is present, it's a problem
	}

	return allocInfo, nil
}
