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

var Config struct {
	Api_url  string
	User     string
	Password string
}

// ShardAllocationResponse represents the response from the /_cluster/allocation/explain API
type ShardAllocationResponse struct {
	Index               string          `json:"index"`
	Shard               int             `json:"shard"`
	Primary             bool            `json:"primary"`
	CurrentState        string          `json:"current_state"`
	UnassignedInfo      *UnassignedInfo `json:"unassigned_info,omitempty"`
	CanAllocate         string          `json:"can_allocate"`
	AllocateExplanation string          `json:"allocate_explanation"`
}

// UnassignedInfo contains information about why a shard is unassigned
type UnassignedInfo struct {
	Reason               string    `json:"reason"`
	At                   time.Time `json:"at"`
	LastAllocationStatus string    `json:"last_allocation_status,omitempty"`
}

func Main(cmd *cobra.Command, args []string) {
	version := "0.1.0"
	common.ScriptName = "esHealth"
	common.TmpDir = common.TmpDir + "esHealth"
	common.Init()
	common.ConfInit("es", &Config)

	fmt.Println("esHealth - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))
	api.WrapperGetServiceStatus("esHealth")

	// Check Elasticsearch cluster health
	checkElasticsearchHealth()

	// Check shard allocation
	checkShardAllocation()
}

// checkElasticsearchHealth checks the Elasticsearch cluster health status
// by making a request to /_cluster/health?pretty endpoint
func checkElasticsearchHealth() {
	common.LogFunctionEntry()

	url := Config.Api_url
	if url == "" {
		common.LogError("API URL is not configured")
		return
	}

	// Make sure URL ends with "/"
	if url[len(url)-1:] != "/" {
		url = url + "/"
	}

	healthEndpoint := url + "_cluster/health?pretty"

	// Create HTTP request
	req, err := http.NewRequest("GET", healthEndpoint, nil)
	if err != nil {
		common.LogError("Error creating request: " + err.Error())
		common.AlarmCheckDown("elasticsearch_health",
			"Failed to create request to Elasticsearch health endpoint", false, "", "")
		return
	}

	// Add basic auth if credentials are provided
	if Config.User != "" && Config.Password != "" {
		req.SetBasicAuth(Config.User, Config.Password)
	}

	// Add user agent
	common.AddUserAgent(req)

	// Execute request
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
	resp, err := client.Do(req)

	if err != nil {
		common.LogError("Error connecting to Elasticsearch: " + err.Error())
		common.AlarmCheckDown("elasticsearch_health",
			"Cannot connect to Elasticsearch: "+err.Error(), false, "", "")
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		common.LogError("Error reading response body: " + err.Error())
		common.AlarmCheckDown("elasticsearch_health",
			"Failed to read Elasticsearch health response", false, "", "")
		return
	}

	// Check status code
	if resp.StatusCode == 200 {
		common.PrettyPrintStr("Elasticsearch Health", true, "good, status code: 200")
		common.AlarmCheckUp("elasticsearch_health",
			"Elasticsearch cluster is healthy", false)
	} else {
		common.LogError("Elasticsearch health check failed with status code: " +
			fmt.Sprintf("%d", resp.StatusCode))
		common.PrettyPrintStr("Elasticsearch Health", false, "good, status code: "+fmt.Sprintf("%d", resp.StatusCode))
		common.AlarmCheckDown("elasticsearch_health",
			fmt.Sprintf("Elasticsearch health check failed with status code %d: %s",
				resp.StatusCode, string(body)), false, "", "")
	}
}

// checkShardAllocation checks shard allocation status using _cluster/allocation/explain API
func checkShardAllocation() {
	common.LogFunctionEntry()

	url := Config.Api_url
	if url == "" {
		common.LogError("API URL is not configured")
		return
	}

	// Make sure URL ends with "/"
	if url[len(url)-1:] != "/" {
		url = url + "/"
	}

	allocationEndpoint := url + "_cluster/allocation/explain?pretty"

	// Create HTTP request
	req, err := http.NewRequest("GET", allocationEndpoint, nil)
	if err != nil {
		common.LogError("Error creating request: " + err.Error())
		common.AlarmCheckDown("elasticsearch_shard_allocation",
			"Failed to create request to Elasticsearch allocation endpoint", false, "", "")
		return
	}

	// Add basic auth if credentials are provided
	if Config.User != "" && Config.Password != "" {
		req.SetBasicAuth(Config.User, Config.Password)
	}

	// Add user agent
	common.AddUserAgent(req)

	// Execute request
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
	resp, err := client.Do(req)

	if err != nil {
		common.LogError("Error connecting to Elasticsearch: " + err.Error())
		common.AlarmCheckDown("elasticsearch_shard_allocation",
			"Cannot connect to Elasticsearch for shard allocation check: "+err.Error(), false, "", "")
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		common.LogError("Error reading response body: " + err.Error())
		common.AlarmCheckDown("elasticsearch_shard_allocation",
			"Failed to read Elasticsearch allocation response", false, "", "")
		return
	}

	// If the status code is 400, it may mean there are no unassigned shards to explain
	if resp.StatusCode == 400 {
		common.PrettyPrintStr("Elasticsearch Shard Allocation", true, common.Green+"OK"+common.Reset)
		common.AlarmCheckUp("elasticsearch_shard_allocation",
			"No unassigned shards detected", false)
		return
	}

	// Parse JSON response
	var allocationResponse ShardAllocationResponse
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		common.LogError("Error parsing JSON response: " + err.Error())
		common.AlarmCheckDown("elasticsearch_shard_allocation",
			"Failed to parse Elasticsearch allocation response: "+err.Error(), false, "", "")
		return
	}

	// Check for allocation issues
	if allocationResponse.Shard == 0 && allocationResponse.CanAllocate == "no" {
		message := fmt.Sprintf("Shards cannot be allocated. Shard: %d, Index: %s, Explanation: %s",
			allocationResponse.Shard, allocationResponse.Index, allocationResponse.AllocateExplanation)
		common.LogError(message)
		common.PrettyPrintStr("Elasticsearch Shard Allocation", false, common.Fail+"FAILED"+common.Reset)
		common.AlarmCheckDown("elasticsearch_shard_allocation", message, false, "", "")
	} else {
		common.PrettyPrintStr("Elasticsearch Shard Allocation", true, common.Green+"OK"+common.Reset)
		common.AlarmCheckUp("elasticsearch_shard_allocation",
			"Elasticsearch shard allocation is healthy", false)
	}
}
