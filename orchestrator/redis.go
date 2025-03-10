package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

func ConnectToRedis(ctx context.Context) (*redis.Client, error) {
	host := os.Getenv("REDIS_ADDRESS")
	port := os.Getenv("REDIS_PORT")
	addr := fmt.Sprintf("%s:%s", host, port)
	pw := os.Getenv("REDIS_PASSWORD")
	if host == "" || port == "" || pw == "" {
		return nil, errors.New("REDIS_ADDRESS, REDIS_PORT, and REDIS_PASSWORD must be set")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pw,
		DB:       0,
		Protocol: 3,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return rdb, nil
}
