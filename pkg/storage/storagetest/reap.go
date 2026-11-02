package storagetest

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
)

// Labels on every container started by newCluster. clusterLabel marks the
// container as storagetest's; the owner labels name the test process that
// started it, so a later run can tell a leftover from a container in use.
const (
	clusterLabel   = "dev.andurel.storagetest"
	ownerPIDLabel  = clusterLabel + ".pid"
	ownerHostLabel = clusterLabel + ".host"
	ownerBootLabel = clusterLabel + ".boot"
	ownerNSLabel   = clusterLabel + ".pidns"
)

// reapTimeout bounds the leftover check, so a slow Docker daemon cannot hold up
// a test package's start for long.
const reapTimeout = 30 * time.Second

// processStart is when this test process loaded the package. Only containers
// created before it are considered leftovers.
var processStart = time.Now()

// clusterLabels returns the labels for a container started by this process.
func clusterLabels(self owner) map[string]string {
	labels := map[string]string{
		clusterLabel:  "true",
		ownerPIDLabel: strconv.Itoa(self.pid),
	}
	if self.host != "" {
		labels[ownerHostLabel] = self.host
	}
	if self.boot != "" && self.pidns != "" {
		labels[ownerBootLabel] = self.boot
		labels[ownerNSLabel] = self.pidns
	}
	return labels
}

// labelledContainer is the part of a Docker container listing the leftover
// check needs.
type labelledContainer struct {
	ID      string
	Created time.Time
	Labels  map[string]string
}

// owner describes the current test process: the host and PID it runs as, when
// it started, and how to ask whether another PID on this host is still running.
type owner struct {
	host  string
	boot  string
	pidns string
	pid   int
	start time.Time
	alive func(pid int) bool
}

func currentOwner() owner {
	host, _ := os.Hostname()
	boot, pidns := processScope()
	return owner{
		host:  host,
		boot:  boot,
		pidns: pidns,
		pid:   os.Getpid(),
		start: processStart,
		alive: processAlive,
	}
}

// leftovers sorts labelled containers into those safe to remove and those that
// predate the owner labels. A container is stale only when all of these hold:
//
//   - Docker reports it created in a whole second before this process started
//     (Docker's listing has one-second resolution, so a container created in the
//     same second as this process is never touched);
//   - its Linux boot ID and PID namespace match this process's verified scope;
//   - its PID label is a positive number other than this process's PID;
//   - that PID is no longer running.
//
// Missing scope labels, including all legacy containers, are never removed.
// PID reuse is conservative: any currently live PID keeps the container.
func leftovers(containers []labelledContainer, self owner) (stale, unowned []string) {
	for _, c := range containers {
		if c.Labels[clusterLabel] != "true" {
			continue
		}
		if c.Created.Unix() >= self.start.Unix() {
			continue
		}

		pidText, hasPID := c.Labels[ownerPIDLabel]
		boot, ns := c.Labels[ownerBootLabel], c.Labels[ownerNSLabel]
		if !hasPID || boot == "" || ns == "" {
			unowned = append(unowned, c.ID)
			continue
		}
		if self.host == "" || c.Labels[ownerHostLabel] != self.host || self.boot == "" || self.pidns == "" || boot != self.boot || ns != self.pidns {
			continue
		}

		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 0 || pid == self.pid {
			continue
		}
		if self.alive(pid) {
			continue
		}
		stale = append(stale, c.ID)
	}
	return stale, unowned
}

// containerAPI is the part of the Docker API the leftover check uses.
type containerAPI interface {
	listLabelled(ctx context.Context) ([]labelledContainer, error)
	remove(ctx context.Context, id string) error
}

// reapLeftovers removes storagetest containers whose test process has exited,
// for example after a panic or a `go test -timeout`. It never fails the run:
// when it cannot list or remove containers it writes guidance to w for manual
// inspection or removal of the identified leftovers.
func reapLeftovers(ctx context.Context, api containerAPI, self owner, w io.Writer) {
	containers, err := api.listLabelled(ctx)
	if err != nil {
		writeCheckFailed(w, err)
		return
	}

	stale, unowned := leftovers(containers, self)

	var failed []string
	removed := 0
	for _, id := range stale {
		err := api.remove(ctx, id)
		switch {
		case err == nil:
			removed++
		case cerrdefs.IsNotFound(err) || cerrdefs.IsConflict(err):
			// Another test package removed it first, or is removing it now.
		default:
			failed = append(failed, shortID(id))
		}
	}

	if removed > 0 {
		fmt.Fprintf(w, "storagetest: removed %d leftover test container(s) from test processes that exited without cleanup\n", removed)
	}
	if len(failed) > 0 {
		fmt.Fprintf(w, "storagetest: could not remove leftover test container(s); remove them with: docker rm -f %s\n",
			strings.Join(failed, " "))
	}
	if len(unowned) > 0 {
		ids := make([]string, len(unowned))
		for i, id := range unowned {
			ids[i] = shortID(id)
		}
		fmt.Fprintf(w, "storagetest: %d test container(s) have no verifiable owner scope; manually verify each container is unused, including by tests on other hosts and PID namespaces, before removing with: docker rm -f %s\n",
			len(unowned), strings.Join(ids, " "))
	}
}

// writeCheckFailed reports that the leftover check could not run. Without a
// listing it cannot tell leftovers from containers in use, so it only suggests
// listing containers and manually checking their owners.
func writeCheckFailed(w io.Writer, err error) {
	fmt.Fprintf(w,
		"storagetest: could not check for leftover test containers (%v); inspect with: docker ps -a --filter label=%s=true; manually verify each container's owner and that it is unused, including by tests on other hosts, before removing it\n",
		err, clusterLabel)
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// dockerContainers implements containerAPI with testcontainers' Docker client.
type dockerContainers struct {
	client *testcontainers.DockerClient
}

func (d dockerContainers) listLabelled(ctx context.Context) ([]labelledContainer, error) {
	result, err := d.client.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", clusterLabel+"=true"),
	})
	if err != nil {
		return nil, err
	}

	containers := make([]labelledContainer, 0, len(result.Items))
	for _, item := range result.Items {
		containers = append(containers, labelledContainer{
			ID:      item.ID,
			Created: time.Unix(item.Created, 0),
			Labels:  item.Labels,
		})
	}
	return containers, nil
}

func (d dockerContainers) remove(ctx context.Context, id string) error {
	_, err := d.client.ContainerRemove(ctx, id, client.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	return err
}

// reapDockerLeftovers runs reapLeftovers against the Docker daemon
// testcontainers uses, writing any message to stderr.
func reapDockerLeftovers(ctx context.Context, self owner) {
	ctx, cancel := context.WithTimeout(ctx, reapTimeout)
	defer cancel()

	dockerClient, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		writeCheckFailed(os.Stderr, err)
		return
	}
	defer func() {
		_ = dockerClient.Close()
	}()

	reapLeftovers(ctx, dockerContainers{client: dockerClient}, self, os.Stderr)
}
