package discoverers

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/borderzero/discovery"
	"golang.org/x/sync/semaphore"
)

const (
	defaultNetworkDiscovererDiscovererId   = "network_discoverer"
	defaultNetworkDiscovererScanTimeout    = time.Second * 120
	defaultNetworkDiscovererMaxConcurrency = 1000
)

var (
	defaultNetworkDiscovererTargets = []string{
		"192.168.1.0/24",
	}

	defaultNetworkDiscovererPorts = []string{
		"22",   // default ssh port
		"80",   // default http port
		"443",  // default https port
		"3306", // default mysql port
		"5432", // default postgresql port
		"8080", // common http port
		"8443", // common https port
	}

	mysqlBannerCanaries = []string{
		"mariadb", // MariaDB is a fork of MySQL
		"caching_sha2_password",
		"mysql_native_password",
		"mysql_clear_password",
		"sha256_password",
		"5.7.", // for MySQL 5.7.x
		"8.0.", // for MySQL 8.0.x
		"10.",  // for MariaDB 10.x.x
	}

	sshBannerCanaries = []string{
		"ssh",      // SSH v2, OpenSSH, LibSSH, etc...
		"dropbear", // Dropbear server
		"lsh",      // lsh server
	}
)

// NetworkDiscoverer represents a discoverer for network-reachable resources
// with a very rudimentary check using TCP probes. This check can (and will) give
// false positives or negatives. For a more thorough service detection, you would
// need a more comprehensive set of checks.
// Nmap for example uses a combination of probes and a large database of well known
// service banners to identify the service running on a particular port.
type NetworkDiscoverer struct {
	discovererId   string
	scanTimeout    time.Duration
	maxConcurrency int64
	targets        []string
	ports          []string
}

// ensure NetworkDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*NetworkDiscoverer)(nil)

// NetworkDiscovererOption represents a configuration option for a NetworkDiscoverer.
type NetworkDiscovererOption func(*NetworkDiscoverer)

// WithNetworkDiscovererDiscovererId is the NetworkDiscovererOption to set a non default discoverer id.
func WithNetworkDiscovererDiscovererId(discovererId string) NetworkDiscovererOption {
	return func(nd *NetworkDiscoverer) { nd.discovererId = discovererId }
}

// WithNetworkDiscovererScanTimeout is the NetworkDiscovererOption
// to set a non default timeout for scanning the network.
func WithNetworkDiscovererScanTimeout(timeout time.Duration) NetworkDiscovererOption {
	return func(nd *NetworkDiscoverer) { nd.scanTimeout = timeout }
}

// WithNetworkDiscovererMaxConcurrency is the NetworkDiscovererOption
// to set a non default value for the maximum concurrency (port scans in-flight).
func WithNetworkDiscovererMaxConcurrency(concurrency int64) NetworkDiscovererOption {
	return func(nd *NetworkDiscoverer) { nd.maxConcurrency = concurrency }
}

// WithNetworkDiscovererTargets is the NetworkDiscovererOption
// to set non default targets (IPs or DNS names) for discovery.
func WithNetworkDiscovererTargets(targets ...string) NetworkDiscovererOption {
	return func(nd *NetworkDiscoverer) { nd.targets = targets }
}

// WithNetworkDiscovererPorts is the NetworkDiscovererOption
// to set non default target ports for discovery.
func WithNetworkDiscovererPorts(ports ...string) NetworkDiscovererOption {
	return func(nd *NetworkDiscoverer) { nd.ports = ports }
}

// NewNetworkDiscoverer returns a new NetworkDiscoverer, initialized with the given options.
func NewNetworkDiscoverer(opts ...NetworkDiscovererOption) *NetworkDiscoverer {
	nd := &NetworkDiscoverer{
		discovererId:   defaultNetworkDiscovererDiscovererId,
		scanTimeout:    defaultNetworkDiscovererScanTimeout,
		maxConcurrency: defaultNetworkDiscovererMaxConcurrency,
		targets:        defaultNetworkDiscovererTargets,
		ports:          defaultNetworkDiscovererPorts,
	}
	for _, opt := range opts {
		opt(nd)
	}
	return nd
}

