package ipc

import (
	"fmt"

	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

func (h *Handler) GetVMStatus(args []string, res *string) error {
	if len(args) != 1 {
		return fmt.Errorf("GetVMStatus: requires 1 argument")
	}
	vms, err := qemu.List(h.configDir)
	if err != nil {
		return err
	}

	for _, vm := range vms {
		if args[0] == vm {
			*res = qemu.GetStatus(args[0])
			return nil
		}
	}

	return fmt.Errorf("virtual machine not found: %s", args[0])
}

func (c *ClientHandler) GetVMStatus(name string) (string, error) {
	var response string
	if err := c.Client.Call(ServiceName+".GetVMStatus", []string{name}, &response); err != nil {
		return "", err
	}
	return response, nil
}
