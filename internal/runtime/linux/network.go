// Bridge network mode: each session gets a veth pair. The host end (skv_<id>) is
// attached to bridge sk0; the container end (skc_<id>) is moved into the session's
// network namespace and renamed eth0. IPs are allocated from 10.55.0.0/16; NAT
// masquerade allows outbound traffic. Setup is lazy (on first Exec) to save ~50-150ms
// at session create.
package linux

import (
	"fmt"
	"net"
	"os/exec"
	"sync"
)

const (
	BridgeName = "sk0"
	BridgeIP   = "10.55.0.1/16"
	Subnet     = "10.55.0.0/16"
)

var (
	ipPoolMu   sync.Mutex
	usedIPs    = make(map[string]bool)  // IP -> in use
	sessionIPs = make(map[string]string) // sessionID -> IP
	nextIP     = net.ParseIP("10.55.0.2")
)

// incIP increments an IPv4 address in place (big-endian).
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// AllocateIP assigns a unique IP from the pool to the session. Wraps around the subnet
// when exhausted; returns error only if the entire pool is in use.
func AllocateIP(sessionID string) (string, error) {
	ipPoolMu.Lock()
	defer ipPoolMu.Unlock()

	startIP := make(net.IP, len(nextIP))
	copy(startIP, nextIP)

	for {
		ipStr := nextIP.String()
		if !usedIPs[ipStr] {
			usedIPs[ipStr] = true
			sessionIPs[sessionID] = ipStr
			incIP(nextIP)
			return ipStr, nil
		}
		incIP(nextIP)

		// Wrap around when we've left 10.55.0.0/16 subnet.
		// For IPv4-mapped format, bytes [12:14] are the network octets (10.55).
		if len(nextIP) >= 14 && (nextIP[12] != 10 || nextIP[13] != 55) {
			nextIP = net.ParseIP("10.55.0.2")
		}

		if nextIP.Equal(startIP) {
			return "", fmt.Errorf("ip pool exhausted")
		}
	}
}

// ReleaseIP returns the session's IP to the pool. Idempotent if session had no IP.
func ReleaseIP(sessionID string) {
	ipPoolMu.Lock()
	defer ipPoolMu.Unlock()
	if ip, ok := sessionIPs[sessionID]; ok {
		delete(usedIPs, ip)
		delete(sessionIPs, sessionID)
	}
}

// GetIPForSession returns the allocated IP for a session, or empty string if none.
func GetIPForSession(sessionID string) string {
	ipPoolMu.Lock()
	defer ipPoolMu.Unlock()
	return sessionIPs[sessionID]
}

// SetupHostBridge creates the sk0 bridge with 10.55.0.1/16, enables it, adds iptables
// MASQUERADE for the subnet, and enables ip_forward. Idempotent; skips if bridge exists.
func SetupHostBridge() error {
	if err := exec.Command("ip", "link", "show", BridgeName).Run(); err == nil {
		// Already exists
		return nil
	}

	commands := [][]string{
		{"ip", "link", "add", "name", BridgeName, "type", "bridge"},
		{"ip", "addr", "add", BridgeIP, "dev", BridgeName},
		{"ip", "link", "set", "dev", BridgeName, "up"},
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-s", Subnet, "!", "-o", BridgeName, "-j", "MASQUERADE"},
		{"sysctl", "-w", "net.ipv4.ip_forward=1"},
	}

	for _, cmd := range commands {
		if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("command %v failed: %v, output: %s", cmd, err, out)
		}
	}

	return nil
}

// SetupSessionNetwork creates a veth pair, attaches the host end to sk0, moves the
// container end into the session's net ns (via pid), renames it to eth0, assigns the IP,
// and sets the default route via 10.55.0.1. Call with nsinit already running (pid valid).
func SetupSessionNetwork(sessionID string, pid int, ip string) error {
	vethHost := "skv_" + sessionID[:8]
	vethCont := "skc_" + sessionID[:8]

	commands := [][]string{
		{"ip", "link", "add", "name", vethHost, "type", "veth", "peer", "name", vethCont},
		// Attach host side to bridge
		{"ip", "link", "set", "dev", vethHost, "master", BridgeName},
		{"ip", "link", "set", "dev", vethHost, "up"},
		// Move container side to namespace
		{"ip", "link", "set", "dev", vethCont, "netns", fmt.Sprintf("%d", pid)},
		// Rename to eth0 inside namespace
		{"nsenter", "-t", fmt.Sprintf("%d", pid), "-n", "ip", "link", "set", "dev", vethCont, "name", "eth0"},
		// Set IP inside namespace
		{"nsenter", "-t", fmt.Sprintf("%d", pid), "-n", "ip", "addr", "add", ip + "/16", "dev", "eth0"},
		// Bring interfaces up inside namespace
		{"nsenter", "-t", fmt.Sprintf("%d", pid), "-n", "ip", "link", "set", "dev", "eth0", "up"},
		{"nsenter", "-t", fmt.Sprintf("%d", pid), "-n", "ip", "link", "set", "dev", "lo", "up"},
		// Set default route inside namespace
		{"nsenter", "-t", fmt.Sprintf("%d", pid), "-n", "ip", "route", "add", "default", "via", "10.55.0.1"},
	}

	for _, cmd := range commands {
		if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("setup network command %v failed: %v, output: %s", cmd, err, out)
		}
	}

	// Write the resolv.conf file for the container
	return nil
}
