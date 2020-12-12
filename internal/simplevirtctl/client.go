package simplevirtctl

import (
	"fmt"
	"net/rpc"

	"github.com/rafaelmartins/simplevirt/internal/ipc"
)

type Client struct {
	RPCClient *rpc.Client
	Handler   *ipc.ClientHandler
}

func NewClient(path string) (*Client, error) {
	c, err := rpc.Dial("unix", path)
	if err != nil {
		return nil, err
	}

	h := ipc.NewClientHandler(c)
	client := &Client{RPCClient: c, Handler: h}

	version, err := client.Handler.GetProtocolVersion()
	if err != nil {
		return nil, err
	}

	if version != ipc.ProtocolVersion {
		return nil, fmt.Errorf("simplevirtctl: unsupported protocol version: %d", version)
	}

	return client, nil
}

func (c *Client) Close() error {
	if c.RPCClient != nil {
		return c.RPCClient.Close()
	}
	return nil
}
