package qemu

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

var (
	driveInterfaceChoices = []string{"ide", "scsi", "sd", "mtd", "floppy", "pflash", "virtio", "none"}
	driveMediaChoices     = []string{"disk", "cdrom"}
	driveCacheChoices     = []string{"none", "writeback", "unsafe", "directsync", "writethrough"}
	driveFormatChoices    = []string{"raw"}

	reRAM    = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?[MG]?$`)
	reConfig = regexp.MustCompile(`^([^\.].*)\.ya?ml$`)
)

type drive struct {
	File      string `yaml:"file"`
	Interface string `yaml:"interface"`
	Media     string `yaml:"media"`
	Snapshot  bool   `yaml:"snapshot"`
	Cache     string `yaml:"cache"`
	Format    string `yaml:"format"`
}

type nic struct {
	Bridge      string            `yaml:"bridge"`
	MACAddr     string            `yaml:"mac_address"`
	Model       string            `yaml:"model"`
	NetUserArgs map[string]string `yaml:"net_user_args"`
	device      string
}

type virtualmachine struct {
	name    string
	qmp     string
	pidfile string

	AutoStart bool `yaml:"auto_start"`

	SystemTarget string `yaml:"system_target"`
	MachineType  string `yaml:"machine_type"`
	RunAs        string `yaml:"run_as"`
	EnableKVM    bool   `yaml:"enable_kvm"`

	Boot   map[string]string `yaml:"boot"`
	Drives []*drive          `yaml:"drives"`
	NICs   []*nic            `yaml:"nics"`

	CPUModel   string `yaml:"cpu_model"`
	CPUs       int    `yaml:"cpus"`
	RAM        string `yaml:"ram"`
	VNCDisplay string `yaml:"vnc_display"`

	AdditionalArgs []string `yaml:"additional_args"`

	ShutdownTimeout int `yaml:"shutdown_timeout"`
}

func appendParam(name string, param string, deft string, choices []string, error_name string) (string, error) {
	if param == "" {
		if deft == "" {
			return "", nil
		}
		return fmt.Sprintf(",%s=%s", name, deft), nil
	}
	if choices != nil {
		found := false
		for _, choice := range choices {
			if param == choice {
				found = true
			}
		}
		if !found {
			strChoices := strings.Join(choices, "', '")
			return "", fmt.Errorf("qemu: %s: invalid value (%s). valid choices are: '%s'",
				error_name, param, strChoices)
		}
	}
	return fmt.Sprintf(",%s=%s", name, param), nil
}

func buildCmdDrive(idx int, drv *drive) ([]string, error) {
	if drv.File == "" {
		return nil, fmt.Errorf("qemu: drive[%d].file: parameter is required", idx)
	}
	if !filepath.IsAbs(drv.File) {
		return nil, fmt.Errorf("qemu: drive[%d].file: path must be absolute", idx)
	}

	arg := fmt.Sprintf("file=%s", strings.Replace(drv.File, ",", ",,", -1))

	v, err := appendParam("if", drv.Interface, "virtio", driveInterfaceChoices, fmt.Sprintf("drive[%d].interface", idx))
	if err != nil {
		return nil, err
	}
	arg += v

	v, err = appendParam("media", drv.Media, "disk", driveMediaChoices, fmt.Sprintf("drive[%d].media", idx))
	if err != nil {
		return nil, err
	}
	arg += v

	v, err = appendParam("cache", drv.Cache, "none", driveCacheChoices, fmt.Sprintf("drive[%d].cache", idx))
	if err != nil {
		return nil, err
	}
	arg += v

	v, err = appendParam("format", drv.Format, "", driveFormatChoices, fmt.Sprintf("drive[%d].format", idx))
	if err != nil {
		return nil, err
	}
	arg += v

	if drv.Snapshot {
		arg += ",snapshot=on"
	}

	return []string{"-drive", arg}, nil
}

func buildCmdDrives(drvs []*drive) ([]string, error) {
	if len(drvs) == 0 {
		return nil, fmt.Errorf("qemu: drive: at least one drive must be defined")
	}
	rv := []string{}
	for i, drv := range drvs {
		v, err := buildCmdDrive(i+1, drv)
		if err != nil {
			return nil, err
		}
		rv = append(rv, v...)
	}
	return rv, nil
}

func buildCmdNIC(idx int, nc *nic) ([]string, error) {
	if nc.MACAddr == "" {
		return nil, fmt.Errorf("qemu: nic[%d].mac_address: parameter is required", idx)
	}

	hdAddr, err := net.ParseMAC(nc.MACAddr)
	if err != nil {
		return nil, fmt.Errorf("qemu: nic[%d].mac_address: invalid value (%s)", idx, err)
	}

	arg := ""

	if nc.Bridge != "" {
		if nc.device == "" {
			return nil, fmt.Errorf("qemu: nic[%d]: missing device", idx)
		}
		arg += fmt.Sprintf("tap,ifname=%s,script=no", nc.device)
	} else {
		arg += fmt.Sprintf("user")
		for k, v := range nc.NetUserArgs {
			arg += fmt.Sprintf(",%s=%s", k, v)
		}
	}

	arg += fmt.Sprintf(",mac=%s", hdAddr.String())

	v, err := appendParam("model", nc.Model, "virtio", nil, fmt.Sprintf("nic[%d].model", idx))
	if err != nil {
		return nil, err
	}
	arg += v

	return []string{"-nic", arg}, nil
}

func buildCmdNICs(nics []*nic) ([]string, error) {
	if len(nics) == 0 {
		return nil, fmt.Errorf("qemu: nic: at least one NIC must be defined")
	}
	rv := []string{}
	for i, nc := range nics {
		v, err := buildCmdNIC(i+1, nc)
		if err != nil {
			return nil, err
		}
		rv = append(rv, v...)
	}
	return rv, nil
}

func buildCmdVirtualMachine(vm *virtualmachine) ([]string, error) {
	if vm == nil {
		return nil, fmt.Errorf("qemu: virtualmachine: not defined")
	}

	rv := []string{}

	if vm.name != "" {
		rv = append(rv, "-name", vm.name)
	}

	if vm.qmp != "" {
		rv = append(rv, "-qmp", fmt.Sprintf("unix:%s,server,nowait", vm.qmp))
	}

	if vm.pidfile != "" {
		rv = append(rv, "-daemonize", "-pidfile", vm.pidfile)
	}

	if vm.MachineType != "" {
		rv = append(rv, "-M", vm.MachineType)
	}

	if vm.EnableKVM {
		rv = append(rv, "-enable-kvm")
	}

	if vm.RunAs != "" {
		rv = append(rv, "-runas", vm.RunAs)
	}

	if vm.CPUModel != "" {
		rv = append(rv, "-cpu", vm.CPUModel)
	}

	if vm.CPUs > 0 {
		rv = append(rv, "-smp", fmt.Sprintf("cpus=%d", vm.CPUs))
	}

	if vm.RAM != "" {
		if !reRAM.MatchString(vm.RAM) {
			return nil, fmt.Errorf("qemu: virtualmachine: invalid RAM size (%s)", vm.RAM)
		}
		rv = append(rv, "-m", fmt.Sprintf("size=%s", vm.RAM))
	}

	bootArgs := []string{}
	for k, v := range vm.Boot {
		bootArgs = append(bootArgs, fmt.Sprintf("%s=%s", k, v))
	}
	if len(bootArgs) > 0 {
		rv = append(rv, "-boot", strings.Join(bootArgs, ","))
	}

	rv = append(rv, "-display")
	if vm.VNCDisplay != "" {
		rv = append(rv, fmt.Sprintf("vnc=%s", vm.VNCDisplay))
	} else {
		rv = append(rv, "none")
	}

	drives, err := buildCmdDrives(vm.Drives)
	if err != nil {
		return nil, err
	}
	rv = append(rv, drives...)

	nics, err := buildCmdNICs(vm.NICs)
	if err != nil {
		return nil, err
	}
	rv = append(rv, nics...)

	rv = append(rv, vm.AdditionalArgs...)

	return rv, nil
}

func parseConfig(configDir string, name string) (*virtualmachine, error) {
	var cfg string
	for _, value := range []string{name + ".yml", name + ".yaml"} {
		value := filepath.Join(configDir, value)
		if _, err := os.Stat(value); err == nil {
			cfg = value
			break
		}
	}

	if cfg == "" {
		return nil, fmt.Errorf("qemu: config: failed to find configuration file for virtual machine: %s", name)
	}

	data, err := ioutil.ReadFile(cfg)
	if err != nil {
		return nil, err
	}

	config := virtualmachine{
		SystemTarget:    "x86_64",
		EnableKVM:       true,
		ShutdownTimeout: 60,
		RunAs:           "nobody",
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func listConfigs(configDir string) ([]string, error) {
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		return nil, err
	}

	rv := []string{}
	for _, info := range files {
		if info.IsDir() {
			continue
		}

		m := reConfig.FindStringSubmatch(info.Name())
		if len(m) == 0 {
			continue
		}

		found := false
		for _, n := range rv {
			if n == m[1] {
				found = true
			}
		}
		if !found {
			rv = append(rv, m[1])
		}
	}

	return rv, nil
}
