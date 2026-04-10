// Package kubernetes provides Kubernetes-based service discovery for FARP.
//
// It implements ServiceDiscovery by watching Kubernetes Endpoints resources
// labeled with farp.io/enabled=true. It supports both in-cluster configuration
// (when running inside a pod) and out-of-cluster via a kubeconfig file.
//
// Unlike etcd or Redis backends, Kubernetes discovery does not implement
// StorageBackend since Kubernetes has its own native service registry.
//
// # Usage
//
// Service side (in-cluster):
//
//	disc, _ := kubernetes.New(kubernetes.Config{Namespace: "default"})
//	node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
//	    ServiceName: "user-service",
//	    Address:     "10.0.0.5:8080",
//	    Discovery:   disc,
//	})
//	node.Start(ctx)
//
// Gateway side (out-of-cluster):
//
//	disc, _ := kubernetes.New(kubernetes.Config{
//	    KubeConfigPath: "/home/user/.kube/config",
//	    Namespace:      "default",
//	})
//	gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
//	    Discovery:       disc,
//	    OnRoutesChanged: updateRoutes,
//	})
//	gw.Start(ctx)
package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/xraph/farp"
	"github.com/xraph/farp/discovery"
)

const (
	// labelEnabled is the label used to identify FARP-enabled services.
	labelEnabled = "farp.io/enabled"
	// annotationManifestURL is the annotation key for the manifest URL.
	annotationManifestURL = "farp.io/manifest-url"
	// annotationInstanceID is the annotation key for the FARP instance ID.
	annotationInstanceID = "farp.io/instance-id"
)

// Config holds Kubernetes connection configuration.
type Config struct {
	// KubeConfigPath is the path to the kubeconfig file.
	// If empty, in-cluster configuration is used.
	KubeConfigPath string
	// Namespace is the Kubernetes namespace to watch (default: "default").
	Namespace string
}

// KubernetesDiscovery implements discovery.ServiceDiscovery using Kubernetes.
type KubernetesDiscovery struct {
	clientset *k8s.Clientset
	config    Config
	mu        sync.RWMutex
	closed    bool
}

// New creates a new Kubernetes-based discovery backend.
func New(cfg Config) (*KubernetesDiscovery, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}

	var restConfig *rest.Config
	var err error

	if cfg.KubeConfigPath != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfigPath)
	} else {
		restConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := k8s.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &KubernetesDiscovery{
		clientset: clientset,
		config:    cfg,
	}, nil
}

// Discover returns all FARP-enabled service instances from Kubernetes Endpoints.
func (k *KubernetesDiscovery) Discover(ctx context.Context, serviceName string) ([]discovery.ServiceInstance, error) {
	labelSelector := labels.Set{labelEnabled: "true"}.String()

	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	endpoints, err := k.clientset.CoreV1().Endpoints(k.config.Namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	var instances []discovery.ServiceInstance

	for _, ep := range endpoints.Items {
		if serviceName != "" && ep.Name != serviceName {
			continue
		}

		epInstances := endpointsToInstances(&ep)
		instances = append(instances, epInstances...)
	}

	return instances, nil
}

// Watch watches for changes to FARP-enabled Endpoints using the Kubernetes watch API.
func (k *KubernetesDiscovery) Watch(ctx context.Context, serviceName string, handler discovery.DiscoveryEventHandler) error {
	labelSelector := labels.Set{labelEnabled: "true"}.String()

	lastInstances := make(map[string]discovery.ServiceInstance)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		opts := metav1.ListOptions{
			LabelSelector: labelSelector,
		}

		watcher, err := k.clientset.CoreV1().Endpoints(k.config.Namespace).Watch(ctx, opts)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			time.Sleep(time.Second)

			continue
		}

		func() {
			defer watcher.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case event, ok := <-watcher.ResultChan():
					if !ok {
						return // Watch channel closed, will reconnect
					}

					ep, ok := event.Object.(*corev1.Endpoints)
					if !ok {
						continue
					}

					if serviceName != "" && ep.Name != serviceName {
						continue
					}

					currentInstances := make(map[string]discovery.ServiceInstance)

					for _, inst := range endpointsToInstances(ep) {
						currentInstances[inst.ID] = inst
					}

					switch event.Type {
					case watch.Added:
						for _, inst := range currentInstances {
							handler(discovery.DiscoveryEvent{
								Type:      farp.EventTypeAdded,
								Instance:  inst,
								Timestamp: time.Now(),
							})
						}
					case watch.Modified:
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
							if inst.ServiceName != ep.Name {
								continue
							}

							if _, exists := currentInstances[id]; !exists {
								handler(discovery.DiscoveryEvent{
									Type:      farp.EventTypeRemoved,
									Instance:  inst,
									Timestamp: time.Now(),
								})
							}
						}
					case watch.Deleted:
						for _, inst := range endpointsToInstances(ep) {
							handler(discovery.DiscoveryEvent{
								Type:      farp.EventTypeRemoved,
								Instance:  inst,
								Timestamp: time.Now(),
							})
						}
					}

					// Update tracking for this service
					for id, inst := range lastInstances {
						if inst.ServiceName == ep.Name {
							delete(lastInstances, id)
						}
					}

					for id, inst := range currentInstances {
						lastInstances[id] = inst
					}
				}
			}
		}()
	}
}

