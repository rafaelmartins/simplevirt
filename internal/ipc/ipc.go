package ipc

import (
	"net/rpc"

	"github.com/rafaelmartins/simplevirt/internal/monitor"
)

var (
	ProtocolVersion = 0
	ServiceName     = "SimpleVirt"

	emptyStruct struct{}
)

type Handler struct {
	configDir  string
	runtimeDir string
	monitor    *monitor.Monitor
}

type ClientHandler struct {
	Client *rpc.Client
}

func RegisterHandlers(configDir string, runtimeDir string) (*monitor.Monitor, error) {
	mon, err := monitor.NewMonitor(configDir, runtimeDir)
	if err != nil {
		return nil, err
	}
	hdr := Handler{
		configDir:  configDir,
		runtimeDir: runtimeDir,
		monitor:    mon,
	}
	rpc.RegisterName(ServiceName, &hdr)
	return mon, nil
}

func NewClientHandler(client *rpc.Client) *ClientHandler {
	return &ClientHandler{Client: client}
}
