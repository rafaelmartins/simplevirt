package ipc

import (
	"fmt"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
)

func (h *Handler) RestartVM(args []string, res *int) error {
	*res = 0

	if len(args) != 1 {
		return fmt.Errorf("RestartVM: requires 1 argument")
	}

	logutils.Notice.Printf("ipc: RestartVM(%q)", args[0])

	mErr := make(chan error)

	if err := h.monitor.Shutdown(args[0], mErr); err != nil {
		*res = 1
		return logutils.LogErrorR(err)
	}

	if err := <-mErr; err != nil {
		*res = 1
		return logutils.LogErrorR(err)
	}

	if err := h.monitor.Start(args[0], mErr); err != nil {
		*res = 1
		return logutils.LogErrorR(err)
	}

	if err := <-mErr; err != nil {
		*res = 1
		return logutils.LogErrorR(err)
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
