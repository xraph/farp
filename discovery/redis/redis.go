// Package redis provides Redis-based service discovery and storage for FARP.
//
// It implements both ServiceDiscovery (for finding services) and StorageBackend
// (for KV-based schema storage), enabling a single Redis instance to handle
// both service discovery and schema registry.
//
// Services are stored as hash sets with EXPIRE-based TTL for automatic cleanup.
// Watches use Redis Pub/Sub for real-time event notifications.
//
// # Usage
//
// Service side:
//
//	disc, _ := redis.New(redis.Config{Address: "localhost:6379"})
//	node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
//	    ServiceName: "user-service",
//	    Address:     "10.0.0.5:8080",
//	    Discovery:   disc,
//	})
//	node.Start(ctx)
//
// Gateway side:
//
//	disc, _ := redis.New(redis.Config{Address: "localhost:6379"})
//	gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
//	    Discovery:       disc,
//	    OnRoutesChanged: updateRoutes,
//	})
//	gw.Start(ctx)
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/xraph/farp"
	"github.com/xraph/farp/discovery"
)

// Config holds Redis connection configuration.
type Config struct {
	// Address is the Redis server address (default: "localhost:6379").
	Address string
	// Password for authentication.
	Password string
	// DB is the Redis database number (default: 0).
	DB int
	// FARP namespace prefix for keys (default: "farp").
	Namespace string
	// TTL is the expiration duration for service registrations (default: 30s).
	TTL time.Duration
}

// RedisDiscovery implements discovery.ServiceDiscovery using Redis.
// It also implements farp.StorageBackend for KV operations.
type RedisDiscovery struct {
	client *goredis.Client
	config Config
	mu     sync.RWMutex
	closed bool
}

