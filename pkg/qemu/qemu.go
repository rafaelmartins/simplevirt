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

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
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

func cleanupDevices(name string, devices []*device) error {
	errs := []string{}
	for _, device := range devices {
		logutils.Notice.Printf("qemu: %s:   cleaning up network device: %s (bridge: %s)", name, device.iface.Name, device.bridge)
		if err := netdev.RemoveDevFromBridge(device.bridge, device.iface); err != nil {
			errs = append(errs, fmt.Sprintf("qemu: %s: %s - %s: %s", name, device.iface.Name, device.bridge, err.Error()))
		}
		if err := netdev.DestroyQtap(device.iface); err != nil {
			errs = append(errs, fmt.Sprintf("qemu: %s: %s: %s", name, device.iface.Name, err.Error()))
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

	if err := cleanupDevices(name, inst.devices); err != nil {
		return err
	}

	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(registry, name)

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
	logutils.Notice.Printf("qemu: %s: starting", name)

	if ok := IsRunning(name); ok {
		return logutils.LogError(fmt.Errorf("qemu: %s: already running", name))
	}

	vm, err := parseConfig(configDir, name)
	if err != nil {
		return logutils.LogError(err)
	}

	inst := instance{
		vm: vm,
	}

	inst.vm.name = name
	inst.vm.monitor = filepath.Join(runtimeDir, fmt.Sprintf("%s.sock", name))
	inst.vm.pidfile = filepath.Join(runtimeDir, fmt.Sprintf("%s.pid", name))

	for i, nic := range inst.vm.NICs {
		logutils.Notice.Printf("qemu: %s:   creating network device (bridge: %s)", name, nic.Bridge)

		if nic.Bridge == "" {
			continue
		}

		tap, err := netdev.CreateQtap(inst.vm.RunAs)
		if err != nil {
			return logutils.LogError(err)
		}
		logutils.Notice.Printf("qemu: %s:     device: %s", name, tap.Name)

		if err := netdev.AddDevToBridge(nic.Bridge, tap); err != nil {
			netdev.DestroyQtap(tap)
			return logutils.LogError(err)
		}

		inst.devices = append(inst.devices, &device{bridge: nic.Bridge, iface: tap})
		inst.vm.NICs[i].device = tap.Name
	}

	args, err := buildCmdVirtualMachine(vm)
	if err != nil {
		return logutils.LogError(err)
	}

	cmd := exec.Command(fmt.Sprintf("qemu-system-%s", inst.vm.SystemTarget), args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanupDevices(name, inst.devices)
		return logutils.LogError(fmt.Errorf("qemu: %s: failed to start: %s\n\n%s", name, err, string(out)))
	}

	pidS, err := ioutil.ReadFile(inst.vm.pidfile)
	if err != nil {
		cleanupDevices(name, inst.devices)
		return logutils.LogError(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidS)))
	if err != nil {
		cleanupDevices(name, inst.devices)
		return logutils.LogError(err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		cleanupDevices(name, inst.devices)
		return logutils.LogError(err)
	}

	inst.proc = proc

	registryMutex.Lock()
	defer registryMutex.Unlock()
	registry[name] = inst

	return nil
}

func AutoStart(configDir string, runtimeDir string) error {
	logutils.Notice.Printf("qemu: starting virtual machines automatically")

	vms, err := listConfigs(configDir)
	if err != nil {
		return logutils.LogError(err)
	}

	for _, vmName := range vms {
		vm, err := parseConfig(configDir, vmName)
		if err != nil {
			return logutils.LogError(err)
		}

		if vm.AutoStart {
			Start(configDir, runtimeDir, vmName)
		}
	}

	return nil
}

func Shutdown(name string) error {
	logutils.Notice.Printf("qemu: %s: shutting down", name)

	if ok := IsRunning(name); !ok {
		return logutils.LogError(fmt.Errorf("qemu: %s: not running", name))
	}

	inst, ok := registry[name]
	if !ok {
		return logutils.LogError(fmt.Errorf("qemu: %s: not running", name))
	}

	logutils.Notice.Printf("qemu: %s:   with %ds timeout", name, inst.vm.ShutdownTimeout)
	if err := cleanupInstance(name, inst); err != nil {
		return logutils.LogError(err)
	}

	return nil
}

func List(configDir string) ([]string, error) {
	conf, err := listConfigs(configDir)
	return conf, logutils.LogError(err)
}

func Cleanup() {
	for name, _ := range registry {
		Shutdown(name)
	}
}
