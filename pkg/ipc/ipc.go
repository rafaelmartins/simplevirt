package ipc

import (
	"net/rpc"
)

var (
	ProtocolVersion = 0
	ServiceName     = "SimpleVirt"

	emptyStruct struct{}
)

type Handler struct {
	configDir  string
	runtimeDir string
}

type ClientHandler struct {
	Client *rpc.Client
}

func RegisterHandlers(configDir string, runtimeDir string) {
	hdr := Handler{
		configDir:  configDir,
		runtimeDir: runtimeDir,
	}
	rpc.RegisterName(ServiceName, &hdr)
}

func NewClientHandler(client *rpc.Client) *ClientHandler {
	return &ClientHandler{Client: client}
}
