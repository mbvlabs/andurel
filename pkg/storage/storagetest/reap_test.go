package storagetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
)

var testStart = time.Date(2026, 11, 1, 12, 0, 30, 500_000_000, time.UTC)

func testOwner(dead ...int) owner {
	return owner{
		host:  "devbox",
		boot:  "boot-a",
		pidns: "pidns-a",
		pid:   100,
		start: testStart,
		alive: func(pid int) bool { return !slices.Contains(dead, pid) },
	}
}

func ownedBy(pid, host string) map[string]string {
	return map[string]string{clusterLabel: "true", ownerPIDLabel: pid, ownerHostLabel: host, ownerBootLabel: "boot-a", ownerNSLabel: "pidns-a"}
}

func TestClusterLabels(t *testing.T) {
	got := clusterLabels(owner{pid: 4242, host: "devbox", boot: "boot-a", pidns: "pidns-a"})
	want := map[string]string{
		"dev.andurel.storagetest":       "true",
		"dev.andurel.storagetest.pid":   "4242",
		"dev.andurel.storagetest.host":  "devbox",
		"dev.andurel.storagetest.boot":  "boot-a",
		"dev.andurel.storagetest.pidns": "pidns-a",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clusterLabels = %v, want %v", got, want)
	}

	noHost := clusterLabels(owner{pid: 4242})
	if _, ok := noHost[ownerHostLabel]; ok {
		t.Fatalf("clusterLabels without a hostname = %v, want no host label", noHost)
	}
}

