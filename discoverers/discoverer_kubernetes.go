package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/borderzero/border0-go/lib/types/pointer"
	"github.com/borderzero/border0-go/lib/types/slice"
	"github.com/borderzero/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultKubernetesDiscovererId = "kubernetes_discoverer"

	defaultKubernetesDiscovererMasterUrl       = ""
	defaultKubernetesDiscovererKubeconfigPath  = ""
	defaultKubernetesDiscovererNamespace       = ""
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
}

// ensure KubernetesDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*KubernetesDiscoverer)(nil)

// KubernetesDiscovererOption represents a configuration option for an KubernetesDiscoverer.
type KubernetesDiscovererOption func(*KubernetesDiscoverer)

// WithKubernetesDiscovererDiscovererId is the KubernetesDiscovererOption to set a non default discoverer id.
func WithKubernetesDiscovererDiscovererId(discovererId string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) {
		k8d.discovererId = discovererId
	}
}

// WithKubernetesDiscovererMasterUrl is the KubernetesDiscovererOption to run
// the discovery against a remote kubernetes cluster with a given master URL.
func WithKubernetesDiscovererMasterUrl(masterUrl string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) {
		k8d.masterUrl = masterUrl
	}
}

// WithKubernetesDiscovererKubeconfigPath is the KubernetesDiscovererOption to run
// the discovery against a remote kubernetes cluster with a given kubeconfig.
func WithKubernetesDiscovererKubeconfigPath(kubeconfigPath string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) {
		k8d.kubeconfigPath = kubeconfigPath
	}
}

// WithKubernetesDiscovererNamespace is the KubernetesDiscovererOption
// to run the discovery against a given kubernetes resources namespace.
func WithKubernetesDiscovererNamespace(namespace string) KubernetesDiscovererOption {
	return func(k8d *KubernetesDiscoverer) {
		k8d.namespace = namespace
	}
}

// NewKubernetesDiscoverer returns a new KubernetesDiscoverer, initialized with the given options.
func NewKubernetesDiscoverer(opts ...KubernetesDiscovererOption) *KubernetesDiscoverer {
	k8d := &KubernetesDiscoverer{
		discovererId:    defaultKubernetesDiscovererId,
		masterUrl:       defaultKubernetesDiscovererMasterUrl,
		kubeconfigPath:  defaultKubernetesDiscovererKubeconfigPath,
		namespace:       defaultKubernetesDiscovererNamespace,
		listPodsTimeout: defaultKubernetesDiscovererListPodsTimeout,
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
		result.AddError(fmt.Errorf("failed to get k8s config: %v", err))
		return result
	}

	// create a new clientset which includes all the k8s APIs
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		result.AddError(fmt.Errorf("failed to create new client set for k8s config: %v", err))
		return result
	}

	// initial list options
	opts := metav1.ListOptions{
		TimeoutSeconds: pointer.To(int64(k8d.listPodsTimeout.Seconds())),
		// TODO: expose label selection as option
	}

	for {
		// make k8s api call to list pods
		pods, err := clientset.CoreV1().Pods(k8d.namespace).List(ctx, opts)
		if err != nil {
			result.AddError(fmt.Errorf("failed to list pods via k8s api: %v", err))
			return result
		}

		// process pods
		for _, pod := range pods.Items {
			result.AddResources(discovery.Resource{
				ResourceType: discovery.ResourceTypeKubernetesPod,
				KubernetesPodDetails: &discovery.KubernetesPodDetails{
					Namespace:   pod.Namespace,
					PodName:     pod.Name,
					PodIP:       pod.Status.PodIP,
					NodeName:    pod.Spec.NodeName,
					Status:      string(pod.Status.Phase),
					Containers:  slice.Transform(pod.Spec.Containers, containerSpecToDetails),
					Labels:      pod.Labels,
					Annotations: pod.Annotations,
				},
			})
		}

		// check if there are more results
		if pods.Continue != "" {
			opts.Continue = pods.Continue
			continue
		}

		break
	}

	return result
}

func containerSpecToDetails(container corev1.Container) discovery.KubernetesContainerDetails {
	return discovery.KubernetesContainerDetails{
		Name:  container.Name,
		Image: container.Image,
	}
}