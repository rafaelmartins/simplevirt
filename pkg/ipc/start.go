package ipc

import (
	"fmt"

	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

func (h *Handler) StartVM(args []string, res *int) error {
	*res = 0

	if len(args) != 1 {
		return fmt.Errorf("StartVM: requires 1 argument")
	}

	if err := qemu.Start(h.configDir, h.runtimeDir, args[0]); err != nil {
		*res = 1
		return err
	}

	return nil
}

func (c *ClientHandler) StartVM(name string) (int, error) {
	var response int
	if err := c.Client.Call(ServiceName+".StartVM", []string{name}, &response); err != nil {
		return 2, err
	}
	return response, nil
}