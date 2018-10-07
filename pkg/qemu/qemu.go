package qemu

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"

	"gopkg.in/yaml.v2"
)

func ParseConfig(configDir string, name string) (*VirtualMachine, error) {
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

	config := VirtualMachine{
		SystemTarget:    "x86_64",
		EnableKVM:       true,
		ShutdownTimeout: 60,
		MaximumRetries:  5,
		RunAs:           "nobody",
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func ListConfigs(configDir string) ([]string, error) {
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

func Run(config *VirtualMachine) error {
	args, err := buildCmdVirtualMachine(config)
	if err != nil {
		return err
	}

	bin := fmt.Sprintf("qemu-system-%s", config.SystemTarget)

	logutils.Notice.Printf("qemu: %s: calling %q with arguments: %q", config.name, bin, args)

	cmd := exec.Command(bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu: %s: %q failed to start: %s\n\n%s", config.name, bin, err, string(out))
	}

	return nil
}
