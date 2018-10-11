package monitor

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

type Monitor struct {
	ConfigDir  string
	RuntimeDir string

	instances      map[string]*Instance
	instancesMutex *sync.RWMutex
	exit           bool
	exitChan       chan bool
}

func NewMonitor(configDir string, runtimeDir string) *Monitor {
	mon := Monitor{
		ConfigDir:      configDir,
		RuntimeDir:     runtimeDir,
		instances:      make(map[string]*Instance),
		instancesMutex: &sync.RWMutex{},
		exit:           false,
		exitChan:       make(chan bool),
	}

	go func() {
		for {
			for name, _ := range mon.instances {
				mon.instancesMutex.RLock()
				instance := mon.Get(name)
				mon.instancesMutex.RUnlock()
				if instance == nil {
					continue
				}

				var err error

				switch instance.op {
				case Start:
					err = instance.Start()

				case Shutdown:
					err = instance.Shutdown()

				case Reset:
					err = instance.Reset()
				}

				if instance.opResult != nil {
					select {
					case instance.opResult <- err:
					}
					instance.opResult = nil
				}
			}

			if mon.exit {
				mon.exitChan <- true
				break
			}

			time.Sleep(time.Second)
		}
	}()

	vms, err := qemu.ListConfigs(configDir)
	if err != nil {
		logutils.LogError(err)
		mon.Cleanup()
	}

	for _, vmName := range vms {
		vm, err := qemu.ParseConfig(configDir, vmName)
		if err != nil {
			logutils.LogError(err)
			continue
		}

		if vm.AutoStart {
			logutils.LogError(mon.Start(vmName, nil))
		}
	}

	return &mon
}

func (m *Monitor) Cleanup() {
	logutils.Notice.Printf("monitor: cleanup")

	m.exit = true
	_ = <-m.exitChan

	for _, instance := range m.instances {
		// force cleanup
		instance.op = Start
		instance.Shutdown()
	}
}

func (m *Monitor) Get(name string) *Instance {
	if instance, ok := m.instances[name]; ok {
		return instance
	}

	return nil
}

func (m *Monitor) Status(name string) string {
	instance := m.Get(name)
	if instance == nil {
		return "stopped"
	}

	return instance.Status()
}

func (m *Monitor) Running(name string) bool {
	instance := m.Get(name)
	if instance == nil {
		return false
	}

	return instance.Running()
}

func (m *Monitor) List() ([]string, error) {
	conf, err := qemu.ListConfigs(m.ConfigDir)
	if err != nil {
		return nil, logutils.LogError(err)
	}

	rv := append([]string{}, conf...)

	for name, _ := range m.instances {
		found := false
		for _, confName := range conf {
			if confName == name {
				found = true
				break
			}
		}
		if !found {
			rv = append(rv, name)
		}
	}

	sort.Strings(rv)

	return rv, nil
}

func (m *Monitor) Start(name string, result chan error) error {
	logutils.Notice.Printf("monitor: requesting start: %s", name)

	m.instancesMutex.RLock()
	instance := m.Get(name)
	m.instancesMutex.RUnlock()

	if instance != nil {
		if running := instance.Running(); running {
			return logutils.LogWarning(fmt.Errorf("monitor: %s: already running", name))
		}
	} else {
		m.instancesMutex.Lock()
		defer m.instancesMutex.Unlock()

		var err error
		instance, err = newInstance(m, name, result)
		if err != nil {
			return logutils.LogError(err)
		}

		m.instances[name] = instance
	}

	return nil
}

func (m *Monitor) Shutdown(name string, result chan error) error {
	logutils.Notice.Printf("monitor: requesting shutdown: %s", name)

	m.instancesMutex.RLock()
	defer m.instancesMutex.RUnlock()

	instance := m.Get(name)
	if instance == nil {
		return logutils.LogWarning(fmt.Errorf("monitor: %q not running", name))
	}

	instance.opMutex.Lock()
	defer instance.opMutex.Unlock()
	instance.op = Shutdown
	instance.opResult = result

	return nil
}

func (m *Monitor) Reset(name string, result chan error) error {
	logutils.Notice.Printf("monitor: requesting reset: %s", name)

	m.instancesMutex.RLock()
	defer m.instancesMutex.RUnlock()

	instance := m.Get(name)
	if instance == nil {
		return logutils.LogWarning(fmt.Errorf("monitor: %q not running", name))
	}

	instance.opMutex.Lock()
	defer instance.opMutex.Unlock()
	instance.op = Reset
	instance.opResult = result

	return nil
}
