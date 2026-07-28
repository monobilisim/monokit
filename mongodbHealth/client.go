//go:build linux

package mongodbHealth

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// connectMongo connects to the given MongoDB URI and pings it to verify
// reachability. The caller is responsible for calling disconnectMongo when
// done.
func connectMongo(uri string, timeoutSeconds int) (*mongo.Client, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 5
	}

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}

	return client, nil
}

// isAuthError reports whether err indicates the current connection lacks
// sufficient privileges to run a command (e.g. serverStatus,
// replSetGetStatus), as opposed to a network/connectivity failure.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.Code == 13 // Unauthorized
	}
	return strings.Contains(err.Error(), "Unauthorized")
}

// appendPermissionWarning adds msg to healthData.PermissionWarning, appending
// to any existing warning instead of overwriting it (standalone and replica
// set checks can each independently hit a permission error in the same run).
func appendPermissionWarning(msg string) {
	if healthData.PermissionWarning == "" {
		healthData.PermissionWarning = msg
		return
	}
	healthData.PermissionWarning = healthData.PermissionWarning + "; " + msg
}

// disconnectMongo closes the connection, ignoring nil clients.
func disconnectMongo(client *mongo.Client) {
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = client.Disconnect(ctx)
}
