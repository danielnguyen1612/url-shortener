package libs

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	keyRedisAddr = "redis.addr"
	keyRedisUser = "redis.user"
	keyRedisPwd  = "redis.pwd"
)

func NewRedisFromViper(log *zap.Logger) (*redis.Client, error) {
	for _, key := range []string{keyRedisAddr} {
		if len(viper.GetString(key)) == 0 {
			return nil, errors.New(fmt.Sprintf("%s must be provided", key))
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     viper.GetString(keyRedisAddr),
		Username: viper.GetString(keyRedisUser),
		Password: viper.GetString(keyRedisPwd),
	})

	// Ping server to check connection
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, errors.Wrap(err, "client.Ping")
	}

	return client, nil
}
