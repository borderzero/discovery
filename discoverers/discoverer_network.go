package discoverers

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
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
	conn, err := (&net.Dialer{Timeout: time.Millisecond * 1000}).DialContext(
		ctx,
		"tcp",
		fmt.Sprintf("%s:%s", ip, port),
	)
	if err != nil {
		return "", false
	}
	conn.Close()

	service := checkService(ip, port)
	if service != "unknown" {
		return service, true
	}
	return "", false
}

func targetToIps(target string) ([]string, error) {
	if isCidr(target) {
		ips, err := cidrToIPs(target)
		if err != nil {
			return nil, fmt.Errorf("failed to get IPs from CIDR: %v", err)
		}
		return ips, nil
	}
	if isIpRange(target) {
		ips, err := rangeToIPs(target)
		if err != nil {
			return nil, fmt.Errorf("failed to get IPs from range: %v", err)
		}
		return ips, nil
	}
	if isIP(target) {
		return []string{target}, nil
	}
	if isInterface(target) {
		ips, err := interfaceToIPs(target)
		if err != nil {
			return nil, fmt.Errorf("failed to get IPs from interface: %v", err)
		}
		return ips, nil
	}
	if isHostname(target) {
		ips, err := hostnameToIps(target)
		if err != nil {
			return nil, fmt.Errorf("failed to get IPs from hostname: %v", err)
		}
		return ips, nil
	}
	return nil, fmt.Errorf("target \"%s\" is not a valid CIDR, IP range, IP, network interface, nor hostname", target)
}

func isIP(target string) bool {
	return regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(target)
}

func isIpRange(target string) bool {
	return regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}-(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(target)
}

func isCidr(target string) bool {
	return regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`).MatchString(target)
}

func isHostname(target string) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString(target)
}

func isInterface(target string) bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, inter := range interfaces {
		if inter.Name == target {
			return true
		}
	}
	return false
}

func interfaceToIPs(target string) ([]string, error) {
	iface, err := net.InterfaceByName(target)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface by name: %v", err)
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %s: %v", iface.Name, err)
	}

	ips := []string{}
	for _, addr := range addresses {
		ipnet, ok := addr.(*net.IPNet)
		if ok && ipnet.IP.To4() != nil {
			netIps, err := cidrToIPs(ipnet.String())
			if err != nil {
				return nil, fmt.Errorf("failed to convert interface cidr to list of ips: %v", err)
			}
			ips = append(ips, netIps...)
		}
	}
	return ips, nil
}

func hostnameToIps(hostname string) ([]string, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	ipStrings := make([]string, len(ips))
	for i, ip := range ips {
		ipStrings[i] = ip.String()
	}
	return ipStrings, nil
}

func rangeToIPs(ipRange string) ([]string, error) {
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range")
	}

	startIP := net.ParseIP(parts[0])
	endIP := net.ParseIP(parts[1])
	if startIP == nil || endIP == nil {
		return nil, fmt.Errorf("invalid IP address")
	}

	ips := make([]string, 0)
	for ip := startIP; !ip.Equal(endIP); incIP(ip) {
		ips = append(ips, ip.String())
	}
	ips = append(ips, endIP.String()) // Include the endIP in the list

	return ips, nil
}

func cidrToIPs(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
	}

	// remove network and broadcast addresses
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
