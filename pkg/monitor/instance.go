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
	monitor  *Monitor
	Config   *qemu.VirtualMachine `json:"config"`
	Name     string               `json:"name"`
	NICs     []*NIC               `json:"nics"`
	pid      int
	retries  int
	op       Operation
	opMutex  *sync.RWMutex
	opResult chan error
}

func newInstance(monitor *Monitor, name string, result chan error) (*Instance, error) {
	// NOTE: creating an instance WON'T start qemu. the monitor will start it
	//       as soon as it notices a non-running instance in the registry.

	if monitor == nil {
		return nil, fmt.Errorf("monitor: %s: invalid monitor", name)
	}

	inst := Instance{
		monitor:  monitor,
		Name:     name,
		pid:      -1,
		retries:  0,
		op:       Start,
		opMutex:  &sync.RWMutex{},
		opResult: result,
	}

	if err := inst.readConfig(); err != nil {
		return nil, err
	}

	return &inst, nil
}

func (i *Instance) readConfig() error {
	var err error
	if i.Config, err = qemu.ParseConfig(i.monitor.ConfigDir, i.Name); err != nil {
		return err
	}

	i.NICs, err = newNICs(i.Name, i.Config)
	if i.NICs == nil {
		return err
	}

	i.Config.SetName(i.Name)
	i.Config.SetQMP(i.QMPSocket())
	i.Config.SetPIDFile(i.PIDFile())

	return nil
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
	// if pid is set
	if i.pid > 0 {
		// check if process still alive
		if err := syscall.Kill(i.pid, syscall.Signal(0)); err == nil {
			return true
		}
	}

	// fallback to re-reading PID file
	pid, err := i.PID()
	if err != nil {
		return false
	}
	i.pid = pid

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

func (i *Instance) Start() error {
	if running := i.ProcessRunning(); running {
		return nil
	}

	i.opMutex.RLock()

	if i.retries > i.Config.MaximumRetries {
		i.opMutex.RUnlock()
		if err := i.Shutdown(); err != nil {
			return err
		}
		return logutils.LogError(
			fmt.Errorf("monitor: %s: maximum number of retries exceeded (%d)",
				i.Name, i.Config.MaximumRetries))
	}

	defer i.opMutex.RUnlock()

	logutils.Warning.Printf("monitor: %s: start", i.Name)

	if i.retries > 0 {
		logutils.Warning.Printf("monitor: %s: start: retry %d", i.Name, i.retries)
	}

	if err := qemu.Run(i.Config); err != nil {
		i.retries++
		logutils.Warning.Printf("monitor: %s: start: failed", i.Name)
		return logutils.LogError(err)
	} else {
		logutils.Warning.Printf("monitor: %s: start: done", i.Name)
	}

	return nil
}

func (i *Instance) Reset() error {
	i.opMutex.RLock()
	defer i.opMutex.RUnlock()

	if running := i.Running(); !running {
		return nil
	}

	logutils.Warning.Printf("monitor: %s: reset", i.Name)

	qmp, err := i.QMP()
	if err != nil {
		return logutils.LogError(err)
	}

	if err := qmp.Reset(); err != nil {
		return logutils.LogError(err)
	}

	i.op = Start

	logutils.Warning.Printf("monitor: %s: reset: done", i.Name)

	return nil
}

func (i *Instance) shutdown() error {
	qmp, err := i.QMP()
	if err == nil {
		logutils.Notice.Printf("monitor: %s: sending powerdown command (%ds timeout)", i.Name,
			i.Config.ShutdownTimeout)
		if err := qmp.Powerdown(); err != nil {
			logutils.LogError(err)
		}

		for j := 0; j < i.Config.ShutdownTimeout; j++ {
			if ok := i.ProcessRunning(); !ok {
				break
			}
			time.Sleep(time.Second)
		}
	}

	if ok := i.ProcessRunning(); ok {
		logutils.Notice.Printf("monitor: %s: sending SIGKILL", i.Name)

		// if process is running, our cached PID is valid
		syscall.Kill(i.pid, syscall.SIGKILL)
	}

	logutils.Notice.Printf("monitor: %s: waiting for process to exit", i.Name)
	for i.ProcessRunning() {
		time.Sleep(time.Second)
	}

	return CleanupNICs(i.Name, i.NICs)
}

func (i *Instance) Shutdown() error {
	i.monitor.instancesMutex.Lock()
	defer i.monitor.instancesMutex.Unlock()

	i.opMutex.RLock()
	defer i.opMutex.RUnlock()

	logutils.Warning.Printf("monitor: %s: shutdown", i.Name)

	if err := i.shutdown(); err != nil {
		return err
	}

	delete(i.monitor.instances, i.Name)

	logutils.Warning.Printf("monitor: %s: shutdown: done", i.Name)

	return nil
}

func (i *Instance) Restart() error {
	i.opMutex.RLock()
	defer i.opMutex.RUnlock()

	logutils.Warning.Printf("monitor: %s: restart (shutdown + start)", i.Name)
	logutils.Warning.Printf("monitor: %s: shutdown", i.Name)

	if err := i.shutdown(); err != nil {
		return err
	}

	if err := i.readConfig(); err != nil {
		i.monitor.instancesMutex.Lock()
		defer i.monitor.instancesMutex.Unlock()

		delete(i.monitor.instances, i.Name)

		return logutils.LogError(err)
	}

	i.op = Start

	logutils.Warning.Printf("monitor: %s: shutdown: done", i.Name)

	return nil
}
