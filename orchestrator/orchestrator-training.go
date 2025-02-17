package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type GroupOutput struct {
	Output string `json:"output"`
	// we give it the advantage instead of the reward
	// so that the orchestrator could output a biased group advantage
	// (not sure why we would want to do that yet though...)
	Advantage float64 `json:"advantage"`
}

type TrainingDataGroup struct {
	// This relies on the uniqueness of the root commit graph node id
	// (generated as a uuid) to be universally unique.
	// This is a different name so that if I find issues with using the node id, I can change this.
	GroupID TrainingGroupID `json:"group_id"`
	Prompt  string          `json:"prompt"`
	Outputs []GroupOutput   `json:"outputs"`
}

const RedisTrainingTxChan = "training:data-chan"
const RedisTrainingRxChan = "training:request-chan"
const RedisTrainingAdvList = "training:advertisement-list"

func addTrainingAdvertisements(ctx context.Context, rdb *redis.Client, advertisements []TrainingGroupID) error {
	for _, advertisement := range advertisements {
		// Lpushed with the expectation that the trainer will scan r(0)->l
		fmt.Println("adding advertisement", advertisement)
		err := rdb.LPush(ctx, RedisTrainingAdvList, string(advertisement)).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func readNextTrainingRequest(ctx context.Context, rdb *redis.Client) (TrainingGroupID, error) {
	request, err := rdb.BRPop(ctx, 3*time.Second, RedisTrainingRxChan).Result()
	if err != nil {
		return "", err
	}
	return TrainingGroupID(request[1]), nil
}

func dropTrainingChans(ctx context.Context, rdb *redis.Client) error {
	fmt.Println("dropping training chans")
	err := rdb.Del(ctx, RedisTrainingTxChan).Err()
	if err != nil {
		return err
	}
	err = rdb.Del(ctx, RedisTrainingRxChan).Err()
	if err != nil {
		return err
	}
	err = rdb.Del(ctx, RedisTrainingAdvList).Err()
	if err != nil {
		return err
	}
	return nil
}

func sendTrainingDataGroup(ctx context.Context, rdb *redis.Client, data TrainingDataGroup) error {
	json, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// TODO: gzip -- this is highly compressible
	return rdb.LPush(ctx, RedisTrainingTxChan, json).Err()
}
