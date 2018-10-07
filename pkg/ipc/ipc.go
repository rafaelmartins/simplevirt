package ipc

import (
	"net/rpc"

	"github.com/rafaelmartins/simplevirt/pkg/monitor"
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

func RegisterHandlers(configDir string, runtimeDir string) *monitor.Monitor {
	mon := monitor.NewMonitor(configDir, runtimeDir)
	hdr := Handler{
		configDir:  configDir,
		runtimeDir: runtimeDir,
		monitor:    mon,
	}
	rpc.RegisterName(ServiceName, &hdr)
	return mon
}

func NewClientHandler(client *rpc.Client) *ClientHandler {
	return &ClientHandler{Client: client}
}
