// Package consul provides Consul-based service discovery and storage for FARP.
//
// It implements both ServiceDiscovery (for finding services) and StorageBackend
// (for KV-based schema storage), enabling a single Consul cluster to handle
// both service discovery and schema registry.
//
// # Usage
//
// Service side:
//
//	disc, _ := consul.New(consul.Config{Address: "consul:8500"})
//	node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
//	    ServiceName: "user-service",
//	    Address:     "10.0.0.5:8080",
//	    Discovery:   disc,
//	})
//	node.Start(ctx)
//
// Gateway side:
//
//	disc, _ := consul.New(consul.Config{Address: "consul:8500"})
//	gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
//	    Discovery:       disc,
//	    OnRoutesChanged: updateRoutes,
//	})
//	gw.Start(ctx)
package consul

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/xraph/farp"
	"github.com/xraph/farp/discovery"
)

// Config holds Consul connection configuration.
type Config struct {
	// Consul HTTP address (default: "localhost:8500")
	Address string
	// Consul datacenter
	Datacenter string
	// ACL token
	Token string
	// FARP namespace prefix for KV keys (default: "farp")
	Namespace string
	// HTTP or HTTPS (default: "http")
	Scheme string
}

// ConsulDiscovery implements discovery.ServiceDiscovery using Consul.
// It also implements farp.StorageBackend for KV operations.
type ConsulDiscovery struct {
	client    *consulapi.Client
	config    Config
	mu        sync.RWMutex
	closed    bool
}

// New creates a new Consul-based discovery backend.
func New(cfg Config) (*ConsulDiscovery, error) {
	if cfg.Address == "" {
		cfg.Address = "localhost:8500"
	}

	if cfg.Namespace == "" {
		cfg.Namespace = "farp"
	}

	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}

	consulCfg := consulapi.DefaultConfig()
	consulCfg.Address = cfg.Address
	consulCfg.Scheme = cfg.Scheme

	if cfg.Datacenter != "" {
		consulCfg.Datacenter = cfg.Datacenter
	}

	if cfg.Token != "" {
		consulCfg.Token = cfg.Token
	}

	client, err := consulapi.NewClient(consulCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &ConsulDiscovery{
		client: client,
		config: cfg,
	}, nil
}

// Discover returns all healthy instances of a service from Consul.
func (c *ConsulDiscovery) Discover(ctx context.Context, serviceName string) ([]discovery.ServiceInstance, error) {
	opts := &consulapi.QueryOptions{}
	opts = opts.WithContext(ctx)

	entries, _, err := c.client.Health().Service(serviceName, "farp", true, opts)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	instances := make([]discovery.ServiceInstance, 0, len(entries))

	for _, entry := range entries {
		instances = append(instances, consulEntryToInstance(entry))
	}

	return instances, nil
}

// Watch watches for changes to instances of a service using Consul blocking queries.
func (c *ConsulDiscovery) Watch(ctx context.Context, serviceName string, handler discovery.DiscoveryEventHandler) error {
	var lastIndex uint64
	lastInstances := make(map[string]discovery.ServiceInstance)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		opts := &consulapi.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  30 * time.Second,
		}
		opts = opts.WithContext(ctx)

		entries, meta, err := c.client.Health().Service(serviceName, "farp", true, opts)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			time.Sleep(time.Second) // Brief backoff on error

			continue
		}

		if meta.LastIndex == lastIndex {
			continue // No changes
		}

		lastIndex = meta.LastIndex

		// Diff current vs previous
		currentInstances := make(map[string]discovery.ServiceInstance, len(entries))
		for _, entry := range entries {
			inst := consulEntryToInstance(entry)
			currentInstances[inst.ID] = inst
		}

		// Find added/updated
		for id, inst := range currentInstances {
			if _, exists := lastInstances[id]; !exists {
				handler(discovery.DiscoveryEvent{
					Type:      farp.EventTypeAdded,
					Instance:  inst,
					Timestamp: time.Now(),
				})
			} else {
				handler(discovery.DiscoveryEvent{
					Type:      farp.EventTypeUpdated,
					Instance:  inst,
					Timestamp: time.Now(),
				})
			}
		}

		// Find removed
		for id, inst := range lastInstances {
			if _, exists := currentInstances[id]; !exists {
				handler(discovery.DiscoveryEvent{
					Type:      farp.EventTypeRemoved,
					Instance:  inst,
					Timestamp: time.Now(),
				})
			}
		}

		lastInstances = currentInstances
	}
}

