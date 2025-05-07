package pritunlHealth

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper" // Import viper for config reading in detection
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// DetectPritunl checks if the Pritunl service seems to be configured and reachable via MongoDB.
func DetectPritunl() bool {
	// 1. Try to load the configuration
	var tempConfig struct {
		Url string
	}
	// Use viper directly to avoid initializing the full common stack just for detection
	viper.SetConfigName("pritunl")
	viper.AddConfigPath("/etc/mono") // Assuming standard config path
	err := viper.ReadInConfig()
	if err != nil {
		common.LogDebug(fmt.Sprintf("pritunlHealth auto-detection failed: Cannot read config file: %v", err))
		return false
	}
	err = viper.Unmarshal(&tempConfig)
	if err != nil {
		common.LogDebug(fmt.Sprintf("pritunlHealth auto-detection failed: Cannot unmarshal config: %v", err))
		return false
	}

	// 2. Check if essential config values are present
	pritunlURL := tempConfig.Url
	if pritunlURL == "" {
		// Default URL if not specified in config, consistent with Main function logic
		pritunlURL = "mongodb://localhost:27017"
		common.LogDebug("pritunlHealth auto-detection: Using default MongoDB URL: " + pritunlURL)
	} else {
		common.LogDebug("pritunlHealth auto-detection: Found MongoDB URL in config: " + pritunlURL)
	}

	// 3. Attempt to connect and ping MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Short timeout for detection
	defer cancel()                                                          // Keep context for Ping and Disconnect

	clientOptions := options.Client().ApplyURI(pritunlURL)
	// Remove ctx from Connect call
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		common.LogDebug(fmt.Sprintf("pritunlHealth auto-detection failed: Cannot connect to MongoDB at %s: %v", pritunlURL, err))
		return false
	}
	// Ensure disconnection even if ping fails
	defer client.Disconnect(ctx)

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		common.LogDebug(fmt.Sprintf("pritunlHealth auto-detection failed: Cannot ping MongoDB at %s: %v", pritunlURL, err))
		return false
	}

	common.LogDebug(fmt.Sprintf("pritunlHealth auto-detection: Successfully connected and pinged MongoDB at %s.", pritunlURL))
	common.LogDebug("pritunlHealth auto-detected successfully.")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "pritunlHealth",
		EntryPoint: Main,
		Platform:   "any",         // Connects to MongoDB, platform-agnostic
		AutoDetect: DetectPritunl, // Add the auto-detect function
	})
}

type PritunlHealth struct {
	Url          string
	Allowed_orgs []string
}

type Client struct {
	User_id      bson.ObjectID
	Real_address string
}

var PritunlHealthConfig PritunlHealth

func Main(cmd *cobra.Command, args []string) {
	version := "1.0.0"
	common.ScriptName = "pritunlHealth"
	common.TmpDir = common.TmpDir + "pritunlHealth"
	common.Init()

	// Load config after common.Init
	if common.ConfExists("pritunl") {
		common.ConfInit("pritunl", &PritunlHealthConfig)
	}

	// Apply default URL after attempting to load config
	if PritunlHealthConfig.Url == "" {
		PritunlHealthConfig.Url = "mongodb://localhost:27017"
	}

	// Create health data structure
	healthData := NewPritunlHealthData()
	healthData.Version = version

	// Connect call does not use context in this driver version
	clientOptions := options.Client().ApplyURI(PritunlHealthConfig.Url)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		common.LogError("Couldn't connect to the server: " + err.Error())
		common.AlarmCheckDown("pritunl_connect", "Couldn't connect to the server: "+err.Error(), false, "", "")
		healthData.IsHealthy = false
		fmt.Println(healthData.RenderAll())
		return
	} else {
		common.AlarmCheckUp("pritunl_connect", "Server is now connected", false)
	}

	// Use a separate context for operations after connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Longer timeout for operations
	defer cancel()

	defer func() {
		// Use a separate context for disconnection
		ctxDisconnect, cancelDisconnect := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelDisconnect()
		if err = client.Disconnect(ctxDisconnect); err != nil {
			// Log error instead of panic
			common.LogError(fmt.Sprintf("Error disconnecting from MongoDB: %v", err))
		}
	}()

	// Use the operation context for ping
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		common.LogError("Couldn't ping the server: " + err.Error())
		common.AlarmCheckDown("pritunl_ping", "Couldn't ping the server: "+err.Error(), false, "", "")
		healthData.IsHealthy = false
		fmt.Println(healthData.RenderAll())
		return
	} else {
		common.AlarmCheckUp("pritunl_ping", "Server is now pingable", false)
	}

	// Get to the pritunl database
	db := client.Database("pritunl")

	// Collect server status
	collectServerStatus(ctx, db, healthData)

	// Collect user status
	collectUserStatus(ctx, db, healthData)

	// Collect organization status
	collectOrganizationStatus(ctx, db, healthData)

	// Set overall health status
	healthData.IsHealthy = true
	for _, server := range healthData.Servers {
		if !server.IsHealthy {
			healthData.IsHealthy = false
			break
		}
	}

	// Display the health data
	fmt.Println(healthData.RenderAll())
}

