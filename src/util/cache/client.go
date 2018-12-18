package cache

import (
	"encoding/json"
	"time"

	"github.com/go-redis/redis"
)

type Cache interface {
	Get(key string) (string, error)
	GetJson(key string, schemaPointer interface{}) error
	Set(key string, value interface{}) error
	SetWithExpiration(key string, value interface{}, exp time.Duration) error
	Remove(key string) error
	Exists(key string) (bool, error)
}

type RedisCacheClient struct {
	client *redis.Client
}

func MakeRedisCache(address, password string) *RedisCacheClient {
	cacheClient := RedisCacheClient{}

	cacheClient.client = redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       0,
	})

	return &cacheClient
}

func (config *RedisCacheClient) Get(key string) (string, error) {
	return config.client.Get(key).Result()
}

func (config *RedisCacheClient) GetJson(key string, schemaPointer interface{}) error {
	jsonString, err := config.client.Get(key).Result()

	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(jsonString), schemaPointer)
}

func (config *RedisCacheClient) Set(key string, value interface{}) error {
	return config.client.Set(key, value, time.Hour*24).Err()
}

func (config *RedisCacheClient) SetWithExpiration(key string, value interface{}, exp time.Duration) error {
	return config.client.Set(key, value, exp).Err()
}

func (config *RedisCacheClient) Remove(key string) error {
	return config.client.Del(key).Err()
}

func (config *RedisCacheClient) Exists(key string) (bool, error) {
	v, err := config.client.Exists(key).Result()

	if v == 1 {
		return true, err
	} else {
		return false, err
	}
}
