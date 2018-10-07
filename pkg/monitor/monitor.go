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
	instancesMutex *sync.Mutex
	exit           bool
	exitChan       chan bool
}

func NewMonitor(configDir string, runtimeDir string) *Monitor {
	mon := Monitor{ConfigDir: configDir, RuntimeDir: runtimeDir}
	mon.instances = make(map[string]*Instance)
	mon.instancesMutex = &sync.Mutex{}
	mon.exit = false
	mon.exitChan = make(chan bool)

	go func() {
		for {
			for name, _ := range mon.instances {
				instance := mon.Get(name)
				if instance == nil {
					continue
				}

				if running := instance.ProcessRunning(); running {
					continue
				}

				if instance.retries > instance.Config.MaximumRetries {
					logutils.Notice.Printf("monitor: %s: maximum number of retries exceeded (%d)", name,
						instance.Config.MaximumRetries)
					mon.cleanup(instance)
					continue
				} else if instance.retries > 0 {
					logutils.Notice.Printf("monitor: %s: retrying to start (%d)", name, instance.retries)
				}

				if err := qemu.Run(instance.Config); err != nil {
					logutils.LogError(err)
					instance.retries++
				} else {
					logutils.Warning.Printf("monitor: %s: started", name)
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
			logutils.LogError(mon.Start(vmName))
		}
	}

	return &mon
}

func (m *Monitor) Cleanup() {
	logutils.Notice.Printf("monitor: requesting cleanup")

	m.exit = true
	_ = <-m.exitChan

	for _, instance := range m.instances {
		m.cleanup(instance)
	}

	logutils.Warning.Printf("monitor: cleaned up")
}

func (m *Monitor) cleanup(instance *Instance) {
	if instance == nil {
		return
	}

	m.instancesMutex.Lock()
	defer m.instancesMutex.Unlock()

	instance.Cleanup()
	delete(m.instances, instance.Name)
}

func (m *Monitor) Get(name string) *Instance {
	m.instancesMutex.Lock()
	defer m.instancesMutex.Unlock()

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

func (m *Monitor) Start(name string) error {
	logutils.Notice.Printf("monitor: %s: requesting start", name)

	instance := m.Get(name)
	if instance != nil {
		if running := instance.Running(); running {
			return logutils.LogWarning(fmt.Errorf("monitor: %s: already running", name))
		}
	} else {
		var err error
		instance, err = newInstance(m, name)
		if err != nil {
			return logutils.LogError(err)
		}

		m.instancesMutex.Lock()
		defer m.instancesMutex.Unlock()
		m.instances[name] = instance
	}

	logutils.Notice.Printf("monitor: %s: start request completed", name)

	// the monitor will start the virtual machine
	return nil
}

func (m *Monitor) Shutdown(name string) error {
	logutils.Notice.Printf("monitor: %s: requesting shutdown", name)

	instance := m.Get(name)
	if instance == nil {
		return logutils.LogWarning(fmt.Errorf("monitor: %s: not running", name))
	}

	if running := instance.Running(); !running {
		return logutils.LogWarning(fmt.Errorf("monitor: %s: not running", name))
	}

	go m.cleanup(instance)

	logutils.Notice.Printf("monitor: %s: shutdown request completed", name)

	return nil
}

func (m *Monitor) Restart(name string) error {
	logutils.Notice.Printf("monitor: %s: requesting restart", name)

	if err := m.Shutdown(name); err != nil {
		return err
	}

	time.Sleep(time.Second)
	go m.Start(name)

	logutils.Notice.Printf("monitor: %s: restart request completed", name)

	return nil
}

func (m *Monitor) Reset(name string) error {
	logutils.Notice.Printf("monitor: %s: requesting reset", name)

	instance := m.Get(name)
	if instance == nil {
		return logutils.LogWarning(fmt.Errorf("monitor: %s: not running", name))
	}

	if running := instance.Running(); !running {
		return logutils.LogWarning(fmt.Errorf("monitor: %s: not running", name))
	}

	qmp, err := instance.QMP()
	if err != nil {
		return logutils.LogError(err)
	}

	if err := qmp.Reset(); err != nil {
		return logutils.LogError(err)
	}

	logutils.Warning.Printf("monitor: %s: reset", name)

	return nil
}
