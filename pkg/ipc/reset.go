package ipc

import (
	"fmt"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
)

func (h *Handler) ResetVM(args []string, res *int) error {
	*res = 0

	if len(args) != 1 {
		return fmt.Errorf("ResetVM: requires 1 argument")
	}

	logutils.Notice.Printf("ipc: ResetVM(%q)", args[0])

	if err := h.monitor.Reset(args[0]); err != nil {
		*res = 1
		return err
	}

	return nil
}

func (c *ClientHandler) ResetVM(name string) (int, error) {
	var response int
	if err := c.Client.Call(ServiceName+".ResetVM", []string{name}, &response); err != nil {
		return 2, err
	}
	return response, nil
}