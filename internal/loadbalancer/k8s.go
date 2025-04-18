package loadbalancer

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sDiscoveryConfig holds configuration for Kubernetes service discovery
type K8sDiscoveryConfig struct {
	Enabled          bool
	KubeconfigPath   string
	Namespace        string
	LabelSelector    string
	AnnotationPrefix string
	SyncInterval     time.Duration
	InCluster        bool
}

// K8sServiceDiscovery provides Kubernetes service discovery
type K8sServiceDiscovery struct {
	client        *kubernetes.Clientset
	config        K8sDiscoveryConfig
	backendPool   *BackendPool
	failoverMgr   *FailoverManager
	stopCh        chan struct{}
	wg            sync.WaitGroup
	labelSelector labels.Selector
}

// NewK8sServiceDiscovery creates a new K8s service discovery instance
func NewK8sServiceDiscovery(config K8sDiscoveryConfig, backendPool *BackendPool, failoverMgr *FailoverManager) (*K8sServiceDiscovery, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("kubernetes service discovery is disabled")
	}

	var k8sConfig *rest.Config
	var err error

	if config.InCluster {
		// Use in-cluster config
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %v", err)
		}
	} else {
		// Use provided kubeconfig
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", config.KubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %v", err)
		}
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	// Parse label selector
	selector, err := labels.Parse(config.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %v", err)
	}

	return &K8sServiceDiscovery{
		client:        clientset,
		config:        config,
		backendPool:   backendPool,
		failoverMgr:   failoverMgr,
		stopCh:        make(chan struct{}),
		labelSelector: selector,
	}, nil
}

// Start starts the service discovery process
func (k *K8sServiceDiscovery) Start() {
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		ticker := time.NewTicker(k.config.SyncInterval)
		defer ticker.Stop()

		// Initial sync
		k.syncEndpoints()

		for {
			select {
			case <-ticker.C:
				k.syncEndpoints()
			case <-k.stopCh:
				log.Println("Stopping Kubernetes service discovery")
				return
			}
		}
	}()

	log.Printf("Started Kubernetes service discovery in namespace %s with interval %v",
		k.config.Namespace, k.config.SyncInterval)
}

// Stop stops the service discovery process
func (k *K8sServiceDiscovery) Stop() {
	close(k.stopCh)
	k.wg.Wait()
}

// syncEndpoints synchronizes endpoints from Kubernetes services
func (k *K8sServiceDiscovery) syncEndpoints() {
	log.Println("Syncing endpoints from Kubernetes")

	// Get services matching the label selector
	listOptions := metav1.ListOptions{
		LabelSelector: k.labelSelector.String(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	services, err := k.client.CoreV1().Services(k.config.Namespace).List(ctx, listOptions)
	if err != nil {
		log.Printf("Failed to list services: %v", err)
		return
	}

	// Find endpoints for each service
	for _, service := range services.Items {
		k.processService(service)
	}
}

// processService processes a single Kubernetes service
func (k *K8sServiceDiscovery) processService(service v1.Service) {
	serviceName := service.Name
	log.Printf("Processing Kubernetes service: %s", serviceName)

	// Skip services without selectors (externally managed)
	if len(service.Spec.Selector) == 0 {
		log.Printf("Skipping service %s without selector", serviceName)
		return
	}

	// Check if service has required annotations
	protocol := k.getAnnotation(service.Annotations, "protocol", "http")
	groupName := k.getAnnotation(service.Annotations, "group", "")
	isHAGroup := k.getAnnotation(service.Annotations, "ha-group", "false") == "true"
	usePrimary := k.getAnnotation(service.Annotations, "primary", "false") == "true"

	// Get endpoints for this service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	endpoints, err := k.client.CoreV1().Endpoints(k.config.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get endpoints for service %s: %v", serviceName, err)
		return
	}

	// Process endpoints
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			for _, port := range subset.Ports {
				// Create a unique identifier for this endpoint
				endpointID := fmt.Sprintf("%s:%s:%d", serviceName, address.IP, port.Port)

				// Check if endpoint already exists
				k.backendPool.mu.RLock()
				_, exists := k.backendPool.backends[endpointID]
				k.backendPool.mu.RUnlock()

				if !exists {
					// Create new backend
					backend := &Backend{
						Address:      address.IP,
						Protocol:     protocol,
						Port:         int(port.Port),
						Weight:       1, // Default weight
						Health:       true,
						CircuitState: CircuitClosed,
					}

					// Set weight if annotation exists
					if weightStr := k.getAnnotation(service.Annotations, "weight", ""); weightStr != "" {
						if weight, err := strconv.Atoi(weightStr); err == nil && weight > 0 {
							backend.Weight = weight
						}
					}

					// Add to backend pool
					k.backendPool.AddBackend(backend)
					log.Printf("Added Kubernetes endpoint: %s (%s:%d)", endpointID, address.IP, port.Port)

					// Add to HA group if specified
					if groupName != "" && k.failoverMgr != nil {
						// Check if group exists, create if not
						group := k.failoverMgr.GetBackendGroup(groupName)
						if group == nil {
							mode := NormalMode
							if isHAGroup {
								mode = ActivePassiveMode
							}
							k.failoverMgr.CreateBackendGroup(groupName, mode)
						}

						// Add backend to group
						k.failoverMgr.AddBackendToGroup(groupName, endpointID, usePrimary)
					}
				}
			}
		}
	}
}

