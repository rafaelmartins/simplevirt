package qemu

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rafaelmartins/simplevirt/pkg/netdev"
)

var (
	registry      = make(map[string]instance)
	registryMutex = &sync.Mutex{}
)

type device struct {
	bridge string
	iface  *net.Interface
}

type instance struct {
	vm      *virtualmachine
	proc    *os.Process
	devices []*device
}

func cleanupDevices(devices []*device) error {
	errs := []string{}
	for _, device := range devices {
		if err := netdev.RemoveDevFromBridge(device.bridge, device.iface); err != nil {
			errs = append(errs, fmt.Sprintf("%s - %s: %s", device.iface.Name, device.bridge, err.Error()))
		}
		if err := netdev.DestroyQtap(device.iface); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", device.iface.Name, err.Error()))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func cleanupInstance(name string, inst instance) error {
	for i := 0; i < inst.vm.ShutdownTimeout; i++ {
		if ok := IsRunning(name); !ok {
			break
		}
		time.Sleep(time.Second)
	}

	if ok := IsRunning(name); ok {
		inst.proc.Kill()
	}

	for IsRunning(name) {
		time.Sleep(time.Second)
	}

	if err := cleanupDevices(inst.devices); err != nil {
		return err
	}

	registryMutex.Lock()
	delete(registry, name)
	registryMutex.Unlock()

	return nil
}

func GetStatus(name string) string {
	inst, ok := registry[name]
	if !ok {
		return "stopped"
	}

	if inst.proc == nil {
		return "exited"
	}

	if err := inst.proc.Signal(syscall.Signal(0)); err != nil {
		return "exited"
	}

	return "running"
}

func IsRunning(name string) bool {
	status := GetStatus(name)
	return status == "running"
}

func Start(configDir string, runtimeDir string, name string) error {
	if ok := IsRunning(name); ok {
		return fmt.Errorf("qemu: virtual machine is already running: %s", name)
	}

	vm, err := parseConfig(configDir, name)
	if err != nil {
		return err
	}

	inst := instance{
		vm: vm,
	}

	inst.vm.name = name
	inst.vm.monitor = filepath.Join(runtimeDir, fmt.Sprintf("%s.sock", name))
	inst.vm.pidfile = filepath.Join(runtimeDir, fmt.Sprintf("%s.pid", name))

	for i, nic := range inst.vm.NICs {
		if nic.Bridge == "" {
			continue
		}
		tap, err := netdev.CreateQtap(inst.vm.RunAs)
		if err != nil {
			return err
		}
		if err := netdev.AddDevToBridge(nic.Bridge, tap); err != nil {
			netdev.DestroyQtap(tap)
			return err
		}

		inst.devices = append(inst.devices, &device{bridge: nic.Bridge, iface: tap})
		inst.vm.NICs[i].device = tap.Name
	}

	args, err := buildCmdVirtualMachine(vm)
	if err != nil {
		return err
	}

	cmd := exec.Command(fmt.Sprintf("qemu-system-%s", inst.vm.SystemTarget), args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanupDevices(inst.devices)
		return fmt.Errorf("qemu: failed to start virtual machine: %s\n\n%s", err, string(out))
	}

	pidS, err := ioutil.ReadFile(inst.vm.pidfile)
	if err != nil {
		cleanupDevices(inst.devices)
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidS)))
	if err != nil {
		cleanupDevices(inst.devices)
		return err
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		cleanupDevices(inst.devices)
		return err
	}

	inst.proc = proc

	registryMutex.Lock()
	registry[name] = inst
	registryMutex.Unlock()

	return nil
}

func Shutdown(name string) error {
	if ok := IsRunning(name); !ok {
		return fmt.Errorf("qemu: virtual machine is not running: %s", name)
	}

	inst, ok := registry[name]
	if !ok {
		return fmt.Errorf("qemu: virtual machine is not running: %s", name)
	}

	if err := cleanupInstance(name, inst); err != nil {
		return err
	}

	return nil
}

func List(configDir string) ([]string, error) {
	return listConfigs(configDir)
}

func Cleanup() {
	for name, inst := range registry {
		cleanupInstance(name, inst)
	}
}
