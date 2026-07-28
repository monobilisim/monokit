//go:build linux

package mongodbHealth

import (
	"context"
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	db "github.com/monobilisim/monokit/common/db"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
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

	isRS, rsErr := IsReplicaSet(ctx, mongoClient)
	rsEnabled := DbHealthConfig.Mongodb.Replicaset.Enabled
	if isRS {
		healthData.IsReplicaSet = true
		CheckReplicaSet(ctx, mongoClient)
		if rsEnabled {
			common.AlarmCheckUp("mongodb-replicaset-mismatch", "Node confirmed as replica set member as expected", false)
			issues.CheckUp("mongodb-replicaset-mismatch", "MongoDB replica set durumu doğrulandı")
		}
	} else if rsEnabled {
		// Expected replica set membership (replicaset.enabled=true) but
		// couldn't confirm it - either genuinely standalone or an
		// auth/permission error prevented the check. Either way this is a
		// mismatch and must always alarm.
		var msg string
		if isAuthError(rsErr) {
			msg = fmt.Sprintf("mongodb.replicaset.enabled=true but replSetGetStatus failed due to insufficient permissions: %v", rsErr)
			appendPermissionWarning(fmt.Sprintf("Could not determine replica set status: insufficient permissions (%v)", rsErr))
		} else {
			msg = "mongodb.replicaset.enabled=true but node reports as standalone (not part of a replica set)"
		}
		subject := fmt.Sprintf("%s için MongoDB replica set durumu doğrulanamıyor", common.Config.Identifier)
		common.AlarmCheckDown("mongodb-replicaset-mismatch", msg, false, "", "")
		issues.CheckDown("mongodb-replicaset-mismatch", subject, msg, false, 0)
	} else if isAuthError(rsErr) {
		appendPermissionWarning(fmt.Sprintf("Could not determine replica set status: insufficient permissions (%v)", rsErr))
	}

	fmt.Println(healthData.RenderAll())
}
