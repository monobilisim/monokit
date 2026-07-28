//go:build linux

package mongodbHealth

import (
	"context"
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	db "github.com/monobilisim/monokit/common/db"
	"github.com/spf13/cobra"
)

var DbHealthConfig db.DbHealth
var healthData *MongoHealthData

// DetectMongoDB checks whether this host has a usable MongoDB configuration
// by loading db.yaml and attempting a real connect+ping.
func DetectMongoDB() bool {
	if !common.ConfExists("db") {
		return false
	}

	var detectConf db.DbHealth
	common.ConfInit("db", &detectConf)

	if detectConf.Mongodb.Uri == "" {
		return false
	}

	mongoClient, err := connectMongo(detectConf.Mongodb.Uri, detectConf.Mongodb.Connect_timeout_seconds)
	if err != nil {
		return false
	}
	disconnectMongo(mongoClient)

	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "mongodbHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectMongoDB,
	})
}

func Main(cmd *cobra.Command, args []string) {
	version := "1.0.0"
	common.ScriptName = "mongodbHealth"
	common.TmpDir = common.TmpDir + "mongodbHealth"
	common.Init()

	common.ConfInit("db", &DbHealthConfig)

	healthData = NewMongoHealthData()
	healthData.Version = version

	client.WrapperGetServiceStatus("mongodbHealth")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mongoClient, err := connectMongo(DbHealthConfig.Mongodb.Uri, DbHealthConfig.Mongodb.Connect_timeout_seconds)
	if err != nil {
		msg := fmt.Sprintf("Failed to connect to MongoDB: %v", err)
		common.AlarmCheckDown("ping", msg, false, "", "")
		healthData.ConnectionInfo.Connected = false
		healthData.ConnectionInfo.Error = err.Error()
		fmt.Println(healthData.RenderAll())
		return
	}
	defer disconnectMongo(mongoClient)

	common.AlarmCheckUp("ping", "MongoDB connection is up", false)
	healthData.ConnectionInfo.Connected = true

	CheckStandalone(ctx, mongoClient)

	if IsReplicaSet(ctx, mongoClient) {
		healthData.IsReplicaSet = true
		CheckReplicaSet(ctx, mongoClient)
	}

	fmt.Println(healthData.RenderAll())
}
