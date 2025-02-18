package main

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
)

// My impl of a advertisement-based data sync method that supports multiple providers & a single consumer.
// Poorly named
// TODO: add compression
type MessageList struct {
	mu       sync.RWMutex
	messages map[string]string
}

func NewMessageList() *MessageList {
	return &MessageList{
		mu:       sync.RWMutex{},
		messages: make(map[string]string),
	}
}

func (ml *MessageList) AddAdvertisement(ctx context.Context, rdb *redis.Client, listName string, key string, val any) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	err := ml.storeJSON(key, val)
	if err != nil {
		return err
	}
	return rdb.LPush(ctx, listName, key).Err()
}

func (ml *MessageList) storeJSON(key string, value any) error {
	json, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ml.messages[key] = string(json)
	return nil
}

func (ml *MessageList) Get(key string) (string, bool) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	value, ok := ml.messages[key]
	return value, ok
}

func (ml *MessageList) Delete(key string) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	delete(ml.messages, key)
}