// Register registers a service instance in Consul with a TTL health check.
func (c *ConsulDiscovery) Register(ctx context.Context, instance discovery.ServiceInstance) error {
	port := instance.Port

	// Parse port from address if not set
	if port == 0 {
		if _, p, ok := parseHostPort(instance.Address); ok {
			port = p
		}
	}

	meta := make(map[string]string)
	for k, v := range instance.Metadata {
		meta[k] = v
	}

	meta["farp.enabled"] = "true"

	reg := &consulapi.AgentServiceRegistration{
		ID:      instance.ID,
		Name:    instance.ServiceName,
		Address: instance.Address,
		Port:    port,
		Meta:    meta,
		Tags:    []string{"farp"},
		Check: &consulapi.AgentServiceCheck{
			TTL:                            "30s",
			DeregisterCriticalServiceAfter: "90s",
		},
	}

	if err := c.client.Agent().ServiceRegister(reg); err != nil {
		return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	// Initial health report
	return c.ReportHealth(ctx, instance.ID, farp.InstanceStatusHealthy)
}

// Deregister removes a service instance from Consul.
func (c *ConsulDiscovery) Deregister(_ context.Context, instanceID string) error {
	if err := c.client.Agent().ServiceDeregister(instanceID); err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
	}

	return nil
}

// ReportHealth reports the health status via Consul TTL check.
func (c *ConsulDiscovery) ReportHealth(_ context.Context, instanceID string, status farp.InstanceStatus) error {
	checkID := "service:" + instanceID

	var err error

	switch status {
	case farp.InstanceStatusHealthy:
		err = c.client.Agent().PassTTL(checkID, "FARP healthy")
	case farp.InstanceStatusDegraded:
		err = c.client.Agent().WarnTTL(checkID, "FARP degraded")
	default:
		err = c.client.Agent().FailTTL(checkID, "FARP "+string(status))
	}

	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
	}

	return nil
}

// Close is a no-op for Consul (the client has no Close method).
func (c *ConsulDiscovery) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	return nil
}

// Health checks if Consul is reachable.
func (c *ConsulDiscovery) Health(ctx context.Context) error {
	opts := &consulapi.QueryOptions{}
	opts = opts.WithContext(ctx)

	_, err := c.client.Status().Leader()
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	return nil
}

// =============================================================================
// StorageBackend implementation (for KV-based SchemaRegistry)
// =============================================================================

// Put stores a value in Consul KV.
func (c *ConsulDiscovery) Put(_ context.Context, key string, value []byte) error {
	p := &consulapi.KVPair{
		Key:   c.config.Namespace + "/" + key,
		Value: value,
	}

	_, err := c.client.KV().Put(p, nil)

	return err
}

// Get retrieves a value from Consul KV.
func (c *ConsulDiscovery) Get(_ context.Context, key string) ([]byte, error) {
	p, _, err := c.client.KV().Get(c.config.Namespace+"/"+key, nil)
	if err != nil {
		return nil, err
	}

	if p == nil {
		return nil, farp.ErrSchemaNotFound
	}

	return p.Value, nil
}

// Delete removes a key from Consul KV.
func (c *ConsulDiscovery) Delete(_ context.Context, key string) error {
	_, err := c.client.KV().Delete(c.config.Namespace+"/"+key, nil)

	return err
}

// List lists keys with a prefix from Consul KV.
func (c *ConsulDiscovery) List(_ context.Context, prefix string) ([]string, error) {
	keys, _, err := c.client.KV().Keys(c.config.Namespace+"/"+prefix, "", nil)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

// =============================================================================
// Helpers
// =============================================================================

func consulEntryToInstance(entry *consulapi.ServiceEntry) discovery.ServiceInstance {
	metadata := make(map[string]string)
	for k, v := range entry.Service.Meta {
		metadata[k] = v
	}

	status := farp.InstanceStatusHealthy

	for _, check := range entry.Checks {
		if check.Status == "critical" {
			status = farp.InstanceStatusUnhealthy

			break
		} else if check.Status == "warning" {
			status = farp.InstanceStatusDegraded
		}
	}

	return discovery.ServiceInstance{
		ID:          entry.Service.ID,
		ServiceName: entry.Service.Service,
		Address:     entry.Service.Address,
		Port:        entry.Service.Port,
		Status:      status,
		Metadata:    metadata,
	}
}

// Name returns the backend name.
func (c *ConsulDiscovery) Name() string { return "consul" }

// Initialize is a no-op; the backend is initialized in the constructor.
func (c *ConsulDiscovery) Initialize(_ context.Context) error { return nil }

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
