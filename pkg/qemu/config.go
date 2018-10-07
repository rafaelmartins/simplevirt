package qemu

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	driveInterfaceChoices = []string{"ide", "scsi", "sd", "mtd", "floppy", "pflash", "virtio", "none"}
	driveMediaChoices     = []string{"disk", "cdrom"}
	driveCacheChoices     = []string{"none", "writeback", "unsafe", "directsync", "writethrough"}
	driveFormatChoices    = []string{"raw"}

	reRAM    = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?[MG]?$`)
	reConfig = regexp.MustCompile(`^([^\.].*)\.ya?ml$`)
)

type Drive struct {
	File      string `yaml:"file" json:"file"`
	Interface string `yaml:"interface" json:"interface"`
	Media     string `yaml:"media" json:"media"`
	Snapshot  bool   `yaml:"snapshot" json:"snapshot"`
	Cache     string `yaml:"cache" json:"cache"`
	Format    string `yaml:"format" json:"format"`
}

type NIC struct {
	Bridge      string            `yaml:"bridge" json:"bridge"`
	MACAddr     string            `yaml:"mac_address" json:"mac_address"`
	Model       string            `yaml:"model" json:"model"`
	NetUserArgs map[string]string `yaml:"net_user_args" json:"net_user_args"`
	device      string
}

type VirtualMachine struct {
	name    string
	qmp     string
	pidfile string

	AutoStart bool `yaml:"auto_start" json:"auto_start"`

	SystemTarget string `yaml:"system_target" json:"system_target"`
	MachineType  string `yaml:"machine_type" json:"machine_type"`
	RunAs        string `yaml:"run_as" json:"run_as"`
	EnableKVM    bool   `yaml:"enable_kvm" json:"enable_kvm"`

	Boot   map[string]string `yaml:"boot" json:"boot"`
	Drives []*Drive          `yaml:"drives" json:"drives"`
	NICs   []*NIC            `yaml:"nics" json:"nics"`

	CPUModel   string `yaml:"cpu_model" json:"cpu_model"`
	CPUs       int    `yaml:"cpus" json:"cpus"`
	RAM        string `yaml:"ram" json:"ram"`
	VNCDisplay string `yaml:"vnc_display" json:"vnc_display"`

	AdditionalArgs []string `yaml:"additional_args" json:"additional_args"`

	ShutdownTimeout int `yaml:"shutdown_timeout" json:"shutdown_timeout"`
	MaximumRetries  int `yaml:"maximum_retries" json:"maximum_retries"`
}

func (n *NIC) SetDevice(device string) {
	n.device = device
}

func (vm *VirtualMachine) SetName(name string) {
	vm.name = name
}

func (vm *VirtualMachine) SetQMP(qmp string) {
	vm.qmp = qmp
}

func (vm *VirtualMachine) SetPIDFile(pidfile string) {
	vm.pidfile = pidfile
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

func buildCmdDrive(idx int, drv *Drive) ([]string, error) {
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

func buildCmdDrives(drvs []*Drive) ([]string, error) {
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

func buildCmdNIC(idx int, nc *NIC) ([]string, error) {
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

func buildCmdNICs(nics []*NIC) ([]string, error) {
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

func buildCmdVirtualMachine(vm *VirtualMachine) ([]string, error) {
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
