package libs

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const (
	keyMongoUri        = "mongo.uri"
	KeyMongoDb         = "mongo.db"
	KeyMongoCollection = "mongo.collection"
)

// Initialise mongo client with config from viper
func NewMongoWithViper(log *zap.Logger) (*mongo.Client, error) {
	for _, key := range []string{keyMongoUri, KeyMongoDb, KeyMongoCollection} {
		if len(viper.GetString(key)) == 0 {
			return nil, errors.New(fmt.Sprintf("%s must be provided", key))
		}
	}

	client, err := mongo.NewClient(
		(&options.ClientOptions{}).ApplyURI(fmt.Sprintf("mongodb://%s", viper.GetString(keyMongoUri))),
	)
	if err != nil {
		return nil, errors.Wrap(err, "mongo.NewClient")
	}

	// Try to connect to mongo server
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "client.Connect")
	}

	// Try to discover mongo server
	ctx, _ = context.WithTimeout(context.Background(), 2*time.Second)
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return nil, errors.Wrap(err, "client.Ping")
	}

	return client, nil
}
