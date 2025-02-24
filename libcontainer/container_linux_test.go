// +build linux

package libcontainer

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libsysbox/sysbox"
)

type mockCgroupManager struct {
	pids    []int
	allPids []int
	stats   *cgroups.Stats
	paths   map[string]string
}

type mockIntelRdtManager struct {
	stats *intelrdt.Stats
	path  string
}

func (m *mockCgroupManager) GetPids() ([]int, error) {
	return m.pids, nil
}

func (m *mockCgroupManager) GetAllPids() ([]int, error) {
	return m.allPids, nil
}

func (m *mockCgroupManager) GetStats() (*cgroups.Stats, error) {
	return m.stats, nil
}

func (m *mockCgroupManager) Apply(pid int) error {
	return nil
}

func (m *mockCgroupManager) Set(container *configs.Config) error {
	return nil
}

func (m *mockCgroupManager) Destroy() error {
	return nil
}

func (m *mockCgroupManager) Exists() bool {
	_, err := os.Lstat(m.Path("devices"))
	return err == nil
}

func (m *mockCgroupManager) GetPaths() map[string]string {
	return m.paths
}

func (m *mockCgroupManager) Path(subsys string) string {
	return m.paths[subsys]
}

func (m *mockCgroupManager) Freeze(state configs.FreezerState) error {
	return nil
}

func (m *mockCgroupManager) GetCgroups() (*configs.Cgroup, error) {
	return nil, nil
}

func (m *mockCgroupManager) GetFreezerState() (configs.FreezerState, error) {
	return configs.Thawed, nil
}

func (m *mockCgroupManager) CreateChildCgroup(container *configs.Config) error {
	return nil
}

func (m *mockCgroupManager) ApplyChildCgroup(pid int) error {
	return nil
}

func (m *mockCgroupManager) GetChildCgroupPaths() map[string]string {
	return m.paths
}

func (m *mockCgroupManager) GetType() cgroups.CgroupType {
	return cgroups.Cgroup_v1_fs
}

func (m *mockIntelRdtManager) Apply(pid int) error {
	return nil
}

func (m *mockIntelRdtManager) GetStats() (*intelrdt.Stats, error) {
	return m.stats, nil
}

func (m *mockIntelRdtManager) Destroy() error {
	return nil
}

func (m *mockIntelRdtManager) GetPath() string {
	return m.path
}

func (m *mockIntelRdtManager) Set(container *configs.Config) error {
	return nil
}

func (m *mockIntelRdtManager) GetCgroups() (*configs.Cgroup, error) {
	return nil, nil
}

type mockProcess struct {
	_pid    int
	started uint64
}

func (m *mockProcess) terminate() error {
	return nil
}

func (m *mockProcess) pid() int {
	return m._pid
}

func (m *mockProcess) startTime() (uint64, error) {
	return m.started, nil
}

func (m *mockProcess) start() error {
	return nil
}

func (m *mockProcess) wait() (*os.ProcessState, error) {
	return nil, nil
}

func (m *mockProcess) signal(_ os.Signal) error {
	return nil
}

func (m *mockProcess) externalDescriptors() []string {
	return []string{}
}

func (m *mockProcess) setExternalDescriptors(newFds []string) {
}

func (m *mockProcess) forwardChildLogs() {
}

func TestGetContainerPids(t *testing.T) {
	pid := 1
	stat, err := system.Stat(pid)
	if err != nil {
		t.Fatalf("can't stat pid %d, got %v", pid, err)
	}
	container := &linuxContainer{
		id:     "myid",
		config: &configs.Config{},
		sysMgr: sysbox.NewMgr("myid", false),
		sysFs:  sysbox.NewFs("myid", false),
		cgroupManager: &mockCgroupManager{
			allPids: []int{1, 2, 3},
			paths: map[string]string{
				"device": "/proc/self/cgroups",
			},
		},
		initProcess: &mockProcess{
			_pid:    1,
			started: 10,
		},
		initProcessStartTime: stat.StartTime,
	}
	container.state = &runningState{c: container}
	pids, err := container.Processes()
	if err != nil {
		t.Fatal(err)
	}
	for i, expected := range []int{1, 2, 3} {
		if pids[i] != expected {
			t.Fatalf("expected pid %d but received %d", expected, pids[i])
		}
	}
}