func collectServerStatus(ctx context.Context, db *mongo.Database, healthData *PritunlHealthData) {
	// Get to the servers collection
	collection := db.Collection("servers")

	// make a for loop to get all the servers
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		common.LogError("Couldn't get the collection: " + err.Error())
		common.AlarmCheckDown("pritunl", "Couldn't get the collection: "+err.Error(), false, "", "")
		return
	} else {
		common.AlarmCheckUp("pritunl", "Collection is now available", false)
	}

	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			common.LogError(fmt.Sprintf("Error decoding server document: %v", err))
			continue // Skip this document on error
		}

		var status string
		var name string

		// Safely access fields
		if statusVal, ok := result["status"].(string); ok {
			status = statusVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping server document due to missing or invalid 'status': %v", result["_id"]))
			continue
		}

		if nameVal, ok := result["name"].(string); ok {
			name = nameVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping server document due to missing or invalid 'name': %v", result["_id"]))
			continue
		}

		isHealthy := status == "online"
		serverInfo := ServerInfo{
			Name:      name,
			Status:    status,
			IsHealthy: isHealthy,
		}

		healthData.Servers = append(healthData.Servers, serverInfo)

		if !isHealthy {
			common.AlarmCheckDown("server_"+name, "Server "+name+" is down, status '"+status+"'", false, "", "")
		} else {
			common.AlarmCheckUp("server_"+name, "Server "+name+" is now up, status '"+status+"'", false)
		}
	}
}

func collectUserStatus(ctx context.Context, db *mongo.Database, healthData *PritunlHealthData) {
	// Get to the users collection
	collection := db.Collection("users")

	// make a for loop to get all the users
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		common.LogError("Couldn't get the collection: " + err.Error())
		common.AlarmCheckDown("pritunl_users", "Couldn't get the users collection: "+err.Error(), false, "", "")
		return
	} else {
		common.AlarmCheckUp("pritunl_users", "Users collection is now available", false)
	}

	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			common.LogError(fmt.Sprintf("Error decoding user document: %v", err))
			continue // Skip this document on error
		}

		var name string
		var orgIdActual bson.ObjectID
		var userIdActual bson.ObjectID

		// Safely access fields
		if nameVal, ok := result["name"].(string); ok {
			name = nameVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping user document due to missing or invalid 'name': %v", result["_id"]))
			continue
		}

		if name == "" || name == "undefined" {
			continue
		}

		if orgIDVal, ok := result["org_id"].(bson.ObjectID); ok {
			orgIdActual = orgIDVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping user '%s' due to missing or invalid 'org_id'", name))
			continue
		}

		if userIDVal, ok := result["_id"].(bson.ObjectID); ok {
			userIdActual = userIDVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping user '%s' due to missing or invalid '_id'", name))
			continue
		}

		// Check organization validity
		orgIsValid := OrgCheck(orgIdActual, ctx, db)
		if !orgIsValid {
			continue
		}

		// Get organization name
		orgName := getOrganizationName(ctx, db, orgIdActual)
		if orgName == "" {
			orgName = "Unknown Organization"
		}

		// Check client status
		connectedClients := ClientUpCheck(userIdActual, ctx, db)

		// Create user info
		userInfo := UserInfo{
			Name:             name,
			Organization:     orgName,
			Status:           "offline",
			ConnectedClients: []ClientInfo{},
			IsHealthy:        false,
		}

		if len(connectedClients) > 0 {
			userInfo.Status = "online"
			userInfo.IsHealthy = true
			for _, client := range connectedClients {
				userInfo.ConnectedClients = append(userInfo.ConnectedClients, ClientInfo{
					IPAddress: client.Real_address,
				})
			}
		}

		healthData.Users = append(healthData.Users, userInfo)

		if len(connectedClients) == 0 {
			common.AlarmCheckDown("user_"+name, "User "+name+" is offline, no client is connected", false, "", "")
		} else {
			var addresses []string
			for _, client := range connectedClients {
				addresses = append(addresses, client.Real_address)
			}
			addressesStr := strings.Join(addresses, ", ")
			common.AlarmCheckUp("user_"+name, "User "+name+" is now online, "+fmt.Sprint(len(connectedClients))+" client(s) is/are connected with IP(s): "+addressesStr, false)
		}
	}
}

