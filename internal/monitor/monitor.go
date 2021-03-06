package monitor

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rafaelmartins/simplevirt/internal/logutils"
	"github.com/rafaelmartins/simplevirt/internal/qemu"
)

type Monitor struct {
	ConfigDir  string
	RuntimeDir string

	instances      map[string]*Instance
	instancesMutex *sync.RWMutex
	exit           bool
	exitChan       chan bool
}

func NewMonitor(configDir string, runtimeDir string) (*Monitor, error) {
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
				instance := mon.Get(name)
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
					default:
						if err != nil {
							logutils.LogError(err)
						}
					}
					instance.opResult = nil

					continue
				}

				if err != nil {
					logutils.LogError(err)
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
		mon.Cleanup()
		return nil, err
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

	return &mon, nil
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
	m.instancesMutex.RLock()
	defer m.instancesMutex.RUnlock()

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
		return nil, err
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

	instance := m.Get(name)
	if instance != nil {
		if running := instance.Running(); running {
			return fmt.Errorf("monitor: %s: already running", name)
		}
	} else {
		m.instancesMutex.Lock()
		defer m.instancesMutex.Unlock()

		var err error
		instance, err = newInstance(m, name, result)
		if err != nil {
			return err
		}

		m.instances[name] = instance
	}

	return nil
}

func (m *Monitor) Shutdown(name string, result chan error) error {
	logutils.Notice.Printf("monitor: requesting shutdown: %s", name)

	instance := m.Get(name)
	if instance == nil {
		return fmt.Errorf("monitor: %q not running", name)
	}

	instance.opMutex.Lock()
	defer instance.opMutex.Unlock()
	instance.op = Shutdown
	instance.opResult = result

	return nil
}

func (m *Monitor) Reset(name string, result chan error) error {
	logutils.Notice.Printf("monitor: requesting reset: %s", name)

	instance := m.Get(name)
	if instance == nil {
		return fmt.Errorf("monitor: %q not running", name)
	}

	instance.opMutex.Lock()
	defer instance.opMutex.Unlock()
	instance.op = Reset
	instance.opResult = result

	return nil
}
