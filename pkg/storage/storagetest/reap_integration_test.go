package storagetest

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

const leakHelperEnv = "STORAGETEST_LEAK_HELPER"

// TestScopedReapProbe is an opt-in runtime probe. Its Docker API is restricted
// to one explicitly supplied container ID, so it cannot remove other resources.
func TestScopedReapProbe(t *testing.T) {
	id := os.Getenv("STORAGETEST_SCOPE_PROBE_ID")
	if id == "" {
		t.Skip("explicit own-container ID required")
	}
	ctx := context.Background()
	client, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	api := probeContainerAPI{client: client, id: id}
	var report bytes.Buffer
	reapLeftovers(ctx, api, currentOwner(), &report)
	t.Logf("probe %s: %s", id, report.String())
}

type probeContainerAPI struct {
	dockerContainers
	id string
}

func (p probeContainerAPI) listLabelled(ctx context.Context) ([]labelledContainer, error) {
	all, err := p.dockerContainers.listLabelled(ctx)
	if err != nil {
		return nil, err
	}
	var own []labelledContainer
	for _, c := range all {
		if c.ID == p.id {
			own = append(own, c)
		}
	}
	return own, nil
}

func (p probeContainerAPI) remove(ctx context.Context, id string) error {
	if id != p.id {
		panic("scope probe attempted to remove an unrelated container")
	}
	return p.dockerContainers.remove(ctx, id)
}

// TestLeakClusterHelper is not a test on its own. TestReapRemovesLeakedContainer
// runs it in a child process, where it starts a cluster, prints the container
// ID and panics before Close, as a failing test package would. With the
// variable set to "hang" it blocks instead, so `go test -timeout` kills it.
func TestLeakClusterHelper(t *testing.T) {
	mode := os.Getenv(leakHelperEnv)
	if mode == "" {
		t.Skip("helper for TestReapRemovesLeakedContainer")
	}

	cluster, err := NewCluster(context.Background())
	if err != nil {
		t.Fatalf("start cluster: %v", err)
	}
	os.Stdout.WriteString("container=" + cluster.container.GetContainerID() + "\n")
	if mode == "hang" {
		time.Sleep(time.Hour)
	}
	panic("storagetest: simulated test panic before Cluster.Close")
}

// TestReapRemovesLeakedContainer needs Docker. A child test process leaks a
// container by panicking; the leftover check then removes it, while the
// container of this live process stays.
func TestReapRemovesLeakedContainer(t *testing.T) {
	if boot, ns := processScope(); boot == "" || ns == "" {
		t.Skip("automatic reaping requires a verifiable process scope")
	}
	own := newTestCluster(t)
	ownID := own.container.GetContainerID()

	child := exec.Command(os.Args[0], "-test.run=^TestLeakClusterHelper$", "-test.v")
	child.Env = append(os.Environ(), leakHelperEnv+"=1")
	out, err := child.CombinedOutput()
	if err == nil {
		t.Fatalf("leak helper exited cleanly, want a panic; output:\n%s", out)
	}
	leakedID := ""
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		if id, ok := strings.CutPrefix(scanner.Text(), "container="); ok {
			leakedID = id
		}
	}
	if leakedID == "" {
		t.Fatalf("leak helper printed no container ID; output:\n%s", out)
	}

	ctx := context.Background()
	dockerClient, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}
	t.Cleanup(func() {
		_ = dockerClient.Close()
	})
	api := dockerContainers{client: dockerClient}
	t.Cleanup(func() {
		_ = api.remove(context.Background(), leakedID)
	})

	if ids := labelledIDs(t, api); !slices.Contains(ids, leakedID) {
		t.Fatalf("leaked container %s not listed after the helper panicked; listed %v", leakedID, ids)
	}

	// The leak happened after this process started, so reap as a process that
	// starts now, as the next `go test` run would. Docker's creation time has
	// one-second resolution.
	self := currentOwner()
	self.start = time.Now().Add(time.Second)

	var report bytes.Buffer
	reapLeftovers(ctx, api, self, &report)

	ids := labelledIDs(t, api)
	if slices.Contains(ids, leakedID) {
		t.Fatalf("leaked container %s still present after reaping; report %q", leakedID, report.String())
	}
	if !slices.Contains(ids, ownID) {
		t.Fatalf("reaping removed this process's own container %s", ownID)
	}
	if !strings.Contains(report.String(), "storagetest: removed ") {
		t.Fatalf("report = %q, want the removal reported", report.String())
	}
}

func labelledIDs(t *testing.T, api dockerContainers) []string {
	t.Helper()

	containers, err := api.listLabelled(context.Background())
	if err != nil {
		t.Fatalf("list labelled containers: %v", err)
	}
	ids := make([]string, 0, len(containers))
	for _, c := range containers {
		ids = append(ids, c.ID)
	}
	return ids
}
