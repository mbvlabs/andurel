//go:build linux

package cli

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// verifyPortOwnedByChild proves that the listening socket on the exact host and
// port used by the health probe belongs to the launcher's child process group.
// The preflight bind probe necessarily closes its probe listener, so an
// unrelated process can win the bind race and answer /health. Matching only the
// port is insufficient: a child on 127.0.0.2:P must not prove ownership while
// an unrelated listener answers 127.0.0.1:P. Without this check the launcher
// could publish and advertise a foreign listener as its own child.
//
// The child process group is used instead of a single pid because the
// generated cmd/ssr process spawns the JavaScript runtime that actually binds
// the socket. Both stay in the process group the launcher created.
func verifyPortOwnedByChild(host string, port, childPID int) error {
	if childPID <= 0 {
		return fmt.Errorf("verify port %d ownership: invalid child pid %d", port, childPID)
	}
	requested := net.ParseIP(host)
	if requested == nil {
		return fmt.Errorf("verify port %d ownership: invalid host %q", port, host)
	}
	if requested.IsUnspecified() {
		return fmt.Errorf(
			"verify port %d ownership: wildcard host %q cannot identify the socket that answered the health probe",
			port,
			host,
		)
	}

	sockets, err := listeningSocketInodes(requested, port)
	if err != nil {
		return err
	}
	if len(sockets) == 0 {
		return fmt.Errorf(
			"port %d has no listening socket on host %s after the child reported healthy",
			port,
			host,
		)
	}
	owned, err := processGroupSocketInodes(childPID)
	if err != nil {
		return err
	}
	for inode := range sockets {
		if !owned[inode] {
			return fmt.Errorf(
				"port %d on host %s is owned by another process, not the launcher child pid %d; refusing to publish a foreign listener",
				port,
				host,
				childPID,
			)
		}
	}

	return nil
}

// listeningSocketInodes returns the inodes listening on the exact local
// address and port across the IPv4 and IPv6 tables. Only TCP state 0A (LISTEN)
// counts. Wildcard listeners do not match a specific requested address, so the
// launcher fails closed instead of assuming that an any-address socket is the
// one the health probe reached.
func listeningSocketInodes(requested net.IP, port int) (map[string]bool, error) {
	inodes := map[string]bool{}
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		body, err := os.ReadFile(table)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", table, err)
		}
		for line := range strings.SplitSeq(string(body), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[3] != "0A" {
				continue
			}
			localAddress, localPort, ok := strings.Cut(fields[1], ":")
			if !ok {
				continue
			}
			listenPort, err := strconv.ParseUint(localPort, 16, 16)
			if err != nil {
				return nil, fmt.Errorf("parse %s local port %q: %w", table, localPort, err)
			}
			if int(listenPort) != port {
				continue
			}
			address, err := parseProcNetAddress(localAddress)
			if err != nil {
				return nil, fmt.Errorf("parse %s local address %q: %w", table, localAddress, err)
			}
			if !address.Equal(requested) {
				continue
			}
			inodes[fields[9]] = true
		}
	}
	return inodes, nil
}

// parseProcNetAddress decodes the little-endian address representation used by
// /proc/net/tcp and /proc/net/tcp6. IPv6 is stored as four little-endian 32-bit
// words rather than as a byte-reversed 16-byte value.
func parseProcNetAddress(text string) (net.IP, error) {
	raw, err := hex.DecodeString(text)
	if err != nil {
		return nil, err
	}
	switch len(raw) {
	case net.IPv4len:
		return net.IPv4(raw[3], raw[2], raw[1], raw[0]), nil
	case net.IPv6len:
		address := make(net.IP, net.IPv6len)
		for offset := 0; offset < net.IPv6len; offset += 4 {
			address[offset] = raw[offset+3]
			address[offset+1] = raw[offset+2]
			address[offset+2] = raw[offset+1]
			address[offset+3] = raw[offset]
		}
		return address, nil
	default:
		return nil, fmt.Errorf("unexpected address length %d", len(raw))
	}
}

// processGroupSocketInodes returns the socket inodes held by the process group
// whose leader is leaderPID.
func processGroupSocketInodes(leaderPID int) (map[string]bool, error) {
	inodes := map[string]bool{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		group, err := processGroup(pid)
		if err != nil || group != leaderPID {
			continue
		}
		fdEntries, err := os.ReadDir(filepath.Join("/proc", entry.Name(), "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fdEntries {
			target, err := os.Readlink(filepath.Join("/proc", entry.Name(), "fd", fd.Name()))
			if err != nil {
				continue
			}
			if inode, ok := socketInode(target); ok {
				inodes[inode] = true
			}
		}
	}
	return inodes, nil
}

func processGroup(pid int) (int, error) {
	body, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, err
	}
	// The comm field is parenthesized and may contain spaces or parentheses.
	closeParen := strings.LastIndexByte(string(body), ')')
	if closeParen < 0 {
		return 0, fmt.Errorf("parse /proc/%d/stat", pid)
	}
	fields := strings.Fields(string(body)[closeParen+1:])
	if len(fields) < 3 {
		return 0, fmt.Errorf("parse /proc/%d/stat", pid)
	}
	return strconv.Atoi(fields[2])
}

func socketInode(target string) (string, bool) {
	trimmed, ok := strings.CutPrefix(target, "socket:[")
	if !ok {
		return "", false
	}
	inode, ok := strings.CutSuffix(trimmed, "]")
	return inode, ok
}