func TestGetContainerStats(t *testing.T) {
	container := &linuxContainer{
		id:     "myid",
		config: &configs.Config{},
		cgroupManager: &mockCgroupManager{
			pids: []int{1, 2, 3},
			stats: &cgroups.Stats{
				MemoryStats: cgroups.MemoryStats{
					Usage: cgroups.MemoryData{
						Usage: 1024,
					},
				},
			},
		},
		intelRdtManager: &mockIntelRdtManager{
			stats: &intelrdt.Stats{
				L3CacheSchema: "L3:0=f;1=f0",
				MemBwSchema:   "MB:0=20;1=70",
			},
		},
		sysMgr: sysbox.NewMgr("myid", false),
		sysFs:  sysbox.NewFs("myid", false),
	}
	stats, err := container.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.CgroupStats == nil {
		t.Fatal("cgroup stats are nil")
	}
	if stats.CgroupStats.MemoryStats.Usage.Usage != 1024 {
		t.Fatalf("expected memory usage 1024 but received %d", stats.CgroupStats.MemoryStats.Usage.Usage)
	}
	if intelrdt.IsCATEnabled() {
		if stats.IntelRdtStats == nil {
			t.Fatal("intel rdt stats are nil")
		}
		if stats.IntelRdtStats.L3CacheSchema != "L3:0=f;1=f0" {
			t.Fatalf("expected L3CacheSchema L3:0=f;1=f0 but received %s", stats.IntelRdtStats.L3CacheSchema)
		}
	}
	if intelrdt.IsMBAEnabled() {
		if stats.IntelRdtStats == nil {
			t.Fatal("intel rdt stats are nil")
		}
		if stats.IntelRdtStats.MemBwSchema != "MB:0=20;1=70" {
			t.Fatalf("expected MemBwSchema MB:0=20;1=70 but received %s", stats.IntelRdtStats.MemBwSchema)
		}
	}
}

