package ipc

import (
	"github.com/rafaelmartins/simplevirt/pkg/logutils"
)

func (h *Handler) ListVMs(_ struct{}, res *[]string) error {
	logutils.Notice.Printf("ipc: ListVMs()")

	vms, err := h.monitor.List()
	if err != nil {
		return logutils.LogErrorR(err)
	}
	*res = vms
	return nil
}

func (c *ClientHandler) ListVMs() ([]string, error) {
	var response []string
	if err := c.Client.Call(ServiceName+".ListVMs", emptyStruct, &response); err != nil {
		return nil, err
	}
	return response, nil
}
