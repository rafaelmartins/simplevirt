package monitor

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
	"github.com/rafaelmartins/simplevirt/pkg/qemu"
	"github.com/rafaelmartins/simplevirt/pkg/qmp"
)

type Operation int

const (
	Start Operation = iota
	Shutdown
	Restart
	Reset
)

type Instance struct {
	monitor *Monitor
	Config  *qemu.VirtualMachine `json:"config"`
	Name    string               `json:"name"`
	NICs    []*NIC               `json:"nics"`
	retries int
	op      Operation
	opMutex *sync.Mutex
}

func newInstance(monitor *Monitor, name string) (*Instance, error) {
	// NOTE: creating an instance WON'T start qemu. the monitor will start it
	//       as soon as it notices a non-running instance in the registry.

	config, err := qemu.ParseConfig(monitor.ConfigDir, name)
	if err != nil {
		return nil, err
	}

	nics := newNICs(name, config)
	if nics == nil {
		return nil, fmt.Errorf("monitor: %s: failed to create network interfaces", name)
	}

	inst := Instance{
		monitor: monitor,
		Config:  config,
		Name:    name,
		NICs:    nics,
		retries: 0,
		op:      Start,
		opMutex: &sync.Mutex{},
	}

	inst.Config.SetName(name)
	inst.Config.SetQMP(inst.QMPSocket())
	inst.Config.SetPIDFile(inst.PIDFile())

	return &inst, nil
}

func (i *Instance) PIDFile() string {
	if i.Name == "" {
		return ""
	}

	return filepath.Join(i.monitor.RuntimeDir, fmt.Sprintf("%s.pid", i.Name))
}

func (i *Instance) QMPSocket() string {
	if i.Name == "" {
		return ""
	}

	return filepath.Join(i.monitor.RuntimeDir, fmt.Sprintf("%s.sock", i.Name))
}

func (i *Instance) PID() (int, error) {
	file := i.PIDFile()
	if file == "" {
		return -1, fmt.Errorf("monitor: can't guess PID file path")
	}

	content, err := ioutil.ReadFile(file)
	if err != nil {
		return -1, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return -1, err
	}

	return pid, nil
}

func (i *Instance) QMP() (*qmp.QMP, error) {
	socket := i.QMPSocket()
	if socket == "" {
		return nil, fmt.Errorf("monitor: can't guess QMP socket path")
	}

	return &qmp.QMP{Socket: socket}, nil
}

func (i *Instance) ProcessRunning() bool {
	pid, err := i.PID()
	if err != nil {
		return false
	}

	return syscall.Kill(pid, syscall.Signal(0)) == nil
}

func (i *Instance) QMPStatus() (string, error) {
	qmp, err := i.QMP()
	if err != nil {
		return "exited", logutils.LogError(err)
	}

	status, err := qmp.QueryStatus()
	if err != nil {
		return "exited", logutils.LogError(err)
	}

	return status.Status, nil
}

func (i *Instance) QMPRunning() (bool, error) {
	qmp, err := i.QMP()
	if err != nil {
		return false, logutils.LogError(err)
	}

	status, err := qmp.QueryStatus()
	if err != nil {
		return false, logutils.LogError(err)
	}

	return status.Running, nil
}

func (i *Instance) Status() string {
	status, _ := i.QMPStatus()
	return status
}

func (i *Instance) Running() bool {
	if ok := i.ProcessRunning(); !ok {
		return false
	}

	if ok, err := i.QMPRunning(); err != nil || !ok {
		return false
	}

	return true
}

func (i *Instance) Start() {
	if running := i.ProcessRunning(); running {
		return
	}

	i.opMutex.Lock()

	if i.retries > i.Config.MaximumRetries {
		logutils.Notice.Printf("monitor: %s: maximum number of retries exceeded (%d)", i.Name,
			i.Config.MaximumRetries)
		i.opMutex.Unlock()
		i.monitor.cleanup(i)
		return
	}

	defer i.opMutex.Unlock()

	if i.retries > 0 {
		logutils.Notice.Printf("monitor: %s: retrying to start (%d)", i.Name, i.retries)
	}

	if err := qemu.Run(i.Config); err != nil {
		logutils.LogError(err)
		i.retries++
	} else {
		logutils.Warning.Printf("monitor: %s: started", i.Name)
	}
}

func (i *Instance) Reset() {
	i.opMutex.Lock()
	defer i.opMutex.Unlock()

	if running := i.Running(); !running {
		return
	}

	qmp, err := i.QMP()
	if err != nil {
		logutils.LogError(err)
	}

	if err := qmp.Reset(); err != nil {
		logutils.LogError(err)
	}

	i.op = Start
}

func (i *Instance) Cleanup(withNICs bool) {
	i.opMutex.Lock()
	defer i.opMutex.Unlock()

	pid, err := i.PID()
	if err != nil {
		logutils.LogError(err)
	}

	qmp, err := i.QMP()
	if err == nil {
		logutils.Warning.Printf("monitor: %s: sending powerdown command (%ds timeout)", i.Name,
			i.Config.ShutdownTimeout)
		qmp.Powerdown()

		for j := 0; j < i.Config.ShutdownTimeout; j++ {
			if ok := i.ProcessRunning(); !ok {
				break
			}
			time.Sleep(time.Second)
		}
	}

	if ok := i.ProcessRunning(); ok {
		logutils.Warning.Printf("monitor: %s: sending SIGKILL", i.Name)
		syscall.Kill(pid, syscall.SIGKILL)
	}

	logutils.Notice.Printf("monitor: %s: waiting for process to exit", i.Name)
	for i.ProcessRunning() {
		time.Sleep(time.Second)
	}

	if withNICs {
		CleanupNICs(i.Name, i.NICs)
	}

	logutils.Warning.Printf("monitor: %s: shut down", i.Name)
}
