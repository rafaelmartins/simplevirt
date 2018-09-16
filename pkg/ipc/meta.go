package ipc

func (h *Handler) GetProtocolVersion(_ struct{}, res *int) error {
	*res = ProtocolVersion
	return nil
}

func (c *ClientHandler) GetProtocolVersion() (int, error) {
	var response int
	if err := c.Client.Call(ServiceName+".GetProtocolVersion", emptyStruct, &response); err != nil {
		return 2, err
	}
	return response, nil
}
