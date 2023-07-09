package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const (
	defaultDockerDiscovererId = "docker_discoverer"

	defaultContainerListTimeout = time.Second * 2
)

// DockerDiscoverer represents a discoverer for
// containers managed by the Docker daemon.
type DockerDiscoverer struct {
	discovererId         string
	containerListTimeout time.Duration

	inclusionContainerLabels map[string][]string
	exclusionContainerLabels map[string][]string
}

// ensure DockerDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*DockerDiscoverer)(nil)

// DockerDiscovererOption represents a configuration option for an DockerDiscoverer.
type DockerDiscovererOption func(*DockerDiscoverer)

// WithDockerDiscovererDiscovererId is the DockerDiscovererOption to set a non default discoverer id.
func WithDockerDiscovererDiscovererId(discovererId string) DockerDiscovererOption {
	return func(dd *DockerDiscoverer) { dd.discovererId = discovererId }
}

// WithDockerDiscovererListContainersTimeout is the DockerDiscovererOption
// to set a non default timeout for listing containers with the docker daemon.
func WithDockerDiscovererListContainersTimeout(timeout time.Duration) DockerDiscovererOption {
	return func(dd *DockerDiscoverer) { dd.containerListTimeout = timeout }
}

// WithDockerDiscovererInclusionInstanceTags is the DockerDiscovererOption
// to set the inclusion labels filter for containers to include in results.
func WithDockerDiscovererInclusionInstanceTags(labels map[string][]string) DockerDiscovererOption {
	return func(dd *DockerDiscoverer) { dd.inclusionContainerLabels = labels }
}

// WithDockerDiscovererExclusionContainerLabels is the DockerDiscovererOption
// to set the exclusion labels filter for containers to exclude in results.
func WithDockerDiscovererExclusionContainerLabels(labels map[string][]string) DockerDiscovererOption {
	return func(dd *DockerDiscoverer) { dd.exclusionContainerLabels = labels }
}

// NewDockerDiscoverer returns a new DockerDiscoverer, initialized with the given options.
func NewDockerDiscoverer(opts ...DockerDiscovererOption) *DockerDiscoverer {
	dd := &DockerDiscoverer{
		discovererId:             defaultDockerDiscovererId,
		containerListTimeout:     defaultContainerListTimeout,
		inclusionContainerLabels: nil,
		exclusionContainerLabels: nil,
	}
	for _, opt := range opts {
		opt(dd)
	}
	return dd
}

// Discover runs the DockerDiscoverer.
func (dd *DockerDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(dd.discovererId)
	defer result.Done()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		result.AddError(fmt.Errorf("failed to create Docker client: %w", err))
		return result
	}

	containerListCtx, cancel := context.WithTimeout(ctx, dd.containerListTimeout)
	defer cancel()

	containers, err := cli.ContainerList(containerListCtx, types.ContainerListOptions{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to list Docker containers: %w", err))
		return result
	}

	for _, container := range containers {
		if !utils.KVMatchesFilters(
			container.Labels,
			dd.inclusionContainerLabels,
			dd.exclusionContainerLabels,
		) {
			continue
		}
		portBindings := map[string]string{}
		for _, p := range container.Ports {
			if p.IP != "" {
				key := fmt.Sprintf("%s:%d", p.IP, p.PublicPort)
				value := fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
				portBindings[key] = value
			}
		}
		localContainer := &discovery.LocalDockerContainerDetails{
			ContainerId:  container.ID,
			Image:        container.Image,
			Names:        container.Names,
			Status:       container.Status,
			PortBindings: portBindings,
			Labels:       container.Labels,
		}
		result.AddResources(discovery.Resource{
			ResourceType:                discovery.ResourceTypeLocalDockerContainer,
			LocalDockerContainerDetails: localContainer,
		})
	}

	return result
}
