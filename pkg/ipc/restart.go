package ipc

import (
	"fmt"
	"time"

	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

func (h *Handler) RestartVM(args []string, res *int) error {
	*res = 0

	if len(args) != 1 {
		return fmt.Errorf("StartVM: requires 1 argument")
	}

	if err := qemu.Shutdown(args[0]); err != nil {
		*res = 1
		return err
	}

	time.Sleep(time.Second)

	if err := qemu.Start(h.configDir, h.runtimeDir, args[0]); err != nil {
		*res = 1
		return err
	}

	return nil
}

func (c *ClientHandler) RestartVM(name string) (int, error) {
	var response int
	if err := c.Client.Call(ServiceName+".RestartVM", []string{name}, &response); err != nil {
		return 2, err
	}
	return response, nil
}
