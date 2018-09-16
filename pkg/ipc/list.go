package ipc

import (
	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

func (h *Handler) ListVMs(_ struct{}, res *[]string) error {
	vms, err := qemu.List(h.configDir)
	if err != nil {
		return err
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
