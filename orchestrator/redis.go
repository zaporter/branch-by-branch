package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

func connectToRedis(ctx context.Context) (*redis.Client, error) {
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

/*
func drainQueue(ctx context.Context, rdb *redis.Client, queue string) error {
	for {
		cmd := rdb.RPop(ctx, queue)
		if cmd.Err() != nil {
			if errors.Is(cmd.Err(), redis.Nil) {
				// Queue is empty
				log.Fatalln("exit 2")
				break
			}
			return cmd.Err()
		}

		value, err := cmd.Result()
		if err != nil {
			return err
		}

		if value == "" {
			// The queue is empty, and no more items can be popped
			log.Fatalln("exit 1")
			break
		}
	}
	return nil
}
*/
