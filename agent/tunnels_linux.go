//go:build linux

package agent

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/henrygd/beszel/internal/entities/system"
)

// sysClassNet is a variable so tests can point at a fixture tree.
var sysClassNet = "/sys/class/net"

// Tunnels come and go rarely, while a busy hypervisor can have dozens of
// interfaces — re-reading three sysfs files for each of them every tick would
// be wasteful. The interval is short enough that a tunnel dropping is still
// noticed promptly.
const tunnelRefreshInterval = 30 * time.Second

var (
	tunnelMu      sync.Mutex
	tunnelCache   []system.TunnelStatus
	tunnelFetched time.Time
)

// ARP hardware types that identify a tunnel interface. Taken from
// include/uapi/linux/if_arp.h; these are the encapsulations that actually turn
// up in practice.
var tunnelKindByARPType = map[int]string{
	768: "ipip",   // ARPHRD_TUNNEL
	769: "ip6tnl", // ARPHRD_TUNNEL6
	776: "sit",    // ARPHRD_SIT
	778: "gre",    // ARPHRD_IPGRE
	823: "ip6gre", // ARPHRD_IP6GRE
}

// Fallback devices the kernel creates with each tunnel module. They always
// exist, are never configured and are always down — reporting them would make
// "this tunnel is down" meaningless, which is the one signal worth having.
var kernelFallbackTunnels = map[string]bool{
	"ip6tnl0":  true,
	"tunl0":    true,
	"gre0":     true,
	"gretap0":  true,
	"sit0":     true,
	"ip6gre0":  true,
	"erspan0":  true,
	"ip_vti0":  true,
	"ip6_vti0": true,
}

// DEVTYPE values worth reporting for interfaces that do not carry a telling ARP
// type. WireGuard and vxlan both present as ARPHRD_NONE or Ethernet.
var tunnelKindByDevType = map[string]bool{
	"wireguard": true,
	"vxlan":     true,
	"gretap":    true,
	"ip6gretap": true,
}

func readTrimmed(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// devTypeOf reads DEVTYPE from an interface's uevent file. Absent for plain
// ethernet and for tunnels that are identified by ARP type instead.
func devTypeOf(dir string) string {
	content, err := readTrimmed(filepath.Join(dir, "uevent"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(content, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "DEVTYPE="); ok {
			return rest
		}
	}
	return ""
}

// tunnelKind returns the encapsulation name for an interface, or "" if it is
// not a tunnel.
func tunnelKind(dir string) string {
	if devType := devTypeOf(dir); tunnelKindByDevType[devType] {
		return devType
	}
	raw, err := readTrimmed(filepath.Join(dir, "type"))
	if err != nil {
		return ""
	}
	arpType, err := strconv.Atoi(raw)
	if err != nil {
		return ""
	}
	return tunnelKindByARPType[arpType]
}

// interfaceIsUp reports the administrative state. Tunnels report operstate
// "unknown" because they have no carrier, so the IFF_UP flag is the only
// meaningful signal — a tunnel taken down by hand or by a failed unit reads
// as false here while operstate would say nothing at all.
func interfaceIsUp(dir string) bool {
	raw, err := readTrimmed(filepath.Join(dir, "flags"))
	if err != nil {
		return false
	}
	flags, err := strconv.ParseUint(strings.TrimPrefix(raw, "0x"), 16, 64)
	if err != nil {
		return false
	}
	const iffUp = 0x1
	return flags&iffUp != 0
}

// getTunnelStatuses lists tunnel interfaces with their encapsulation and
// administrative state. Byte counters are deliberately not repeated here: they
// are already reported per interface in Stats.NetworkInterfaces under the same
// name, and a tunnel whose counters stop moving is visible from those.
func getTunnelStatuses() []system.TunnelStatus {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	if !tunnelFetched.IsZero() && time.Since(tunnelFetched) < tunnelRefreshInterval {
		return tunnelCache
	}
	tunnelFetched = time.Now()
	tunnelCache = readTunnelStatuses()
	return tunnelCache
}

func readTunnelStatuses() []system.TunnelStatus {
	entries, err := os.ReadDir(sysClassNet)
	if err != nil {
		return nil
	}
	result := make([]system.TunnelStatus, 0, 4)
	for _, entry := range entries {
		name := entry.Name()
		if kernelFallbackTunnels[name] {
			continue
		}
		dir := filepath.Join(sysClassNet, name)
		kind := tunnelKind(dir)
		if kind == "" {
			continue
		}
		result = append(result, system.TunnelStatus{
			Name: name,
			Kind: kind,
			Up:   interfaceIsUp(dir),
		})
	}
	if len(result) == 0 {
		return nil
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
