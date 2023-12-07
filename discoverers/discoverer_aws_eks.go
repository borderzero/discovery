package discoverers

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/borderzero/border0-go/lib/types/maps"
	"github.com/borderzero/border0-go/lib/types/pointer"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
)

const (
	defaultAwsEksDiscovererDiscovererId        = "aws_eks_discoverer"
	defaultAwsEksDiscovererGetAccountIdTimeout = time.Second * 10

	defaultAwsEksEndpointReachabilityEnabled  = true
	defaultAwsEksEndpointReachabilityRequired = false
)

// AwsEksDiscoverer represents a discoverer for AWS EKS resources.
type AwsEksDiscoverer struct {
	cfg aws.Config

	discovererId        string
	getAccountIdTimeout time.Duration

	inclusionClusterTags map[string][]string
	exclusionClusterTags map[string][]string

	endpointReachabilityEnabled  bool
	endpointReachabilityRequired bool
}

// ensure AwsEksDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEksDiscoverer)(nil)

// AwsEksDiscovererOption represents a configuration option for an AwsEksDiscoverer.
type AwsEksDiscovererOption func(*AwsEksDiscoverer)

// WithAwsEksDiscovererDiscovererId is the AwsEksDiscovererOption to set a non default discoverer id.
func WithAwsEksDiscovererDiscovererId(discovererId string) AwsEksDiscovererOption {
	return func(eksd *AwsEksDiscoverer) { eksd.discovererId = discovererId }
}

// WithAwsEksDiscovererGetAccountIdTimeout is the AwsEksDiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsEksDiscovererGetAccountIdTimeout(timeout time.Duration) AwsEksDiscovererOption {
	return func(eksd *AwsEksDiscoverer) { eksd.getAccountIdTimeout = timeout }
}

// WithAwsEksDiscovererInclusionServiceTags is the AwsEksDiscovererOption
// to set the inclusion tags filter for clusters to include in results.
func WithAwsEksDiscovererInclusionServiceTags(tags map[string][]string) AwsEksDiscovererOption {
	return func(eksd *AwsEksDiscoverer) { eksd.inclusionClusterTags = tags }
}

// WithAwsEksDiscovererExclusionServiceTags is the AwsEksDiscovererOption
// to set the inclusion tags filter for clusters to exclude in results.
func WithAwsEksDiscovererExclusionServiceTags(tags map[string][]string) AwsEksDiscovererOption {
	return func(eksd *AwsEksDiscoverer) { eksd.exclusionClusterTags = tags }
}

// WithAwsEksDiscovererNetworkReachabilityCheck is the AwsEksDiscovererOption
// to enable/disable checking cluster endpoint reachability via the network.
func WithAwsEksDiscovererNetworkReachabilityCheck(enabled bool) AwsEksDiscovererOption {
	return func(eksd *AwsEksDiscoverer) { eksd.endpointReachabilityEnabled = enabled }
}

// WithAwsEksDiscovererReachabilityRequired is the AwsEksDiscovererOption
// to exclude clusters that are not reachable through any means from results.
// If required is true, enabled is automatically set to true.
func WithAwsEksDiscovererReachabilityRequired(required bool) AwsEksDiscovererOption {
	return func(eksd *AwsEksDiscoverer) {
		eksd.endpointReachabilityEnabled = eksd.endpointReachabilityEnabled || required
		eksd.endpointReachabilityRequired = required
	}
}

// NewAwsEksDiscoverer returns a new AwsEksDiscoverer.
func NewAwsEksDiscoverer(cfg aws.Config, opts ...AwsEksDiscovererOption) *AwsEksDiscoverer {
	eksd := &AwsEksDiscoverer{
		cfg: cfg,

		discovererId:         defaultAwsEksDiscovererDiscovererId,
		getAccountIdTimeout:  defaultAwsEksDiscovererGetAccountIdTimeout,
		inclusionClusterTags: nil,
		exclusionClusterTags: nil,

		endpointReachabilityEnabled:  defaultAwsEksEndpointReachabilityEnabled,
		endpointReachabilityRequired: defaultAwsEksEndpointReachabilityRequired,
	}
	for _, opt := range opts {
		opt(eksd)
	}
	return eksd
}