func TestLeftovers(t *testing.T) {
	before := testStart.Add(-time.Hour)

	tests := []struct {
		name        string
		container   labelledContainer
		dead        []int
		wantStale   bool
		wantUnowned bool
	}{
		{
			name:      "exited process on this host",
			container: labelledContainer{ID: "a", Created: before, Labels: ownedBy("200", "devbox")},
			dead:      []int{200},
			wantStale: true,
		},
		{
			name:      "running process on this host",
			container: labelledContainer{ID: "a", Created: before, Labels: ownedBy("200", "devbox")},
		},
		{
			name:      "exited PID on another host",
			container: labelledContainer{ID: "a", Created: before, Labels: ownedBy("200", "laptop")},
			dead:      []int{200},
		},
		{
			name:      "this process",
			container: labelledContainer{ID: "a", Created: before, Labels: ownedBy("100", "devbox")},
			dead:      []int{100},
		},
		{
			name:      "created after this process started",
			container: labelledContainer{ID: "a", Created: testStart.Add(time.Second), Labels: ownedBy("200", "devbox")},
			dead:      []int{200},
		},
		{
			name: "created in the same second this process started",
			// Docker reports whole seconds, so this may be after the start.
			container: labelledContainer{ID: "a", Created: testStart.Truncate(time.Second), Labels: ownedBy("200", "devbox")},
			dead:      []int{200},
		},
		{
			name:      "created the second before this process started",
			container: labelledContainer{ID: "a", Created: testStart.Truncate(time.Second).Add(-time.Second), Labels: ownedBy("200", "devbox")},
			dead:      []int{200},
			wantStale: true,
		},
		{
			name:      "malformed PID",
			container: labelledContainer{ID: "a", Created: before, Labels: ownedBy("abc", "devbox")},
		},
		{
			name:      "zero PID",
			container: labelledContainer{ID: "a", Created: before, Labels: ownedBy("0", "devbox")},
			dead:      []int{0},
		},
		{
			name:      "negative PID",
			container: labelledContainer{ID: "a", Created: before, Labels: ownedBy("-1", "devbox")},
			dead:      []int{-1},
		},
		{
			name:      "not a storagetest container",
			container: labelledContainer{ID: "a", Created: before, Labels: map[string]string{ownerPIDLabel: "200", ownerHostLabel: "devbox"}},
			dead:      []int{200},
		},
		{
			name:        "no owner labels",
			container:   labelledContainer{ID: "a", Created: before, Labels: map[string]string{clusterLabel: "true"}},
			wantUnowned: true,
		},
		{
			name:        "PID label without host label",
			container:   labelledContainer{ID: "a", Created: before, Labels: map[string]string{clusterLabel: "true", ownerPIDLabel: "200"}},
			dead:        []int{200},
			wantUnowned: true,
		},
		{
			name:      "no owner labels, created after this process started",
			container: labelledContainer{ID: "a", Created: testStart.Add(time.Minute), Labels: map[string]string{clusterLabel: "true"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stale, unowned := leftovers([]labelledContainer{tt.container}, testOwner(tt.dead...))
			if got := len(stale) == 1; got != tt.wantStale {
				t.Fatalf("stale = %v, want stale %v", stale, tt.wantStale)
			}
			if got := len(unowned) == 1; got != tt.wantUnowned {
				t.Fatalf("unowned = %v, want unowned %v", unowned, tt.wantUnowned)
			}
		})
	}
}

func TestLeftoversWithoutHostname(t *testing.T) {
	self := testOwner(200)
	self.host = ""

	stale, _ := leftovers([]labelledContainer{{
		ID:      "a",
		Created: testStart.Add(-time.Hour),
		Labels:  ownedBy("200", ""),
	}}, self)
	if len(stale) != 0 {
		t.Fatalf("stale = %v, want none when the hostname is unknown", stale)
	}
}

type fakeContainers struct {
	containers []labelledContainer
	listErr    error
	removeErr  map[string]error
	removed    []string
}

func (f *fakeContainers) listLabelled(context.Context) ([]labelledContainer, error) {
	return f.containers, f.listErr
}

func (f *fakeContainers) remove(_ context.Context, id string) error {
	if err := f.removeErr[id]; err != nil {
		return err
	}
	f.removed = append(f.removed, id)
	return nil
}

func TestReapLeftoversRemovesOnlyStale(t *testing.T) {
	before := testStart.Add(-time.Hour)
	api := &fakeContainers{containers: []labelledContainer{
		{ID: "stale000000000001", Created: before, Labels: ownedBy("200", "devbox")},
		{ID: "live", Created: before, Labels: ownedBy("201", "devbox")},
		{ID: "remote", Created: before, Labels: ownedBy("200", "laptop")},
		{ID: "mine", Created: before, Labels: ownedBy("100", "devbox")},
		{ID: "new", Created: testStart.Add(time.Minute), Labels: ownedBy("200", "devbox")},
	}}

	var out bytes.Buffer
	reapLeftovers(context.Background(), api, testOwner(200), &out)

	if !reflect.DeepEqual(api.removed, []string{"stale000000000001"}) {
		t.Fatalf("removed = %v, want only the stale container", api.removed)
	}
	want := "storagetest: removed 1 leftover test container(s) from test processes that exited without cleanup\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestReapLeftoversSilentWhenNothingStale(t *testing.T) {
	api := &fakeContainers{containers: []labelledContainer{
		{ID: "live", Created: testStart.Add(-time.Hour), Labels: ownedBy("201", "devbox")},
	}}

	var out bytes.Buffer
	reapLeftovers(context.Background(), api, testOwner(), &out)

	if len(api.removed) != 0 || out.Len() != 0 {
		t.Fatalf("removed = %v, output = %q, want nothing", api.removed, out.String())
	}
}

func TestReapLeftoversListFailurePrintsCommand(t *testing.T) {
	api := &fakeContainers{listErr: errors.New("daemon gone")}

	var out bytes.Buffer
	reapLeftovers(context.Background(), api, testOwner(), &out)

	assertOneLine(t, out.String(), "daemon gone", "manually verify", "other hosts",
		"docker ps -a --filter label=dev.andurel.storagetest=true")
	if strings.Contains(out.String(), "docker rm") || strings.Contains(out.String(), "xargs") {
		t.Fatalf("listing failure suggests removal: %q", out.String())
	}
}

func TestReapLeftoversRemoveFailurePrintsCommand(t *testing.T) {
	before := testStart.Add(-time.Hour)
	api := &fakeContainers{
		containers: []labelledContainer{
			{ID: "aaaaaaaaaaaa1111", Created: before, Labels: ownedBy("200", "devbox")},
			{ID: "bbbbbbbbbbbb2222", Created: before, Labels: ownedBy("201", "devbox")},
			{ID: "gone", Created: before, Labels: ownedBy("202", "devbox")},
			{ID: "busy", Created: before, Labels: ownedBy("203", "devbox")},
			{ID: "live", Created: before, Labels: ownedBy("204", "devbox")},
			{ID: "remote", Created: before, Labels: ownedBy("200", "other-host")},
			{ID: "unknown", Created: before, Labels: ownedBy("invalid", "devbox")},
			{ID: "recent", Created: testStart, Labels: ownedBy("200", "devbox")},
		},
		removeErr: map[string]error{
			"aaaaaaaaaaaa1111": errors.New("permission denied"),
			"bbbbbbbbbbbb2222": errors.New("permission denied"),
			"gone":             fmt.Errorf("no such container: %w", cerrdefs.ErrNotFound),
			"busy":             fmt.Errorf("removal in progress: %w", cerrdefs.ErrConflict),
		},
	}

	var out bytes.Buffer
	reapLeftovers(context.Background(), api, testOwner(200, 201, 202, 203), &out)

	assertOneLine(t, out.String(), "docker rm -f aaaaaaaaaaaa bbbbbbbbbbbb")
	for _, id := range []string{"live", "remote", "unknown", "recent", "gone", "busy"} {
		if strings.Contains(out.String(), id) {
			t.Fatalf("remove failure includes non-removable container %q: %q", id, out.String())
		}
	}
}

func TestReapLeftoversReportsUnowned(t *testing.T) {
	api := &fakeContainers{containers: []labelledContainer{
		{ID: "cccccccccccc3333", Created: testStart.Add(-time.Hour), Labels: map[string]string{clusterLabel: "true"}},
	}}

	var out bytes.Buffer
	reapLeftovers(context.Background(), api, testOwner(), &out)

	if len(api.removed) != 0 {
		t.Fatalf("removed = %v, want unowned containers kept", api.removed)
	}
	assertOneLine(t, out.String(), "no verifiable owner scope", "manually verify each container is unused", "other hosts", "docker rm -f cccccccccccc")
}

func TestLeftoversRequireComparableScope(t *testing.T) {
	base := ownedBy("200", "devbox")
	for _, tc := range []struct {
		name   string
		labels map[string]string
		self   owner
	}{
		{"different namespace same hostname", map[string]string{clusterLabel: "true", ownerPIDLabel: "200", ownerHostLabel: "devbox", ownerBootLabel: "boot-a", ownerNSLabel: "pidns-b"}, testOwner(200)},
		{"different boot same hostname", map[string]string{clusterLabel: "true", ownerPIDLabel: "200", ownerHostLabel: "devbox", ownerBootLabel: "boot-b", ownerNSLabel: "pidns-a"}, testOwner(200)},
		{"legacy hostname and pid", map[string]string{clusterLabel: "true", ownerPIDLabel: "200", ownerHostLabel: "devbox"}, testOwner(200)},
		{"local scope unavailable", base, owner{host: "devbox", pid: 100, start: testStart, alive: func(int) bool { t.Fatal("liveness queried without scope"); return false }}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stale, _ := leftovers([]labelledContainer{{ID: "live", Created: testStart.Add(-time.Hour), Labels: tc.labels}}, tc.self)
			if len(stale) != 0 {
				t.Fatalf("unsafe removal: %v", stale)
			}
		})
	}
}

func assertOneLine(t *testing.T, out string, wants ...string) {
	t.Helper()

	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("output = %q, want exactly one line", out)
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want it to contain %q", out, want)
		}
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("processAlive(own PID) = false, want true")
	}

	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run child process: %v", err)
	}
	if processAlive(cmd.Process.Pid) {
		t.Fatalf("processAlive(exited child %d) = true, want false", cmd.Process.Pid)
	}
}
