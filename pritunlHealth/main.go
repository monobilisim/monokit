package pritunlHealth

import (
    "fmt"
    "time"
	"context"
    "github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
    "github.com/monobilisim/monokit/common"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "go.mongodb.org/mongo-driver/v2/mongo/readpref"
)


type PritunlHealth struct {
	Url string
}

var PritunlHealthConfig PritunlHealth

func Main(cmd *cobra.Command, args []string) {
    version := "0.1.0"
    common.ScriptName = "pritunlHealth"
    common.TmpDir = common.TmpDir + "pritunlHealth"
    common.Init()
	
	if common.ConfExists("pritunl") {
    	common.ConfInit("pritunl", &PritunlHealthConfig)
	}

	if PritunlHealthConfig.Url == "" {
		PritunlHealthConfig.Url = "mongodb://localhost:27017"
	}

    fmt.Println("Pritunl Health Check - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	client, err := mongo.Connect(options.Client().ApplyURI(PritunlHealthConfig.Url))
	if err != nil {
		common.LogError("Couldn't connect to the server: " + err.Error())
		common.AlarmCheckDown("pritunl_connect", "Couldn't connect to the server: " + err.Error(), false)
		return
	} else {
		common.AlarmCheckUp("pritunl_connect", "Server is now connected", false)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	defer func() {
    	if err = client.Disconnect(ctx); err != nil {
        	panic(err)
    	}
	}()

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		common.LogError("Couldn't ping the server: " + err.Error())
		common.AlarmCheckDown("pritunl_ping", "Couldn't ping the server: " + err.Error(), false)
		return
	} else {
		common.AlarmCheckUp("pritunl_ping", "Server is now pingable", false)
	}

	// Get to the pritunl database
	db := client.Database("pritunl")

    ServerStatus(ctx, db)
    UsersStatus(ctx, db)
}

func ClientUpCheck(userIdActual bson.ObjectID, ctx context.Context, db *mongo.Database) int {
    
    // Get to the clients collection
    collection := db.Collection("clients")

    // make a for loop to get all the clients
    cursor, err := collection.Find(ctx, bson.D{})
    if err != nil {
        common.LogError("Couldn't get the collection: " + err.Error())
        common.AlarmCheckDown("pritunl_clients", "Couldn't get the clients collection: " + err.Error(), false)
        return 0
    } else {
        common.AlarmCheckUp("pritunl_clients", "Clients collection is now available", false)
    }

    defer cursor.Close(ctx)

    counter := 0

    for cursor.Next(ctx) {
        var result bson.M
        err := cursor.Decode(&result)
        if err != nil {
            fmt.Println("Error: " + err.Error())
            return 0
        }
    
        // Get user_id
        userId := result["user_id"]

        if userId == userIdActual {
            counter++
        }
    }

    return counter
}

func UsersStatus(ctx context.Context, db *mongo.Database) {
    // Get to the users collection
    collection := db.Collection("users")

    common.SplitSection("User Status")

    // make a for loop to get all the users
    cursor, err := collection.Find(ctx, bson.D{})
    if err != nil {
        common.LogError("Couldn't get the collection: " + err.Error())
        common.AlarmCheckDown("pritunl_users", "Couldn't get the users collection: " + err.Error(), false)
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

        // Get id
        isUp := ClientUpCheck(result["_id"].(bson.ObjectID), ctx, db)

        if isUp == 0 {
            fmt.Println(common.Blue + "User " + name + " is " + common.Fail + "offline" + common.Reset)
            common.AlarmCheckDown("user_" + name, "User " + name + " is offline, no client is connected", false)
        } else {
            common.PrettyPrintStr("User " + name, true, "online")
            common.AlarmCheckUp("user_" + name, "User " + name + " is now online, " + fmt.Sprint(isUp) + " client(s) is/are connected", false)
        }
    }
}

func ServerStatus(ctx context.Context, db *mongo.Database) {
	// Get to the servers collection
	collection := db.Collection("servers")

	common.SplitSection("Server Status")

	// make a for loop to get all the servers
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		common.LogError("Couldn't get the collection: " + err.Error())
		common.AlarmCheckDown("pritunl", "Couldn't get the collection: " + err.Error(), false)
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
			common.PrettyPrintStr("Server " + result["name"].(string), false, "online")
			common.AlarmCheckDown("server_" + result["name"].(string), "Server " + result["name"].(string) + " is down, status '" + status + "'", false)
		} else {
			common.PrettyPrintStr("Server " + result["name"].(string), true, "online")
			common.AlarmCheckUp("server_" + result["name"].(string), "Server " + result["name"].(string) + " is now up, status '" + status + "'", false)
		}
	}
}
