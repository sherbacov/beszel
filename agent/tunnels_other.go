//go:build !linux

package agent

import "github.com/henrygd/beszel/internal/entities/system"

// getTunnelStatuses is Linux-only: it reads interface encapsulation and flags
// from sysfs, which has no counterpart on the other supported platforms.
func getTunnelStatuses() []system.TunnelStatus { return nil }
