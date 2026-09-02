package agent

import (
	"errors"
	"net"
	"testing"
)

// stubAddrs installs a fake interface enumeration for the duration of a test.
func stubAddrs(t *testing.T, ifaces []net.Interface, addrs map[string][]net.Addr, err error) {
	t.Helper()
	prev := interfaceAddrsFn
	interfaceAddrsFn = func() ([]net.Interface, map[string][]net.Addr, error) {
		return ifaces, addrs, err
	}
	t.Cleanup(func() { interfaceAddrsFn = prev })
}

func ipnet(cidr string) net.Addr {
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	n.IP = ip
	return n
}

func TestIsGlobalIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"94.141.120.8", true},
		{"2001:41d0:203:d07f:4::1", true},
		{"10.0.0.1", true},   // приватный, но настоящий адрес хоста
		{"127.0.0.1", false}, // loopback
		{"::1", false},
		{"169.254.10.1", false},   // link-local IPv4
		{"fe80::5054:ff:fe5b", false}, // link-local IPv6
		{"224.0.0.1", false},      // multicast
		{"0.0.0.0", false},
	}
	for _, c := range cases {
		if got := isGlobalIP(net.ParseIP(c.ip)); got != c.want {
			t.Errorf("isGlobalIP(%s) = %v, ожидалось %v", c.ip, got, c.want)
		}
	}
	if isGlobalIP(nil) {
		t.Error("isGlobalIP(nil) должен быть false")
	}
}

func TestGetInterfaceAddressesFiltersAndSorts(t *testing.T) {
	stubAddrs(t,
		[]net.Interface{
			{Name: "vmbr0", Flags: net.FlagUp},
			{Name: "eno1", Flags: net.FlagUp},
			{Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
			{Name: "eno2", Flags: 0}, // выключен
		},
		map[string][]net.Addr{
			"vmbr0": {ipnet("94.141.120.254/24"), ipnet("fe80::1/64"), ipnet("10.0.0.1/32")},
			"eno1":  {ipnet("57.128.92.127/32"), ipnet("2001:41d0:203:d07f::2/128")},
			"lo":    {ipnet("127.0.0.1/8"), ipnet("::1/128")},
			"eno2":  {ipnet("192.0.2.1/24")},
		},
		nil,
	)

	got := getInterfaceAddresses()
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 интерфейса, получено %d: %+v", len(got), got)
	}
	// отсортировано по имени
	if got[0].Name != "eno1" || got[1].Name != "vmbr0" {
		t.Errorf("интерфейсы не отсортированы: %s, %s", got[0].Name, got[1].Name)
	}
	// адреса внутри интерфейса тоже отсортированы
	if len(got[1].Addresses) != 2 || got[1].Addresses[0] != "10.0.0.1" || got[1].Addresses[1] != "94.141.120.254" {
		t.Errorf("адреса vmbr0 неверны: %v", got[1].Addresses)
	}
	// link-local отфильтрован
	for _, a := range got[1].Addresses {
		if a == "fe80::1" {
			t.Error("link-local не должен попадать в список")
		}
	}
}

func TestGetInterfaceAddressesEmpty(t *testing.T) {
	stubAddrs(t, []net.Interface{{Name: "lo", Flags: net.FlagUp}},
		map[string][]net.Addr{"lo": {ipnet("127.0.0.1/8")}}, nil)
	if got := getInterfaceAddresses(); got != nil {
		t.Errorf("при одних лишь loopback ожидался nil, получено %+v", got)
	}
}

func TestGetInterfaceAddressesError(t *testing.T) {
	stubAddrs(t, nil, nil, errors.New("не удалось перечислить"))
	if got := getInterfaceAddresses(); got != nil {
		t.Errorf("при ошибке ожидался nil, получено %+v", got)
	}
}
