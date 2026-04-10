// Package etcd provides etcd-based service discovery and storage for FARP.
//
// It implements both ServiceDiscovery (for finding services) and StorageBackend
// (for KV-based schema storage), enabling a single etcd cluster to handle
// both service discovery and schema registry.
//
// Services are stored as keys under a configurable namespace prefix with leases
// for automatic TTL-based expiration. Watches use etcd's native watch API for
// efficient real-time notifications.
//
// # Usage
//
// Service side:
//
//	disc, _ := etcd.New(etcd.Config{Endpoints: []string{"localhost:2379"}})
//	node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
//	    ServiceName: "user-service",
//	    Address:     "10.0.0.5:8080",
//	    Discovery:   disc,
//	})
//	node.Start(ctx)
//
// Gateway side:
//
//	disc, _ := etcd.New(etcd.Config{Endpoints: []string{"localhost:2379"}})
//	gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
//	    Discovery:       disc,
//	    OnRoutesChanged: updateRoutes,
//	})
//	gw.Start(ctx)
package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/xraph/farp"
	"github.com/xraph/farp/discovery"
)

// Config holds etcd connection configuration.
type Config struct {
	// Endpoints is the list of etcd endpoints (default: ["localhost:2379"]).
	Endpoints []string
	// DialTimeout is the timeout for establishing a connection (default: 5s).
	DialTimeout time.Duration
	// Username for authentication.
	Username string
	// Password for authentication.
	Password string
	// FARP namespace prefix for keys (default: "farp").
	Namespace string
	// LeaseTTL is the TTL in seconds for service registrations (default: 30).
	LeaseTTL int64
}

// EtcdDiscovery implements discovery.ServiceDiscovery using etcd.
// It also implements farp.StorageBackend for KV operations.
type EtcdDiscovery struct {
	client *clientv3.Client
	config Config
	mu     sync.RWMutex
	leases map[string]clientv3.LeaseID // instanceID -> leaseID
	closed bool
}

// New creates a new etcd-based discovery backend.
func New(cfg Config) (*EtcdDiscovery, error) {
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"localhost:2379"}
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}

	if cfg.Namespace == "" {
		cfg.Namespace = "farp"
	}

	if cfg.LeaseTTL == 0 {
		cfg.LeaseTTL = 30
	}

	etcdCfg := clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	}

	if cfg.Username != "" {
		etcdCfg.Username = cfg.Username
		etcdCfg.Password = cfg.Password
	}

	client, err := clientv3.New(etcdCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return &EtcdDiscovery{
		client: client,
		config: cfg,
		leases: make(map[string]clientv3.LeaseID),
	}, nil
}

// servicePrefix returns the etcd key prefix for service instances.
func (e *EtcdDiscovery) servicePrefix() string {
	return e.config.Namespace + "/services/"
}

// instanceKey returns the etcd key for a specific service instance.
func (e *EtcdDiscovery) instanceKey(serviceName, instanceID string) string {
	return e.servicePrefix() + serviceName + "/" + instanceID
}

// kvPrefix returns the etcd key prefix for KV storage.
func (e *EtcdDiscovery) kvPrefix() string {
	return e.config.Namespace + "/kv/"
}

