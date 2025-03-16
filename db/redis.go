package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PooriaJ/RediDNS/config"
	"github.com/PooriaJ/RediDNS/models"
	"github.com/go-redis/redis/v8"
)

// RedisClient wraps the Redis client with DNS server specific operations
type RedisClient struct {
	client *redis.Client
	cfg    *config.Config
}

// NewRedisClient creates a new Redis client
func NewRedisClient(ctx context.Context, cfg *config.Config) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client, cfg: cfg}, nil
}

// Close closes the Redis client connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetRecordsByNameAndType retrieves multiple DNS records from Redis cache
func (r *RedisClient) GetRecordsByNameAndType(ctx context.Context, zone, name string, recordType models.RecordType) ([]models.Record, error) {
	key := fmt.Sprintf("dns:records:%s:%s:%s", zone, name, recordType)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, nil // Records not found in cache or error
	}

	var records []models.Record
	err = json.Unmarshal(data, &records)
	return records, err
}

// SetRecords stores multiple DNS records in Redis cache
func (r *RedisClient) SetRecords(ctx context.Context, records []models.Record, ttl time.Duration) error {
	if len(records) == 0 {
		return nil
	}

	// All records should have the same zone, name, and type
	zone := records[0].Zone
	name := records[0].Name
	recordType := records[0].Type

	key := fmt.Sprintf("dns:records:%s:%s:%s", zone, name, recordType)
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}

	// If config specifies TTL=0, cache forever (no expiration)
	if r.cfg.Redis.Cache.TTL == 0 {
		return r.client.Set(ctx, key, data, 0).Err()
	}

	// Always use the configured cache TTL from config
	// TTL in config is in seconds, convert to time.Duration
	cacheTTL := time.Duration(r.cfg.Redis.Cache.TTL) * time.Second

	return r.client.Set(ctx, key, data, cacheTTL).Err()
}

// DeleteRecordsByNameAndType removes multiple DNS records from Redis cache
func (r *RedisClient) DeleteRecordsByNameAndType(ctx context.Context, zone, name string, recordType models.RecordType) error {
	key := fmt.Sprintf("dns:records:%s:%s:%s", zone, name, recordType)
	return r.client.Del(ctx, key).Err()
}

// GetRecord retrieves a DNS record from Redis cache
func (r *RedisClient) GetRecord(ctx context.Context, zone, name string, recordType models.RecordType) (*models.Record, error) {
	key := fmt.Sprintf("dns:record:%s:%s:%s", zone, name, recordType)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Record not found in cache
		}
		return nil, err
	}

	var record models.Record
	err = json.Unmarshal(data, &record)
	return &record, err
}

// SetRecord stores a DNS record in Redis cache
func (r *RedisClient) SetRecord(ctx context.Context, record *models.Record, ttl time.Duration) error {
	key := fmt.Sprintf("dns:record:%s:%s:%s", record.Zone, record.Name, record.Type)
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	// If config specifies TTL=0, cache forever (no expiration)
	if r.cfg.Redis.Cache.TTL == 0 {
		return r.client.Set(ctx, key, data, 0).Err()
	}

	// Always use the configured cache TTL from config
	// TTL in config is in seconds, convert to time.Duration
	cacheTTL := time.Duration(r.cfg.Redis.Cache.TTL) * time.Second

	return r.client.Set(ctx, key, data, cacheTTL).Err()
}

// DeleteRecord removes a DNS record from Redis cache
func (r *RedisClient) DeleteRecord(ctx context.Context, zone, name string, recordType models.RecordType) error {
	key := fmt.Sprintf("dns:record:%s:%s:%s", zone, name, recordType)
	return r.client.Del(ctx, key).Err()
}

// GetRecordsByZone retrieves all records for a specific zone
func (r *RedisClient) GetRecordsByZone(ctx context.Context, zone string) ([]models.Record, error) {
	pattern := fmt.Sprintf("dns:record:%s:*", zone)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var records []models.Record
	for _, key := range keys {
		data, err := r.client.Get(ctx, key).Bytes()
		if err != nil {
			continue // Skip records that can't be retrieved
		}

		var record models.Record
		if err := json.Unmarshal(data, &record); err != nil {
			continue // Skip records that can't be unmarshaled
		}

		records = append(records, record)
	}

	return records, nil
}

// PublishRecordUpdate publishes a record update event
func (r *RedisClient) PublishRecordUpdate(ctx context.Context, record *models.Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return r.client.Publish(ctx, "dns:record:update", data).Err()
}

// SubscribeToRecordUpdates subscribes to record update events
func (r *RedisClient) SubscribeToRecordUpdates(ctx context.Context) *redis.PubSub {
	return r.client.Subscribe(ctx, "dns:record:update")
}

// Keys returns keys matching the pattern
func (r *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

// Del deletes keys
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}
