package pritunlHealth

import (
    "fmt"
    "time"
    "slices"
	"context"
    "strings"
    "github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
    "github.com/monobilisim/monokit/common"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "go.mongodb.org/mongo-driver/v2/mongo/readpref"
)


type PritunlHealth struct {
	Url string
    Allowed_orgs []string
}

type Client struct {
    User_id bson.ObjectID
    Real_address string
}

var PritunlHealthConfig PritunlHealth

func Main(cmd *cobra.Command, args []string) {
    version := "1.0.0"
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
		common.AlarmCheckDown("pritunl_connect", "Couldn't connect to the server: " + err.Error(), false, "", "")
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
		common.AlarmCheckDown("pritunl_ping", "Couldn't ping the server: " + err.Error(), false, "", "")
		return
	} else {
		common.AlarmCheckUp("pritunl_ping", "Server is now pingable", false)
	}

	// Get to the pritunl database
	db := client.Database("pritunl")

    ServerStatus(ctx, db)
    UsersStatus(ctx, db)
}



