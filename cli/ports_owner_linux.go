//go:build linux

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// verifyPortOwnedByChild proves that the listening socket on the leased port
// belongs to the launcher's child process group. The preflight bind probe
// necessarily closes its probe listener, so an unrelated process can win the
// bind race and answer /health. Without this check the launcher could publish
// and advertise a foreign listener as its own child.
//
// The child process group is used instead of a single pid because the
// generated cmd/ssr process spawns the JavaScript runtime that actually binds
// the socket. Both stay in the process group the launcher created.
func verifyPortOwnedByChild(host string, port, childPID int) error {
	if childPID <= 0 {
		return fmt.Errorf("verify port %d ownership: invalid child pid %d", port, childPID)
	}
	sockets, err := listeningSocketInodes(port)
	if err != nil {
		return err
	}
	if len(sockets) == 0 {
		return fmt.Errorf(
			"port %d has no listening socket after the child reported healthy (host %s)",
			port,
			host,
		)
	}
	owned, err := processGroupSocketInodes(childPID)
	if err != nil {
		return err
	}
	for inode := range sockets {
		if owned[inode] {
			return nil
		}
	}

	return fmt.Errorf(
		"port %d is owned by another process, not the launcher child pid %d; refusing to publish a foreign listener",
		port,
		childPID,
	)
}

// listeningSocketInodes returns the socket inodes listening on port across the
// IPv4 and IPv6 tables. Only TCP state 0A (LISTEN) counts.
func listeningSocketInodes(port int) (map[string]bool, error) {
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
			_, localPort, ok := strings.Cut(fields[1], ":")
			if !ok {
				continue
			}
			listenPort, err := strconv.ParseInt(localPort, 16, 32)
			if err != nil || int(listenPort) != port {
				continue
			}
			inodes[fields[9]] = true
		}
	}
	return inodes, nil
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
