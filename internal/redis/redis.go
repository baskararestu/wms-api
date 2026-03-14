package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/baskararestu/wms-api/internal/config"
	"github.com/baskararestu/wms-api/internal/pkg/xlogger"
	goredis "github.com/redis/go-redis/v9"
)

var Client *goredis.Client
var Ctx = context.Background()

// ConnectRedis initializes the connection to the Redis server
func ConnectRedis() {
	addr := fmt.Sprintf("%s:%s", config.App.RedisHost, config.App.RedisPort)

	Client = goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: config.App.RedisPassword,
		DB:       0, // Default DB
		PoolSize: 10,
	})

	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	defer cancel()

	_, err := Client.Ping(ctx).Result()
	if err != nil {
		xlogger.Logger.Fatal().Err(err).Msg("Fatal: Could not connect to Redis")
	}

	xlogger.Logger.Info().Msg("✅ Successfully connected to Redis!")
}
