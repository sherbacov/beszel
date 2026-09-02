//go:build linux

package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeIface writes a minimal sysfs entry for one interface.
func fakeIface(t *testing.T, root, name, arpType, flags, devType string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(file, content string) {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("type", arpType+"\n")
	write("flags", flags+"\n")
	uevent := "INTERFACE=" + name + "\n"
	if devType != "" {
		uevent += "DEVTYPE=" + devType + "\n"
	}
	write("uevent", uevent)
}

// useFixture points the collector at a temporary tree and clears the cache.
func useFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	prev := sysClassNet
	sysClassNet = root
	tunnelMu.Lock()
	tunnelFetched = time.Time{}
	tunnelCache = nil
	tunnelMu.Unlock()
	t.Cleanup(func() {
		sysClassNet = prev
		tunnelMu.Lock()
		tunnelFetched = time.Time{}
		tunnelCache = nil
		tunnelMu.Unlock()
	})
	return root
}

func TestGetTunnelStatuses(t *testing.T) {
	root := useFixture(t)
	// ip6tnl, up — как tun-hw29 между hw20 и hw29
	fakeIface(t, root, "tun-hw29", "769", "0x1003", "")
	// ip6tnl, административно выключен
	fakeIface(t, root, "tun-dead", "769", "0x1002", "")
	// обычный физический интерфейс — не туннель
	fakeIface(t, root, "eno1", "1", "0x1003", "")
	// мост — не туннель, хотя DEVTYPE есть
	fakeIface(t, root, "vmbr0", "1", "0x1003", "bridge")
	// wireguard опознаётся по DEVTYPE, а не по типу ARP
	fakeIface(t, root, "wg0", "65534", "0x1003", "wireguard")
	// запасное устройство ядра: есть всегда, выключено всегда, в список не идёт
	fakeIface(t, root, "ip6tnl0", "769", "0x1002", "")

	got := getTunnelStatuses()
	if len(got) != 3 {
		t.Fatalf("ожидалось 3 туннеля, получено %d: %+v", len(got), got)
	}
	// отсортировано по имени
	want := []struct {
		name string
		kind string
		up   bool
	}{
		{"tun-dead", "ip6tnl", false},
		{"tun-hw29", "ip6tnl", true},
		{"wg0", "wireguard", true},
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].Kind != w.kind || got[i].Up != w.up {
			t.Errorf("запись %d: получено %+v, ожидалось %v/%v/%v", i, got[i], w.name, w.kind, w.up)
		}
	}
}

func TestGetTunnelStatusesNone(t *testing.T) {
	root := useFixture(t)
	fakeIface(t, root, "eno1", "1", "0x1003", "")
	if got := getTunnelStatuses(); got != nil {
		t.Errorf("без туннелей ожидался nil, получено %+v", got)
	}
}

func TestGetTunnelStatusesMissingTree(t *testing.T) {
	useFixture(t)
	sysClassNet = "/nonexistent/sys/class/net"
	if got := getTunnelStatuses(); got != nil {
		t.Errorf("при отсутствии дерева ожидался nil, получено %+v", got)
	}
}

func TestInterfaceIsUp(t *testing.T) {
	root := useFixture(t)
	fakeIface(t, root, "up", "769", "0x1003", "")
	fakeIface(t, root, "down", "769", "0x1002", "")
	if !interfaceIsUp(filepath.Join(root, "up")) {
		t.Error("флаг 0x1003 должен читаться как поднятый")
	}
	if interfaceIsUp(filepath.Join(root, "down")) {
		t.Error("флаг 0x1002 должен читаться как опущенный")
	}
	if interfaceIsUp(filepath.Join(root, "absent")) {
		t.Error("отсутствующий интерфейс не должен считаться поднятым")
	}
}