// Discover runs the NetworkDiscoverer.
func (nd *NetworkDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(nd.discovererId)
	defer result.Done()

	sem := semaphore.NewWeighted(nd.maxConcurrency)

	for _, target := range nd.targets {
		ips, err := targetToIps(target)
		if err != nil {
			result.AddErrorf("failed to get IPs for target: %v", err)
			continue
		}

		var wg sync.WaitGroup
		for _, ip := range ips {
			for _, port := range nd.ports {
				wg.Add(1)

				go func(ip, port string) {
					defer wg.Done()

					if err := sem.Acquire(ctx, 1); err != nil {
						if !errors.Is(err, context.Canceled) {
							result.AddErrorf("failed to acquire semaphore: %v", err)
						}
						return
					}
					defer sem.Release(1)

					svc, ok := scanPort(ctx, ip, port)
					if ok {
						// best effort dns name lookup
						hostnames, _ := net.LookupAddr(ip)
						if hostnames == nil {
							hostnames = []string{}
						}

						networkBaseDetails := discovery.NetworkBaseDetails{
							HostNames: hostnames,
							IpAddress: ip,
							Port:      port,
						}

						switch svc {
						case "http":
							result.AddResources(discovery.Resource{
								ResourceType: discovery.ResourceTypeNetworkHttpServer,
								NetworkHttpServerDetails: &discovery.NetworkHttpServerDetails{
									NetworkBaseDetails: networkBaseDetails,
								},
							})
						case "https":
							result.AddResources(discovery.Resource{
								ResourceType: discovery.ResourceTypeNetworkHttpsServer,
								NetworkHttpsServerDetails: &discovery.NetworkHttpsServerDetails{
									NetworkBaseDetails: networkBaseDetails,
								},
							})
						case "mysql":
							result.AddResources(discovery.Resource{
								ResourceType: discovery.ResourceTypeNetworkMysqlServer,
								NetworkMysqlServerDetails: &discovery.NetworkMysqlServerDetails{
									NetworkBaseDetails: networkBaseDetails,
								},
							})
						case "postgresql":
							result.AddResources(discovery.Resource{
								ResourceType: discovery.ResourceTypeNetworkPostgresqlServer,
								NetworkPostgresqlServerDetails: &discovery.NetworkPostgresqlServerDetails{
									NetworkBaseDetails: networkBaseDetails,
								},
							})
						case "ssh":
							result.AddResources(discovery.Resource{
								ResourceType: discovery.ResourceTypeNetworkSshServer,
								NetworkSshServerDetails: &discovery.NetworkSshServerDetails{
									NetworkBaseDetails: networkBaseDetails,
								},
							})
						}
					}
				}(ip, port)
			}
		}
		wg.Wait()
	}
	return result
}

func checkService(ip string, port string) string {

	for _, scheme := range []string{"https", "http"} {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: time.Second * 3, // TODO: make configurable
		}
		req, _ := http.NewRequest(
			http.MethodGet,
			fmt.Sprintf("%s://%s:%s", scheme, ip, port),
			nil,
		)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		return scheme
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), 10*time.Millisecond)
	if err != nil {
		return "unknown"
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(time.Millisecond * 1000)) // TODO: make configurable

	resp := make([]byte, 1024)
	_, err = conn.Read(resp)
	if err != nil {
		// postgresql normally returns a binary handshake... can't check for that
		// so we assume if something's listening on port 5432, its a postgresql server
		if port == "5432" {
			return "postgresql"
		}
		return "unknown"
	}

	response := strings.ToLower(string(resp))
	for _, canary := range mysqlBannerCanaries {
		if strings.Contains(response, canary) {
			return "mysql"
		}
	}
	for _, canary := range sshBannerCanaries {
		if strings.Contains(response, canary) {
			return "ssh"
		}
	}

	// postgresql normally returns a binary handshake... can't check for that
	// so we assume if something's listening on port 5432, its a postgresql server
	if port == "5432" {
		return "postgresql"
	}

	return "unknown"
}

func scanPort(ctx context.Context, ip, port string) (string, bool) {
	if !addressReachable(ctx, fmt.Sprintf("%s:%s", ip, port)) {
		return "", false
	}

	service := checkService(ip, port)
	if service != "unknown" {
		return service, true
	}
	return "", false
}
