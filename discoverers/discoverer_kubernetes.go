package discoverers

import (
	"context"
	"time"

	"github.com/borderzero/border0-go/lib/types/maps"
	"github.com/borderzero/border0-go/lib/types/pointer"
	"github.com/borderzero/border0-go/lib/types/slice"
	"github.com/borderzero/discovery"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultKubernetesDiscovererId = "kubernetes_discoverer"

	defaultKubernetesDiscovererMasterUrl       = ""
	defaultKubernetesDiscovererKubeconfigPath  = ""
	defaultKubernetesDiscovererNamespace       = "default"
	defaultKubernetesDiscovererListPodsTimeout = time.Second * 5
)

// KubernetesDiscoverer represents a discoverer for Kubernetes pods.
type KubernetesDiscoverer struct {
	discovererId string

	masterUrl      string
	kubeconfigPath string
	namespace      string

	// note that this setting is per api call
	// until all paginated results are returned
	listPodsTimeout time.Duration

	inclusionServiceLabels map[string][]string
	exclusionServiceLabels map[string][]string
}

// ensure KubernetesDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*KubernetesDiscoverer)(nil)

// KubernetesDiscovererOption represents a configuration option for an KubernetesDiscoverer.
type KubernetesDiscovererOption func(*KubernetesDiscoverer)

// WithKubernetesDiscovererDiscovererId is the KubernetesDiscovererOption to set a non default discoverer id.
func WithKubernetesDiscovererDiscovererId(discovererId string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) { k8d.discovererId = discovererId }
}

// WithKubernetesDiscovererMasterUrl is the KubernetesDiscovererOption to run
// the discovery against a remote kubernetes cluster with a given master URL.
func WithKubernetesDiscovererMasterUrl(masterUrl string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) { k8d.masterUrl = masterUrl }
}

// WithKubernetesDiscovererKubeconfigPath is the KubernetesDiscovererOption to run
// the discovery against a remote kubernetes cluster with a given kubeconfig.
func WithKubernetesDiscovererKubeconfigPath(kubeconfigPath string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) { k8d.kubeconfigPath = kubeconfigPath }
}

// WithKubernetesDiscovererNamespace is the KubernetesDiscovererOption
// to run the discovery against a given kubernetes resources namespace.
func WithKubernetesDiscovererNamespace(namespace string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) { k8d.namespace = namespace }
}

// WithKubernetesDiscovererInclusionServiceLabels is the KubernetesDiscovererOption
// to set the inclusion labels filter for services to include in results.
func WithKubernetesDiscovererInclusionServiceLabels(labels map[string][]string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) { k8d.inclusionServiceLabels = labels }
}

// WithKubernetesDiscovererExclusionServiceLabels is the KubernetesDiscovererOption
// to set the exclusion labels filter for services to exclude in results.
func WithKubernetesDiscovererExclusionServiceLabels(labels map[string][]string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) { k8d.exclusionServiceLabels = labels }
}

// NewKubernetesDiscoverer returns a new KubernetesDiscoverer, initialized with the given options.
func NewKubernetesDiscoverer(opts ...KubernetesDiscovererOption) *KubernetesDiscoverer {
	k8d := &KubernetesDiscoverer{
		discovererId:           defaultKubernetesDiscovererId,
		masterUrl:              defaultKubernetesDiscovererMasterUrl,
		kubeconfigPath:         defaultKubernetesDiscovererKubeconfigPath,
		namespace:              defaultKubernetesDiscovererNamespace,
		listPodsTimeout:        defaultKubernetesDiscovererListPodsTimeout,
		inclusionServiceLabels: nil,
		exclusionServiceLabels: nil,
	}
	for _, opt := range opts {
		opt(k8d)
	}
	return k8d
}

// Discover runs the KubernetesDiscoverer.
func (k8d *KubernetesDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(k8d.discovererId)
	defer result.Done()

	// note: if this fails to find config by URL and path, it falls back to try
	// to use the inCluster config (which k8s injects into pod environments)
	config, err := clientcmd.BuildConfigFromFlags(k8d.masterUrl, k8d.kubeconfigPath)
	if err != nil {
		result.AddErrorf("failed to get k8s config: %v", err)
		return result
	}

	// create a new clientset which includes all the k8s APIs
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		result.AddErrorf("failed to create new client set for k8s config: %v", err)
		return result
	}

	// initial list options
	opts := metav1.ListOptions{
		TimeoutSeconds: pointer.To(int64(k8d.listPodsTimeout.Seconds())),
	}

	for {
		// make k8s api call to list services
		services, err := clientset.CoreV1().Services(k8d.namespace).List(ctx, opts)
		if err != nil {
			result.AddErrorf("failed to list services via k8s api: %v", err)
			return result
		}
		// process services
		for _, service := range services.Items {
			// ignore services that don't satisfy label conditions
			if !maps.MatchesFilters(
				service.Labels,
				k8d.inclusionServiceLabels,
				k8d.exclusionServiceLabels,
			) {
				continue
			}

			// build resource
			result.AddResources(discovery.Resource{
				ResourceType: discovery.ResourceTypeKubernetesService,
				KubernetesServiceDetails: &discovery.KubernetesServiceDetails{
					Namespace:      service.Namespace,
					Name:           service.Name,
					Uid:            string(service.UID),
					ServiceType:    string(service.Spec.Type),
					ExternalName:   service.Spec.ExternalName,
					LoadBalancerIp: service.Spec.LoadBalancerIP,
					ClusterIp:      service.Spec.ClusterIP,
					ClusterIps:     service.Spec.ClusterIPs,
					Ports:          slice.Transform(service.Spec.Ports, portSpecToDetails),
					Labels:         maps.EnsureNotNil(service.Labels),
					Annotations:    maps.EnsureNotNil(service.Annotations),
				},
			})
		}

		// check if there are more results
		if services.Continue != "" {
			opts.Continue = services.Continue
			continue
		}

		break
	}

	return result
}

func portSpecToDetails(port v1.ServicePort) discovery.KubernetesServicePort {
	return discovery.KubernetesServicePort{
		Name:        port.Name,
		Protocol:    string(port.Protocol),
		AppProtocol: port.AppProtocol,
		Port:        port.Port,
		TargetPort:  port.TargetPort.String(),
		NodePort:    port.NodePort,
	}
}