// getAnnotation gets an annotation value with a prefix
func (k *K8sServiceDiscovery) getAnnotation(annotations map[string]string, key, defaultValue string) string {
	prefixedKey := k.config.AnnotationPrefix + key
	if value, exists := annotations[prefixedKey]; exists {
		return value
	}
	return defaultValue
}

// syncGroupMembership syncs backends in a failover group based on Kubernetes services
func (k *K8sServiceDiscovery) syncGroupMembership(groupName string) {
	if k.failoverMgr == nil {
		return
	}

	group := k.failoverMgr.GetBackendGroup(groupName)
	if group == nil {
		return
	}

	// Get all services with this group annotation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	listOptions := metav1.ListOptions{
		LabelSelector: k.labelSelector.String(),
	}

	services, err := k.client.CoreV1().Services(k.config.Namespace).List(ctx, listOptions)
	if err != nil {
		log.Printf("Failed to list services: %v", err)
		return
	}

	var matchingServices []v1.Service
	for _, svc := range services.Items {
		svcGroup := k.getAnnotation(svc.Annotations, "group", "")
		if svcGroup == groupName {
			matchingServices = append(matchingServices, svc)
		}
	}

	if len(matchingServices) == 0 {
		return
	}

	// For each service, get endpoints and add to group
	for _, svc := range matchingServices {
		isPrimary := k.getAnnotation(svc.Annotations, "primary", "false") == "true"
		endpoints, err := k.client.CoreV1().Endpoints(k.config.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			log.Printf("Failed to get endpoints for service %s: %v", svc.Name, err)
			continue
		}

		for _, subset := range endpoints.Subsets {
			for _, address := range subset.Addresses {
				for _, port := range subset.Ports {
					endpointID := fmt.Sprintf("%s:%s:%d", svc.Name, address.IP, port.Port)
					k.failoverMgr.AddBackendToGroup(groupName, endpointID, isPrimary)
				}
			}
		}
	}
}

// GetServiceEndpoints returns all endpoints for a specific service
func (k *K8sServiceDiscovery) GetServiceEndpoints(serviceName string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	endpoints, err := k.client.CoreV1().Endpoints(k.config.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get endpoints for service %s: %v", serviceName, err)
		return nil
	}

	var result []string
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			for _, port := range subset.Ports {
				result = append(result, fmt.Sprintf("%s:%d", address.IP, port.Port))
			}
		}
	}

	return result
}

// GetKubernetesStatus returns the status of Kubernetes service discovery
func (k *K8sServiceDiscovery) GetKubernetesStatus() map[string]interface{} {
	nodeCount := 0
	podCount := 0
	serviceCount := 0

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get node count
	if nodes, err := k.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		nodeCount = len(nodes.Items)
	}

	// Get pod count in namespace
	if pods, err := k.client.CoreV1().Pods(k.config.Namespace).List(ctx, metav1.ListOptions{}); err == nil {
		podCount = len(pods.Items)
	}

	// Get service count in namespace
	listOptions := metav1.ListOptions{
		LabelSelector: k.labelSelector.String(),
	}
	if services, err := k.client.CoreV1().Services(k.config.Namespace).List(ctx, listOptions); err == nil {
		serviceCount = len(services.Items)
	}

	return map[string]interface{}{
		"status":         "connected",
		"namespace":      k.config.Namespace,
		"node_count":     nodeCount,
		"pod_count":      podCount,
		"service_count":  serviceCount,
		"label_selector": k.config.LabelSelector,
		"in_cluster":     k.config.InCluster,
		"sync_interval":  k.config.SyncInterval.String(),
	}
}