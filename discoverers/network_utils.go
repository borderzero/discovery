package discoverers

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

var (
	reachabilityProbeTimeout  = time.Millisecond * 500
	reachabilityProbeProtocol = "tcp"
)

func addressReachable(ctx context.Context, address string) bool {
	dialer := &net.Dialer{Timeout: reachabilityProbeTimeout}
	conn, err := dialer.DialContext(ctx, reachabilityProbeProtocol, address)
	if err != nil {
		return false
	}
	conn.Close()
	return true
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

func isCidr(target string) bool {
	return regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`).MatchString(target)
}

func isIpRange(target string) bool {
	return regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}-(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(target)
}

func isIP(target string) bool {
	return regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(target)
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

func isHostname(target string) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString(target)
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

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
