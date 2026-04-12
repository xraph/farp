// Package mdns provides mDNS/DNS-SD based service discovery for FARP.
//
// It implements ServiceDiscovery using multicast DNS (mDNS) for zero-configuration
// service discovery on local networks. Services are registered as mDNS service
// records with TXT records carrying FARP metadata such as manifest URLs.
//
// This backend is ideal for development, testing, and LAN deployments where
// no external service registry (Consul, etcd, etc.) is available.
//
// Unlike etcd or Redis backends, mDNS discovery does not implement
// StorageBackend since mDNS is a discovery-only protocol.
//
// # Usage
//
// Service side:
//
//	disc, _ := mdns.New(mdns.Config{Domain: "local."})
//	node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
//	    ServiceName: "user-service",
//	    Address:     "10.0.0.5:8080",
//	    Discovery:   disc,
//	})
//	node.Start(ctx)
//
// Gateway side:
//
//	disc, _ := mdns.New(mdns.Config{Domain: "local."})
//	gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
//	    Discovery:       disc,
//	    OnRoutesChanged: updateRoutes,
//	})
//	gw.Start(ctx)
package mdns

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"

	"github.com/xraph/farp"
	"github.com/xraph/farp/discovery"
)

// Config holds mDNS configuration.
type Config struct {
	// Domain is the mDNS domain (default: "local.").
	Domain string
	// Interface is the network interface to use for mDNS.
	// If nil, all interfaces are used.
	Interface *net.Interface
	// ServiceType is the DNS-SD service type (default: "_farp._tcp").
	ServiceType string
	// BrowseTimeout is the timeout for a single browse operation (default: 5s).
	BrowseTimeout time.Duration
	// WatchInterval is the interval between browse cycles in Watch (default: 10s).
	WatchInterval time.Duration
}

// MDNSDiscovery implements discovery.ServiceDiscovery using mDNS/DNS-SD.
type MDNSDiscovery struct {
	config  Config
	mu      sync.RWMutex
	servers map[string]*zeroconf.Server // instanceID -> mDNS server
	closed  bool
}

// New creates a new mDNS-based discovery backend.
func New(cfg Config) (*MDNSDiscovery, error) {
	if cfg.Domain == "" {
		cfg.Domain = "local."
	}

	if cfg.ServiceType == "" {
		cfg.ServiceType = "_farp._tcp"
	}

	if cfg.BrowseTimeout == 0 {
		cfg.BrowseTimeout = 5 * time.Second
	}

	if cfg.WatchInterval == 0 {
		cfg.WatchInterval = 10 * time.Second
	}

	return &MDNSDiscovery{
		config:  cfg,
		servers: make(map[string]*zeroconf.Server),
	}, nil
}

// Discover returns all FARP service instances found via mDNS browse.
func (m *MDNSDiscovery) Discover(ctx context.Context, serviceName string) ([]discovery.ServiceInstance, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create mDNS resolver: %w", farp.ErrDiscoveryUnavailable, err)
	}

	entries := make(chan *zeroconf.ServiceEntry)

	var instances []discovery.ServiceInstance

	var mu sync.Mutex

	go func() {
		for entry := range entries {
			inst := entryToInstance(entry)
			if serviceName != "" && inst.ServiceName != serviceName {
				continue
			}

			mu.Lock()
			instances = append(instances, inst)
			mu.Unlock()
		}
	}()

	browseCtx, cancel := context.WithTimeout(ctx, m.config.BrowseTimeout)
	defer cancel()

	err = resolver.Browse(browseCtx, m.config.ServiceType, m.config.Domain, entries)
	if err != nil {
		return nil, fmt.Errorf("%w: mDNS browse failed: %w", farp.ErrDiscoveryUnavailable, err)
	}

	<-browseCtx.Done()

	mu.Lock()
	defer mu.Unlock()

	return instances, nil
}

// Watch continuously browses for mDNS services and reports changes.
func (m *MDNSDiscovery) Watch(ctx context.Context, serviceName string, handler discovery.DiscoveryEventHandler) error {
	lastInstances := make(map[string]discovery.ServiceInstance)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		currentInstances, err := m.Discover(ctx, serviceName)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			time.Sleep(time.Second)

			continue
		}

		currentMap := make(map[string]discovery.ServiceInstance, len(currentInstances))
		for _, inst := range currentInstances {
			currentMap[inst.ID] = inst
		}

		// Find added/updated
		for id, inst := range currentMap {
			if prev, exists := lastInstances[id]; !exists {
				handler(discovery.DiscoveryEvent{
					Type:      farp.EventTypeAdded,
					Instance:  inst,
					Timestamp: time.Now(),
				})
			} else if prev.Status != inst.Status || prev.Address != inst.Address || prev.Port != inst.Port {
				handler(discovery.DiscoveryEvent{
					Type:      farp.EventTypeUpdated,
					Instance:  inst,
					Timestamp: time.Now(),
				})
			}
		}

		// Find removed
		for id, inst := range lastInstances {
			if _, exists := currentMap[id]; !exists {
				handler(discovery.DiscoveryEvent{
					Type:      farp.EventTypeRemoved,
					Instance:  inst,
					Timestamp: time.Now(),
				})
			}
		}

		lastInstances = currentMap

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(m.config.WatchInterval):
		}
	}
}