// Discover runs the AwsEksDiscoverer and closes the channels after a single run.
func (eksd *AwsEksDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(eksd.discovererId)
	defer result.Done()

	awsAccountId, err := utils.AwsAccountIdFromConfig(ctx, eksd.cfg, eksd.getAccountIdTimeout)
	if err != nil {
		result.AddErrorf("failed to get AWS account ID from AWS configuration: %v", err)
		return result
	}

	// wait group for reachability checks
	var wg sync.WaitGroup
	defer wg.Wait()

	eksClient := eks.NewFromConfig(eksd.cfg)
	paginator := eks.NewListClustersPaginator(eksClient, &eks.ListClustersInput{})
	for paginator.HasMorePages() {
		keepGoing := eksd.processEksListClustersPage(
			ctx,
			&wg,
			eksClient,
			paginator,
			result,
			awsAccountId,
		)
		if !keepGoing {
			break
		}
	}

	return result
}

func (eksd *AwsEksDiscoverer) processEksListClustersPage(
	ctx context.Context,
	wg *sync.WaitGroup,
	eksClient *eks.Client,
	listClustersPaginator *eks.ListClustersPaginator,
	result *discovery.Result,
	awsAccountId string,
) bool {
	listClustersOutput, err := listClustersPaginator.NextPage(ctx)
	if err != nil {
		result.AddErrorf("failed to list ecs clusters: %v", err)
		return false
	}
	if len(listClustersOutput.Clusters) == 0 {
		return true
	}

	for _, cluster := range listClustersOutput.Clusters {
		describeClusterInput := &eks.DescribeClusterInput{Name: aws.String(cluster)}
		describeClusterOutput, err := eksClient.DescribeCluster(ctx, describeClusterInput)
		if err != nil {
			result.AddErrorf("failed to describe eks cluster \"%s\": %v", cluster, err)
			return false
		}
		wg.Add(1)
		go eksd.processEksCluster(ctx, wg, describeClusterOutput.Cluster, result, awsAccountId)
	}

	return true
}

func (eksd *AwsEksDiscoverer) processEksCluster(
	ctx context.Context,
	wg *sync.WaitGroup,
	cluster *types.Cluster,
	result *discovery.Result,
	awsAccountId string,
) {
	defer wg.Done()

	// ignore clusters that don't satisfy tag conditions
	if !maps.MatchesFilters(
		cluster.Tags,
		eksd.inclusionClusterTags,
		eksd.exclusionClusterTags,
	) {
		return
	}

	// build resource
	awsBaseDetails := discovery.AwsBaseDetails{
		AwsRegion:    eksd.cfg.Region,
		AwsAccountId: awsAccountId,
		AwsArn:       aws.ToString(cluster.Arn),
	}
	eksClusterDetails := &discovery.AwsEksClusterDetails{
		AwsBaseDetails:    awsBaseDetails,
		ClusterName:       aws.ToString(cluster.Name),
		KubernetesVersion: aws.ToString(cluster.Version),
		Endpoint:          aws.ToString(cluster.Endpoint),
		Tags:              cluster.Tags,
	}
	if cluster.ResourcesVpcConfig != nil {
		eksClusterDetails.VpcId = aws.ToString(cluster.ResourcesVpcConfig.VpcId)
	}
	if eksd.endpointReachabilityEnabled {
		eksClusterDetails.EndpointReachable = pointer.To(reachable(ctx, aws.ToString(cluster.Endpoint)))
	}
	if eksd.endpointReachabilityRequired && !pointer.ValueOrZero(eksClusterDetails.EndpointReachable) {
		return
	}
	result.AddResources(discovery.Resource{
		ResourceType:         discovery.ResourceTypeAwsEksCluster,
		AwsEksClusterDetails: eksClusterDetails,
	})
}

// FIXME: add caching with default TTL of ~10(?) minutes
func reachable(ctx context.Context, endpoint string) bool {
	client := &http.Client{
		// short timeout
		Timeout: 2 * time.Second,
		// don't check tls cert
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// note that we don't care about the status code - only that we are able
	// to make an http request i.e. the code above did not error out.
	return true
}