// New creates a new Redis-based discovery backend.
func New(cfg Config) (*RedisDiscovery, error) {
	if cfg.Address == "" {
		cfg.Address = "localhost:6379"
	}

	if cfg.Namespace == "" {
		cfg.Namespace = "farp"
	}

	if cfg.TTL == 0 {
		cfg.TTL = 30 * time.Second
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisDiscovery{
		client: client,
		config: cfg,
	}, nil
}

// serviceHashKey returns the Redis hash key for a service's instances.
func (r *RedisDiscovery) serviceHashKey(serviceName string) string {
	return r.config.Namespace + ":services:" + serviceName
}

// instanceKey returns the Redis key for a specific instance (used for EXPIRE).
func (r *RedisDiscovery) instanceKey(serviceName, instanceID string) string {
	return r.config.Namespace + ":instance:" + serviceName + ":" + instanceID
}

// eventChannel returns the Pub/Sub channel for service events.
func (r *RedisDiscovery) eventChannel(serviceName string) string {
	if serviceName == "" {
		return r.config.Namespace + ":events:*"
	}

	return r.config.Namespace + ":events:" + serviceName
}

// Discover returns all known instances of a service from Redis.
func (r *RedisDiscovery) Discover(ctx context.Context, serviceName string) ([]discovery.ServiceInstance, error) {
	if serviceName != "" {
		return r.discoverService(ctx, serviceName)
	}

	// Discover all services by scanning for service hash keys
	pattern := r.config.Namespace + ":services:*"
	var instances []discovery.ServiceInstance

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Extract service name from key
		svcName := strings.TrimPrefix(key, r.config.Namespace+":services:")

		svcInstances, err := r.discoverService(ctx, svcName)
		if err != nil {
			continue
		}

		instances = append(instances, svcInstances...)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	return instances, nil
}

func (r *RedisDiscovery) discoverService(ctx context.Context, serviceName string) ([]discovery.ServiceInstance, error) {
	hashKey := r.serviceHashKey(serviceName)

	result, err := r.client.HGetAll(ctx, hashKey).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	instances := make([]discovery.ServiceInstance, 0, len(result))

	for _, data := range result {
		var inst discovery.ServiceInstance
		if err := json.Unmarshal([]byte(data), &inst); err != nil {
			continue
		}

		// Check if the instance key still exists (not expired)
		exists, err := r.client.Exists(ctx, r.instanceKey(serviceName, inst.ID)).Result()
		if err != nil || exists == 0 {
			// Instance expired, clean up the hash entry
			r.client.HDel(ctx, hashKey, inst.ID)

			continue
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

// Watch watches for changes to instances of a service using Redis Pub/Sub.
func (r *RedisDiscovery) Watch(ctx context.Context, serviceName string, handler discovery.DiscoveryEventHandler) error {
	channel := r.eventChannel(serviceName)

	var pubsub *goredis.PubSub

	if serviceName == "" {
		pubsub = r.client.PSubscribe(ctx, channel)
	} else {
		pubsub = r.client.Subscribe(ctx, channel)
	}

	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			var event discovery.DiscoveryEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}

			handler(event)
		}
	}
}

// Register registers a service instance in Redis.
func (r *RedisDiscovery) Register(ctx context.Context, instance discovery.ServiceInstance) error {
	port := instance.Port
	if port == 0 {
		if _, p, ok := parseHostPort(instance.Address); ok {
			port = p
		}
	}

	instance.Port = port
	instance.RegisteredAt = time.Now()
	instance.LastHealthCheck = time.Now()

	if instance.Metadata == nil {
		instance.Metadata = make(map[string]string)
	}

	instance.Metadata["farp.enabled"] = "true"

	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal instance: %w", farp.ErrRegistrationFailed, err)
	}

	hashKey := r.serviceHashKey(instance.ServiceName)
	instKey := r.instanceKey(instance.ServiceName, instance.ID)

	// Store in the service hash
	if err := r.client.HSet(ctx, hashKey, instance.ID, string(data)).Err(); err != nil {
		return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	// Set the instance key with expiration for TTL-based cleanup
	if err := r.client.Set(ctx, instKey, "1", r.config.TTL).Err(); err != nil {
		return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	// Publish registration event
	event := discovery.DiscoveryEvent{
		Type:      farp.EventTypeAdded,
		Instance:  instance,
		Timestamp: time.Now(),
	}

	eventData, _ := json.Marshal(event)

	r.client.Publish(ctx, r.eventChannel(instance.ServiceName), string(eventData))

	return nil
}

// Deregister removes a service instance from Redis.
func (r *RedisDiscovery) Deregister(ctx context.Context, instanceID string) error {
	// Scan for the instance across all service hashes
	pattern := r.config.Namespace + ":services:*"

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		hashKey := iter.Val()

		data, err := r.client.HGet(ctx, hashKey, instanceID).Result()
		if err != nil {
			continue
		}

		// Found the instance
		var inst discovery.ServiceInstance

		if err := json.Unmarshal([]byte(data), &inst); err == nil {
			// Delete from hash
			r.client.HDel(ctx, hashKey, instanceID)

			// Delete the instance TTL key
			instKey := r.instanceKey(inst.ServiceName, instanceID)
			r.client.Del(ctx, instKey)

			// Publish removal event
			event := discovery.DiscoveryEvent{
				Type:      farp.EventTypeRemoved,
				Instance:  inst,
				Timestamp: time.Now(),
			}

			eventData, _ := json.Marshal(event)

			r.client.Publish(ctx, r.eventChannel(inst.ServiceName), string(eventData))

			return nil
		}
	}

	return fmt.Errorf("%w: instance %s", farp.ErrInstanceNotFound, instanceID)
}

// ReportHealth reports the health status of a registered instance.
func (r *RedisDiscovery) ReportHealth(ctx context.Context, instanceID string, status farp.InstanceStatus) error {
	// Find the instance in service hashes
	pattern := r.config.Namespace + ":services:*"

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		hashKey := iter.Val()

		data, err := r.client.HGet(ctx, hashKey, instanceID).Result()
		if err != nil {
			continue
		}

		var inst discovery.ServiceInstance
		if err := json.Unmarshal([]byte(data), &inst); err != nil {
			continue
		}

		inst.Status = status
		inst.LastHealthCheck = time.Now()

		updatedData, err := json.Marshal(inst)
		if err != nil {
			return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
		}

		// Update the hash entry
		if err := r.client.HSet(ctx, hashKey, instanceID, string(updatedData)).Err(); err != nil {
			return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
		}

		// Refresh the TTL on the instance key
		instKey := r.instanceKey(inst.ServiceName, instanceID)
		if err := r.client.Expire(ctx, instKey, r.config.TTL).Err(); err != nil {
			return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
		}

		// Publish update event
		event := discovery.DiscoveryEvent{
			Type:      farp.EventTypeUpdated,
			Instance:  inst,
			Timestamp: time.Now(),
		}

		eventData, _ := json.Marshal(event)

		r.client.Publish(ctx, r.eventChannel(inst.ServiceName), string(eventData))

		return nil
	}

	return fmt.Errorf("%w: instance %s not found", farp.ErrHealthCheckFailed, instanceID)
}

// Close closes the Redis client connection.
func (r *RedisDiscovery) Close() error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()

	return r.client.Close()
}

// Health checks if Redis is reachable.
func (r *RedisDiscovery) Health(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	return nil
}

// =============================================================================
// StorageBackend implementation (for KV-based SchemaRegistry)
// =============================================================================

// Put stores a value in Redis.
func (r *RedisDiscovery) Put(ctx context.Context, key string, value []byte) error {
	return r.client.Set(ctx, r.config.Namespace+":kv:"+key, value, 0).Err()
}

// Get retrieves a value from Redis.
func (r *RedisDiscovery) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, r.config.Namespace+":kv:"+key).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return nil, farp.ErrSchemaNotFound
		}

		return nil, err
	}

	return val, nil
}

// Delete removes a key from Redis.
func (r *RedisDiscovery) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.config.Namespace+":kv:"+key).Err()
}

// List lists keys with a prefix from Redis.
func (r *RedisDiscovery) List(ctx context.Context, prefix string) ([]string, error) {
	pattern := r.config.Namespace + ":kv:" + prefix + "*"

	var keys []string

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

// =============================================================================
// Helpers
// =============================================================================

// Name returns the backend name.
func (r *RedisDiscovery) Name() string { return "redis" }

// Initialize is a no-op; the backend is initialized in the constructor.
func (r *RedisDiscovery) Initialize(_ context.Context) error { return nil }

func parseHostPort(addr string) (string, int, bool) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			port, err := strconv.Atoi(addr[i+1:])
			if err == nil {
				return addr[:i], port, true
			}

			return addr, 0, false
		}
	}

	return addr, 0, false
}