func TestGetContainerState(t *testing.T) {
	var (
		pid                  = os.Getpid()
		expectedMemoryPath   = "/sys/fs/cgroup/memory/myid"
		expectedNetworkPath  = fmt.Sprintf("/proc/%d/ns/net", pid)
		expectedIntelRdtPath = "/sys/fs/resctrl/myid"
	)
	container := &linuxContainer{
		id: "myid",
		config: &configs.Config{
			Namespaces: []configs.Namespace{
				{Type: configs.NEWPID},
				{Type: configs.NEWNS},
				{Type: configs.NEWNET, Path: expectedNetworkPath},
				{Type: configs.NEWUTS},
				// emulate host for IPC
				//{Type: configs.NEWIPC},
				{Type: configs.NEWCGROUP},
			},
		},
		initProcess: &mockProcess{
			_pid:    pid,
			started: 10,
		},
		cgroupManager: &mockCgroupManager{
			pids: []int{1, 2, 3},
			stats: &cgroups.Stats{
				MemoryStats: cgroups.MemoryStats{
					Usage: cgroups.MemoryData{
						Usage: 1024,
					},
				},
			},
			paths: map[string]string{
				"memory": expectedMemoryPath,
			},
		},
		intelRdtManager: &mockIntelRdtManager{
			stats: &intelrdt.Stats{
				L3CacheSchema: "L3:0=f0;1=f",
				MemBwSchema:   "MB:0=70;1=20",
			},
			path: expectedIntelRdtPath,
		},
		sysMgr: sysbox.NewMgr("myid", false),
		sysFs:  sysbox.NewFs("myid", false),
	}
	container.state = &createdState{c: container}
	state, err := container.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.InitProcessPid != pid {
		t.Fatalf("expected pid %d but received %d", pid, state.InitProcessPid)
	}
	if state.InitProcessStartTime != 10 {
		t.Fatalf("expected process start time 10 but received %d", state.InitProcessStartTime)
	}
	paths := state.CgroupPaths
	if paths == nil {
		t.Fatal("cgroup paths should not be nil")
	}
	if memPath := paths["memory"]; memPath != expectedMemoryPath {
		t.Fatalf("expected memory path %q but received %q", expectedMemoryPath, memPath)
	}
	if intelrdt.IsCATEnabled() || intelrdt.IsMBAEnabled() {
		intelRdtPath := state.IntelRdtPath
		if intelRdtPath == "" {
			t.Fatal("intel rdt path should not be empty")
		}
		if intelRdtPath != expectedIntelRdtPath {
			t.Fatalf("expected intel rdt path %q but received %q", expectedIntelRdtPath, intelRdtPath)
		}
	}
	for _, ns := range container.config.Namespaces {
		path := state.NamespacePaths[ns.Type]
		if path == "" {
			t.Fatalf("expected non nil namespace path for %s", ns.Type)
		}
		if ns.Type == configs.NEWNET {
			if path != expectedNetworkPath {
				t.Fatalf("expected path %q but received %q", expectedNetworkPath, path)
			}
		} else {
			file := ""
			switch ns.Type {
			case configs.NEWNET:
				file = "net"
			case configs.NEWNS:
				file = "mnt"
			case configs.NEWPID:
				file = "pid"
			case configs.NEWIPC:
				file = "ipc"
			case configs.NEWUSER:
				file = "user"
			case configs.NEWUTS:
				file = "uts"
			case configs.NEWCGROUP:
				file = "cgroup"
			}
			expected := fmt.Sprintf("/proc/%d/ns/%s", pid, file)
			if expected != path {
				t.Fatalf("expected path %q but received %q", expected, path)
			}
		}
	}
}

func TestGetContainerStateAfterUpdate(t *testing.T) {
	var (
		pid = os.Getpid()
	)
	stat, err := system.Stat(pid)
	if err != nil {
		t.Fatal(err)
	}

	rootDir, err := ioutil.TempDir("", "TestGetContainerStateAfterUpdate")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	container := &linuxContainer{
		root: rootDir,
		id:   "myid",
		config: &configs.Config{
			Namespaces: []configs.Namespace{
				{Type: configs.NEWPID},
				{Type: configs.NEWNS},
				{Type: configs.NEWNET},
				{Type: configs.NEWUTS},
				{Type: configs.NEWIPC},
			},
			Cgroups: &configs.Cgroup{
				Resources: &configs.Resources{
					Memory: 1024,
				},
			},
		},
		initProcess: &mockProcess{
			_pid:    pid,
			started: stat.StartTime,
		},
		cgroupManager: &mockCgroupManager{},
		sysMgr:        sysbox.NewMgr("myid", false),
		sysFs:         sysbox.NewFs("myid", false),
	}
	container.state = &createdState{c: container}
	state, err := container.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.InitProcessPid != pid {
		t.Fatalf("expected pid %d but received %d", pid, state.InitProcessPid)
	}
	if state.InitProcessStartTime != stat.StartTime {
		t.Fatalf("expected process start time %d but received %d", stat.StartTime, state.InitProcessStartTime)
	}
	if state.Config.Cgroups.Resources.Memory != 1024 {
		t.Fatalf("expected Memory to be 1024 but received %q", state.Config.Cgroups.Memory)
	}

	// Set initProcessStartTime so we fake to be running
	container.initProcessStartTime = state.InitProcessStartTime
	container.state = &runningState{c: container}
	newConfig := container.Config()
	newConfig.Cgroups.Resources.Memory = 2048
	if err := container.Set(newConfig); err != nil {
		t.Fatal(err)
	}
	state, err = container.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.Config.Cgroups.Resources.Memory != 2048 {
		t.Fatalf("expected Memory to be 2048 but received %q", state.Config.Cgroups.Memory)
	}
}
