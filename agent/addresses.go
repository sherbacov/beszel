package agent

import (
	"net"
	"sort"
	"strings"

	"github.com/henrygd/beszel/internal/entities/system"
)

// interfaceAddrsFn is a variable so tests can substitute the enumeration.
var interfaceAddrsFn = defaultInterfaceAddrs

// defaultInterfaceAddrs enumerates the host's interfaces and their addresses.
func defaultInterfaceAddrs() ([]net.Interface, map[string][]net.Addr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}
	addrs := make(map[string][]net.Addr, len(ifaces))
	for _, iface := range ifaces {
		a, err := iface.Addrs()
		if err != nil {
			// One unreadable interface must not hide the rest.
			continue
		}
		addrs[iface.Name] = a
	}
	return ifaces, addrs, nil
}

// isGlobalIP reports whether addr is worth showing: a real address the host is
// reachable on. Loopback, link-local (including IPv6 fe80::/10), multicast and
// unspecified addresses are noise on a hypervisor with dozens of interfaces.
func isGlobalIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return false
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	return !ip.IsMulticast()
}

// getInterfaceAddresses returns the host's global IP addresses grouped by
// interface, sorted for stable output. Interfaces that are down, and those left
// with no global address after filtering, are omitted entirely.
func getInterfaceAddresses() []system.InterfaceAddresses {
	ifaces, addrsByName, err := interfaceAddrsFn()
	if err != nil {
		return nil
	}

	result := make([]system.InterfaceAddresses, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		var addresses []string
		for _, addr := range addrsByName[iface.Name] {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}
			if !isGlobalIP(ip) {
				continue
			}
			addresses = append(addresses, ip.String())
		}
		if len(addresses) == 0 {
			continue
		}
		sort.Strings(addresses)
		result = append(result, system.InterfaceAddresses{
			Name:      iface.Name,
			Addresses: addresses,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.Compare(result[i].Name, result[j].Name) < 0
	})
	if len(result) == 0 {
		return nil
	}
	return result
}
