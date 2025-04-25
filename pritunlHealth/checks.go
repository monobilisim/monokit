// pritunlHealth/checks.go
// This file contains the functions used in the pritunlHealth package
//
// It provides the following functions:
// - ClientUpCheck: Checks if a client is online
// - OrgCheck: Checks if an organization is online
// - UsersStatus: Checks the status of all users in the database and sends alerts
// - ServerStatus: Checks the status of all servers in the database and sends alerts
package pritunlHealth

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"github.com/monobilisim/monokit/common"
)

// ClientUpCheck checks if a client is online
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
			fmt.Println("Error: " + err.Error())
			return []Client{}
		}

		var userId bson.ObjectID

		// Get user_id
		userId = result["user_id"].(bson.ObjectID)

		// Get IP address
		ipAddr := result["real_address"].(string)

		if userId == userIdActual {
			res = append(res, Client{userId, ipAddr})
		}
	}

	return res
}

// OrgCheck checks if an organization is online
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
			fmt.Println("Error: " + err.Error())
			return false
		}

		if result["name"] == nil || result["_id"] == nil {
			continue
		}

		// Get id
		id := result["_id"]

		// Get name
		name := result["name"].(string)

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

// UsersStatus checks the status of all users in the database and sends alerts
func UsersStatus(ctx context.Context, db *mongo.Database) {
	// Get to the users collection
	collection := db.Collection("users")

	common.SplitSection("User Status")

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
			fmt.Println("Error: " + err.Error())
			return
		}

		name := result["name"].(string)
		if name == "" || name == "undefined" {
			continue
		}

		// get org_id
		orgId := OrgCheck(result["org_id"].(bson.ObjectID), ctx, db)

		if orgId == false {
			continue
		}

		// Get id
		isUp := ClientUpCheck(result["_id"].(bson.ObjectID), ctx, db)

		var addresses []string
		var addressesStr string

		for _, realAddr := range isUp {
			addresses = append(addresses, realAddr.Real_address)
		}

		if len(addresses) > 0 {
			addressesStr = strings.Join(addresses, ", ")
		}

		if len(isUp) == 0 {
			fmt.Println(common.Blue + "User " + name + " is " + common.Fail + "offline" + common.Reset)
			common.AlarmCheckDown("user_"+name, "User "+name+" is offline, no client is connected", false, "", "")
		} else {
			common.PrettyPrintStr("User "+name, true, "online")
			common.AlarmCheckUp("user_"+name, "User "+name+" is now online, "+fmt.Sprint(len(isUp))+" client(s) is/are connected with IP(s): "+addressesStr, false)
		}
	}
}

// ServerStatus checks the status of all servers in the database and sends alerts
func ServerStatus(ctx context.Context, db *mongo.Database) {
	// Get to the servers collection
	collection := db.Collection("servers")

	common.SplitSection("Server Status")

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
			fmt.Println("Error: " + err.Error())
			return
		}

		// Get status
		status := result["status"].(string)

		if status != "online" {
			common.PrettyPrintStr("Server "+result["name"].(string), false, "online")
			common.AlarmCheckDown("server_"+result["name"].(string), "Server "+result["name"].(string)+" is down, status '"+status+"'", false, "", "")
		} else {
			common.PrettyPrintStr("Server "+result["name"].(string), true, "online")
			common.AlarmCheckUp("server_"+result["name"].(string), "Server "+result["name"].(string)+" is now up, status '"+status+"'", false)
		}
	}
}
