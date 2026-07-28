//go:build linux

package mongodbHealth

import (
	"context"
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

// disconnectMongo closes the connection, ignoring nil clients.
func disconnectMongo(client *mongo.Client) {
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = client.Disconnect(ctx)
}