// Register annotates the current pod with FARP metadata.
// In Kubernetes, registration is done by adding annotations to the pod
// rather than creating a separate registry entry.
func (k *KubernetesDiscovery) Register(ctx context.Context, instance discovery.ServiceInstance) error {
	manifestURL := ""
	if instance.Metadata != nil {
		manifestURL = instance.Metadata["manifest_url"]
	}

	// Try to find and annotate the pod matching this instance
	pods, err := k.clientset.CoreV1().Pods(k.config.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]

		podIP := pod.Status.PodIP
		if podIP == "" {
			continue
		}

		// Match by IP address
		host := instance.Address
		if h, _, ok := parseHostPort(instance.Address); ok {
			host = h
		}

		if podIP != host {
			continue
		}

		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}

		pod.Annotations[annotationInstanceID] = instance.ID
		pod.Annotations[annotationManifestURL] = manifestURL

		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}

		pod.Labels[labelEnabled] = "true"

		_, err := k.clientset.CoreV1().Pods(k.config.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
		}

		return nil
	}

	return fmt.Errorf("%w: no pod found matching address %s", farp.ErrRegistrationFailed, instance.Address)
}

// Deregister removes FARP annotations from the pod.
func (k *KubernetesDiscovery) Deregister(ctx context.Context, instanceID string) error {
	pods, err := k.clientset.CoreV1().Pods(k.config.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]

		if pod.Annotations == nil {
			continue
		}

		if pod.Annotations[annotationInstanceID] != instanceID {
			continue
		}

		delete(pod.Annotations, annotationInstanceID)
		delete(pod.Annotations, annotationManifestURL)
		delete(pod.Labels, labelEnabled)

		_, err := k.clientset.CoreV1().Pods(k.config.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
		}

		return nil
	}

	return fmt.Errorf("%w: instance %s", farp.ErrInstanceNotFound, instanceID)
}

// ReportHealth reports health by updating pod annotations.
// In Kubernetes, pod health is typically managed by readiness/liveness probes,
// but this provides an additional FARP-level health signal.
func (k *KubernetesDiscovery) ReportHealth(ctx context.Context, instanceID string, status farp.InstanceStatus) error {
	pods, err := k.clientset.CoreV1().Pods(k.config.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]

		if pod.Annotations == nil || pod.Annotations[annotationInstanceID] != instanceID {
			continue
		}

		pod.Annotations["farp.io/status"] = string(status)
		pod.Annotations["farp.io/last-health-check"] = time.Now().UTC().Format(time.RFC3339)

		_, err := k.clientset.CoreV1().Pods(k.config.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
		}

		return nil
	}

	return fmt.Errorf("%w: instance %s not found", farp.ErrHealthCheckFailed, instanceID)
}

// Close is a no-op for Kubernetes (client-go has no Close method).
func (k *KubernetesDiscovery) Close() error {
	k.mu.Lock()
	k.closed = true
	k.mu.Unlock()

	return nil
}

// Health checks if the Kubernetes API server is reachable.
func (k *KubernetesDiscovery) Health(ctx context.Context) error {
	_, err := k.clientset.CoreV1().Namespaces().Get(ctx, k.config.Namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}

	return nil
}

// =============================================================================
// Helpers
// =============================================================================

// endpointsToInstances converts a Kubernetes Endpoints resource to ServiceInstances.
func endpointsToInstances(ep *corev1.Endpoints) []discovery.ServiceInstance {
	var instances []discovery.ServiceInstance

	for _, subset := range ep.Subsets {
		ports := make(map[string]int)

		for _, port := range subset.Ports {
			ports[port.Name] = int(port.Port)
		}

		// Default port: use the first one if available
		defaultPort := 0
		if len(subset.Ports) > 0 {
			defaultPort = int(subset.Ports[0].Port)
		}

		for _, addr := range subset.Addresses {
			instanceID := ""

			metadata := make(map[string]string)
			metadata["farp.enabled"] = "true"

			if addr.TargetRef != nil {
				instanceID = string(addr.TargetRef.UID)
				metadata["pod_name"] = addr.TargetRef.Name
				metadata["pod_namespace"] = addr.TargetRef.Namespace
			} else {
				instanceID = ep.Name + "-" + addr.IP
			}

			// Copy endpoint annotations to metadata
			for k, v := range ep.Annotations {
				metadata[k] = v
			}

			for name, port := range ports {
				metadata["port_"+name] = strconv.Itoa(port)
			}

			instances = append(instances, discovery.ServiceInstance{
				ID:          instanceID,
				ServiceName: ep.Name,
				Address:     addr.IP,
				Port:        defaultPort,
				Status:      farp.InstanceStatusHealthy,
				Metadata:    metadata,
			})
		}

		// Not-ready addresses are degraded
		for _, addr := range subset.NotReadyAddresses {
			instanceID := ""

			metadata := make(map[string]string)

			if addr.TargetRef != nil {
				instanceID = string(addr.TargetRef.UID)
				metadata["pod_name"] = addr.TargetRef.Name
			} else {
				instanceID = ep.Name + "-" + addr.IP + "-notready"
			}

			instances = append(instances, discovery.ServiceInstance{
				ID:          instanceID,
				ServiceName: ep.Name,
				Address:     addr.IP,
				Port:        defaultPort,
				Status:      farp.InstanceStatusDegraded,
				Metadata:    metadata,
			})
		}
	}

	return instances
}

// Name returns the backend name.
func (k *KubernetesDiscovery) Name() string { return "kubernetes" }

// Initialize is a no-op; the backend is initialized in the constructor.
func (k *KubernetesDiscovery) Initialize(_ context.Context) error { return nil }

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