// Discover returns all known instances of a service from etcd.
func (e *EtcdDiscovery) Discover(ctx context.Context, serviceName string) ([]discovery.ServiceInstance, error) {
	prefix := e.servicePrefix()
	if serviceName != "" {
		prefix = e.servicePrefix() + serviceName + "/"
	}

	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	instances := make([]discovery.ServiceInstance, 0, len(resp.Kvs))

	for _, kv := range resp.Kvs {
		var inst discovery.ServiceInstance
		if err := json.Unmarshal(kv.Value, &inst); err != nil {
			continue // Skip malformed entries
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

// Watch watches for changes to instances of a service using etcd's watch API.
func (e *EtcdDiscovery) Watch(ctx context.Context, serviceName string, handler discovery.DiscoveryEventHandler) error {
	prefix := e.servicePrefix()
	if serviceName != "" {
		prefix = e.servicePrefix() + serviceName + "/"
	}

	watchCh := e.client.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

	for {
		select {
		case <-ctx.Done():
			return nil
		case resp, ok := <-watchCh:
			if !ok {
				return nil
			}

			if resp.Err() != nil {
				if ctx.Err() != nil {
					return nil
				}
				// Restart the watch on error
				time.Sleep(time.Second)

				watchCh = e.client.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

				continue
			}

			for _, ev := range resp.Events {
				var eventType farp.EventType

				var inst discovery.ServiceInstance

				switch ev.Type {
				case clientv3.EventTypePut:
					if err := json.Unmarshal(ev.Kv.Value, &inst); err != nil {
						continue
					}

					if ev.IsCreate() {
						eventType = farp.EventTypeAdded
					} else {
						eventType = farp.EventTypeUpdated
					}
				case clientv3.EventTypeDelete:
					eventType = farp.EventTypeRemoved
					// Try to recover instance info from previous KV
					if ev.PrevKv != nil {
						if err := json.Unmarshal(ev.PrevKv.Value, &inst); err != nil {
							// Use key to derive minimal info
							inst.ID = extractInstanceID(string(ev.Kv.Key))
						}
					} else {
						inst.ID = extractInstanceID(string(ev.Kv.Key))
					}
				}

				handler(discovery.DiscoveryEvent{
					Type:      eventType,
					Instance:  inst,
					Timestamp: time.Now(),
				})
			}
		}
	}
}

// Register registers a service instance in etcd with a lease for TTL.
func (e *EtcdDiscovery) Register(ctx context.Context, instance discovery.ServiceInstance) error {
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

	// Create a lease
	leaseResp, err := e.client.Grant(ctx, e.config.LeaseTTL)
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal instance: %w", farp.ErrRegistrationFailed, err)
	}

	key := e.instanceKey(instance.ServiceName, instance.ID)

	_, err = e.client.Put(ctx, key, string(data), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	e.mu.Lock()
	e.leases[instance.ID] = leaseResp.ID
	e.mu.Unlock()

	// Start keepalive to maintain the lease
	ch, err := e.client.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		return fmt.Errorf("%w: failed to start keepalive: %w", farp.ErrRegistrationFailed, err)
	}

	// Drain keepalive responses in background
	go func() {
		for range ch {
			// Consume keepalive responses
		}
	}()

	return nil
}

// Deregister removes a service instance from etcd.
func (e *EtcdDiscovery) Deregister(ctx context.Context, instanceID string) error {
	e.mu.RLock()
	leaseID, hasLease := e.leases[instanceID]
	e.mu.RUnlock()

	if hasLease {
		// Revoking the lease automatically deletes associated keys
		_, err := e.client.Revoke(ctx, leaseID)
		if err != nil {
			return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
		}

		e.mu.Lock()
		delete(e.leases, instanceID)
		e.mu.Unlock()

		return nil
	}

	// Fallback: delete by scanning for the key
	prefix := e.servicePrefix()

	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
	}

	for _, kv := range resp.Kvs {
		if strings.HasSuffix(string(kv.Key), "/"+instanceID) {
			_, err := e.client.Delete(ctx, string(kv.Key))
			if err != nil {
				return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
			}

			return nil
		}
	}

	return fmt.Errorf("%w: instance %s", farp.ErrInstanceNotFound, instanceID)
}

// ReportHealth reports the health status of a registered instance.
func (e *EtcdDiscovery) ReportHealth(ctx context.Context, instanceID string, status farp.InstanceStatus) error {
	e.mu.RLock()
	leaseID, hasLease := e.leases[instanceID]
	e.mu.RUnlock()

	if !hasLease {
		return fmt.Errorf("%w: no lease found for instance %s", farp.ErrHealthCheckFailed, instanceID)
	}

	// Keepalive to refresh the lease TTL
	_, err := e.client.KeepAliveOnce(ctx, leaseID)
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
	}

	// Update the instance status in the stored value
	prefix := e.servicePrefix()

	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
	}

	for _, kv := range resp.Kvs {
		if strings.HasSuffix(string(kv.Key), "/"+instanceID) {
			var inst discovery.ServiceInstance
			if err := json.Unmarshal(kv.Value, &inst); err != nil {
				return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
			}

			inst.Status = status
			inst.LastHealthCheck = time.Now()

			data, err := json.Marshal(inst)
			if err != nil {
				return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
			}

			_, err = e.client.Put(ctx, string(kv.Key), string(data), clientv3.WithLease(leaseID))
			if err != nil {
				return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
			}

			return nil
		}
	}

	return fmt.Errorf("%w: instance %s not found in etcd", farp.ErrHealthCheckFailed, instanceID)
}

// Close closes the etcd client connection.
func (e *EtcdDiscovery) Close() error {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()

	return e.client.Close()
}

// Health checks if etcd is reachable.
func (e *EtcdDiscovery) Health(ctx context.Context) error {
	_, err := e.client.Status(ctx, e.config.Endpoints[0])
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	return nil
}

// =============================================================================
// StorageBackend implementation (for KV-based SchemaRegistry)
// =============================================================================

// Put stores a value in etcd.
func (e *EtcdDiscovery) Put(ctx context.Context, key string, value []byte) error {
	_, err := e.client.Put(ctx, e.kvPrefix()+key, string(value))

	return err
}

// Get retrieves a value from etcd.
func (e *EtcdDiscovery) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := e.client.Get(ctx, e.kvPrefix()+key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, farp.ErrSchemaNotFound
	}

	return resp.Kvs[0].Value, nil
}

// Delete removes a key from etcd.
func (e *EtcdDiscovery) Delete(ctx context.Context, key string) error {
	_, err := e.client.Delete(ctx, e.kvPrefix()+key)

	return err
}

// List lists keys with a prefix from etcd.
func (e *EtcdDiscovery) List(ctx context.Context, prefix string) ([]string, error) {
	resp, err := e.client.Get(ctx, e.kvPrefix()+prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Key))
	}

	return keys, nil
}

// =============================================================================
// Helpers
// =============================================================================

// extractInstanceID extracts the instance ID from an etcd key.
// Keys have the format: {namespace}/services/{serviceName}/{instanceID}
func extractInstanceID(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return key
}

// Name returns the backend name.
func (e *EtcdDiscovery) Name() string { return "etcd" }

// Initialize is a no-op; the backend is initialized in the constructor.
func (e *EtcdDiscovery) Initialize(_ context.Context) error { return nil }

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