// Register registers a service instance via mDNS.
func (m *MDNSDiscovery) Register(_ context.Context, instance discovery.ServiceInstance) error {
	port := instance.Port
	if port == 0 {
		if _, p, ok := parseHostPort(instance.Address); ok {
			port = p
		}
	}

	// Build TXT records from metadata
	txt := []string{
		"farp.enabled=true",
		"farp.id=" + instance.ID,
		"farp.service=" + instance.ServiceName,
		"farp.status=" + string(instance.Status),
	}

	for k, v := range instance.Metadata {
		txt = append(txt, "farp.meta."+k+"="+v)
	}

	var ifaces []net.Interface
	if m.config.Interface != nil {
		ifaces = []net.Interface{*m.config.Interface}
	}

	server, err := zeroconf.Register(
		instance.ID,          // instance name
		m.config.ServiceType, // service type
		m.config.Domain,      // domain
		port,                 // port
		txt,                  // TXT records
		ifaces,               // interfaces
	)
	if err != nil {
		return fmt.Errorf("%w: mDNS registration failed: %w", farp.ErrRegistrationFailed, err)
	}

	m.mu.Lock()
	m.servers[instance.ID] = server
	m.mu.Unlock()

	return nil
}

// Deregister removes an mDNS service registration.
func (m *MDNSDiscovery) Deregister(_ context.Context, instanceID string) error {
	m.mu.Lock()
	server, exists := m.servers[instanceID]
	if exists {
		delete(m.servers, instanceID)
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("%w: instance %s", farp.ErrInstanceNotFound, instanceID)
	}

	server.Shutdown()

	return nil
}

// ReportHealth updates health by re-registering with updated TXT records.
// mDNS does not have a native health check mechanism, so we update the
// TXT records to reflect the current status.
func (m *MDNSDiscovery) ReportHealth(_ context.Context, instanceID string, status farp.InstanceStatus) error {
	m.mu.RLock()
	server, exists := m.servers[instanceID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: no mDNS server found for instance %s", farp.ErrHealthCheckFailed, instanceID)
	}

	// Update TXT records with new status
	txt := []string{
		"farp.enabled=true",
		"farp.id=" + instanceID,
		"farp.status=" + string(status),
		"farp.last-health=" + time.Now().UTC().Format(time.RFC3339),
	}

	server.SetText(txt)

	return nil
}

// Close shuts down all mDNS servers.
func (m *MDNSDiscovery) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true

	for id, server := range m.servers {
		server.Shutdown()
		delete(m.servers, id)
	}

	return nil
}

// Health always returns nil for mDNS since there is no central server to check.
// mDNS operates on the local network without a dedicated server.
func (m *MDNSDiscovery) Health(_ context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return fmt.Errorf("%w: mDNS discovery is closed", farp.ErrDiscoveryUnavailable)
	}

	return nil
}

// =============================================================================
// Helpers
// =============================================================================

// entryToInstance converts a zeroconf service entry to a ServiceInstance.
func entryToInstance(entry *zeroconf.ServiceEntry) discovery.ServiceInstance {
	metadata := make(map[string]string)

	var instanceID, serviceName, statusStr string

	for _, txt := range entry.Text {
		parts := strings.SplitN(txt, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		switch key {
		case "farp.id":
			instanceID = value
		case "farp.service":
			serviceName = value
		case "farp.status":
			statusStr = value
		default:
			if strings.HasPrefix(key, "farp.meta.") {
				metadata[strings.TrimPrefix(key, "farp.meta.")] = value
			} else {
				metadata[key] = value
			}
		}
	}

	if instanceID == "" {
		instanceID = entry.Instance
	}

	if serviceName == "" {
		serviceName = entry.Instance
	}

	status := farp.InstanceStatusHealthy
	if statusStr != "" {
		status = farp.InstanceStatus(statusStr)
	}

	addr := ""
	if len(entry.AddrIPv4) > 0 {
		addr = entry.AddrIPv4[0].String()
	} else if len(entry.AddrIPv6) > 0 {
		addr = entry.AddrIPv6[0].String()
	}

	metadata["farp.enabled"] = "true"
	metadata["hostname"] = entry.HostName

	return discovery.ServiceInstance{
		ID:          instanceID,
		ServiceName: serviceName,
		Address:     addr,
		Port:        entry.Port,
		Status:      status,
		Metadata:    metadata,
	}
}

// Name returns the backend name.
func (m *MDNSDiscovery) Name() string { return "mdns" }

// Initialize is a no-op; the backend is initialized in the constructor.
func (m *MDNSDiscovery) Initialize(_ context.Context) error { return nil }

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
