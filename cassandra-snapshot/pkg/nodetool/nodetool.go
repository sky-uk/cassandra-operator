package nodetool

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	nodetoolArguments = "NODETOOL_ARGS"
)

// Nodetool provides an interface to nodetool functions running on Cassandra pods within a Kubernetes cluster.
type Nodetool struct {
	kubeClientset *kubernetes.Clientset
	restConfig    *rest.Config
}

// Snapshot describes properties which identify a keyspace snapshot.
type Snapshot struct {
	Name         string
	Keyspace     string
	ColumnFamily string
}

// SnapshotFilter is an interface describing a function which allows Snapshots to be filtered based on particular
// properties.
type SnapshotFilter func([]Snapshot) []Snapshot

// New creates a new Nodetool using the supplied client and REST configuration to connect to Kubernetes.
func New(kubeClientset *kubernetes.Clientset, restConfig *rest.Config) *Nodetool {
	return &Nodetool{kubeClientset: kubeClientset, restConfig: restConfig}
}

// CreateSnapshot creates a Snapshot on a given Pod, covering a supplied set of keyspaces and with a name derived from
// a timestamp.
func (n *Nodetool) CreateSnapshot(snapshotTimestamp time.Time, keyspaces []string, pod *v1.Pod, snapshotCreationTimeout time.Duration) error {
	snapshotName := strconv.FormatInt(snapshotTimestamp.Unix(), 10)

	log.Infof("Creating Snapshot %s for pod %s and keyspaces %v", snapshotName, pod.Name, keyspaces)
	command := fmt.Sprintf("nodetool ${%s} snapshot -t %s -- %s", nodetoolArguments, snapshotName, strings.Join(keyspaces, " "))

	_, err := n.runCommand(pod, snapshotCreationTimeout, command)
	return err
}

// GetSnapshots returns the Snapshots found on a given Pod, filtered through the supplied SnapshotFilter.
func (n *Nodetool) GetSnapshots(pod *v1.Pod, timeout time.Duration, filter SnapshotFilter) ([]Snapshot, error) {
	var snapshots []Snapshot
	command := fmt.Sprintf("nodetool ${%s} listsnapshots", nodetoolArguments)
	output, err := n.runCommand(pod, timeout, command)
	if err != nil {
		return snapshots, fmt.Errorf("error while listing snapshots on pod %s: %v", pod.Name, err)
	}

	re := regexp.MustCompile("^(\\d+) +(\\w+) +(\\w+)")
	for _, snapshotLine := range strings.Split(string(output), "\n") {
		lineAsBytes := []byte(snapshotLine)
		if re.Match(lineAsBytes) {
			submatches := re.FindAllStringSubmatch(snapshotLine, -1)
			snapshot := Snapshot{submatches[0][1], submatches[0][2], submatches[0][3]}
			snapshots = append(snapshots, snapshot)
		}
	}

	return filter(snapshots), nil
}

// DeleteSnapshot deletes the given Snapshot from the given Pod.
func (n *Nodetool) DeleteSnapshot(pod *v1.Pod, snapshot *Snapshot, timeout time.Duration) error {
	command := fmt.Sprintf("nodetool ${%s} clearsnapshot -t %s -- %s", nodetoolArguments, snapshot.Name, snapshot.Keyspace)
	_, err := n.runCommand(pod, timeout, command)
	return err
}

func (n *Nodetool) runCommand(pod *v1.Pod, timeout time.Duration, command string) (string, error) {
	execRequest := n.kubeClientset.CoreV1().RESTClient().Post().
		Timeout(timeout).
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("container", "cassandra")

	args := []string{"/bin/sh", "-c", command}

	for _, arg := range args {
		execRequest = execRequest.Param("command", arg)
	}

	executor, err := remotecommand.NewSPDYExecutor(n.restConfig, "POST", execRequest.URL())
	if err != nil {
		return "", err
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: stdOut,
		Stderr: stdErr,
	})

	if err != nil {
		if exitErr, ok := err.(exec.ExitError); ok && exitErr.Exited() {
			return "", fmt.Errorf("`%s` failed with exit code %d: %v. stdout: %s. sterr: %s", strings.Join(args, " "), exitErr.ExitStatus(), err, stdOut.String(), stdErr.String())
		}
		return "", fmt.Errorf("`%s` failed with unknown exit code: %v. stdout: %s. sterr: %s", strings.Join(args, " "), err, stdOut.String(), stdErr.String())
	}

	if stdErr.String() != "" {
		return "", fmt.Errorf("`%s` failed with stdout: %s. sterr: %s", strings.Join(args, " "), stdOut.String(), stdErr.String())
	}

	return stdOut.String(), nil
}