func collectOrganizationStatus(ctx context.Context, db *mongo.Database, healthData *PritunlHealthData) {
	// Get to the organizations collection
	collection := db.Collection("organizations")

	// make a for loop to get all the organizations
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		common.LogError("Couldn't get the collection: " + err.Error())
		common.AlarmCheckDown("pritunl_organizations", "Couldn't get the organizations collection: "+err.Error(), false, "", "")
		return
	} else {
		common.AlarmCheckUp("pritunl_organizations", "Organizations collection is now available", false)
	}

	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			common.LogError(fmt.Sprintf("Error decoding organization document: %v", err))
			continue // Skip this document on error
		}

		var name string

		// Safely access fields
		if nameVal, ok := result["name"].(string); ok {
			name = nameVal
		} else {
			common.LogDebug("Skipping organization document due to missing or invalid 'name'")
			continue
		}

		if name == "" || name == "undefined" {
			continue
		}

		// Check if name is in the allowed_orgs
		if len(PritunlHealthConfig.Allowed_orgs) > 0 {
			if !slices.Contains(PritunlHealthConfig.Allowed_orgs, name) {
				continue
			}
		}

		orgInfo := OrganizationInfo{
			Name:     name,
			IsActive: true, // Organizations are considered active if they exist and are allowed
		}

		healthData.Organizations = append(healthData.Organizations, orgInfo)
	}
}

func getOrganizationName(ctx context.Context, db *mongo.Database, orgID bson.ObjectID) string {
	collection := db.Collection("organizations")
	var result bson.M
	err := collection.FindOne(ctx, bson.M{"_id": orgID}).Decode(&result)
	if err != nil {
		common.LogDebug(fmt.Sprintf("Error getting organization name for ID %v: %v", orgID, err))
		return ""
	}

	if name, ok := result["name"].(string); ok {
		return name
	}
	return ""
}

func ClientUpCheck(userIdActual bson.ObjectID, ctx context.Context, db *mongo.Database) []Client {
	// Get to the clients collection
	collection := db.Collection("clients")

	// make a for loop to get all the clients
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		common.LogError("Couldn't get the collection: " + err.Error())
		common.AlarmCheckDown("pritunl_clients", "Couldn't get the clients collection: "+err.Error(), false, "", "")
		return []Client{}
	} else {
		common.AlarmCheckUp("pritunl_clients", "Clients collection is now available", false)
	}

	defer cursor.Close(ctx)

	var res []Client

	for cursor.Next(ctx) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			common.LogError(fmt.Sprintf("Error decoding client document: %v", err))
			continue // Skip this document on error
		}

		var userId bson.ObjectID
		var ipAddr string

		// Safely access fields with type assertion
		if userIDVal, ok := result["user_id"].(bson.ObjectID); ok {
			userId = userIDVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping client document due to missing or invalid 'user_id': %v", result["_id"]))
			continue
		}

		if ipAddrVal, ok := result["real_address"].(string); ok {
			ipAddr = ipAddrVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping client document due to missing or invalid 'real_address': %v", result["_id"]))
			continue
		}

		if userId == userIdActual {
			res = append(res, Client{userId, ipAddr})
		}
	}

	return res
}

func OrgCheck(orgIdActual bson.ObjectID, ctx context.Context, db *mongo.Database) bool {
	// Get to the organizations collection
	collection := db.Collection("organizations")

	// make a for loop to get all the organizations
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		common.LogError("Couldn't get the collection: " + err.Error())
		common.AlarmCheckDown("pritunl_organizations", "Couldn't get the organizations collection: "+err.Error(), false, "", "")
		return false
	} else {
		common.AlarmCheckUp("pritunl_organizations", "Organizations collection is now available", false)
	}

	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			common.LogError(fmt.Sprintf("Error decoding organization document: %v", err))
			continue // Skip this document on error
		}

		var id bson.ObjectID
		var name string

		// Safely access fields
		if idVal, ok := result["_id"].(bson.ObjectID); ok {
			id = idVal
		} else {
			common.LogDebug("Skipping organization document due to missing or invalid '_id'")
			continue
		}

		if nameVal, ok := result["name"].(string); ok {
			name = nameVal
		} else {
			common.LogDebug(fmt.Sprintf("Skipping organization document due to missing or invalid 'name': %v", id))
			continue
		}

		if name == "" || name == "undefined" {
			continue
		}

		// Check if name is in the allowed_orgs
		if len(PritunlHealthConfig.Allowed_orgs) > 0 {
			if !slices.Contains(PritunlHealthConfig.Allowed_orgs, name) {
				continue
			}
		}

		if id == orgIdActual {
			return true
		}
	}

	return false
}
