package qemu

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

type qmpError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

type qmpResponse struct {
	Return *json.RawMessage `json:"return"`
	Error  *qmpError        `json:"error"`
	Hello  *interface{}     `json:"QMP"`
}

type qmpQueryStatusResponse struct {
	Status  string `json:"status"`
	Running bool   `json:"running"`
}

func qmpCall(r *bufio.Reader, w *bufio.Writer, command string) (*json.RawMessage, error) {
	cmd, err := json.Marshal(map[string]string{"execute": command})
	if err != nil {
		return nil, err
	}

	if _, err := w.Write(append(cmd, '\x0a')); err != nil {
		return nil, err
	}

	if err := w.Flush(); err != nil {
		return nil, err
	}

	res, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	resp := &qmpResponse{}
	if err := json.Unmarshal(res, &resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("qmp: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return resp.Return, nil
}

func qmpSendCommand(qmp string, command string) (*json.RawMessage, error) {
	conn, err := net.Dial("unix", qmp)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	hello, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	resp := &qmpResponse{}
	if err := json.Unmarshal(hello, &resp); err != nil {
		return nil, err
	}

	if resp.Hello == nil {
		return nil, fmt.Errorf("qmp: invalid handshake")
	}

	if _, err := qmpCall(r, w, "qmp_capabilities"); err != nil {
		return nil, err
	}

	rv, err := qmpCall(r, w, command)
	if err != nil {
		return nil, err
	}

	return rv, nil
}

func qmpPowerdown(qmp string) error {
	_, err := qmpSendCommand(qmp, "system_powerdown")
	return err
}

func qmpReset(qmp string) error {
	_, err := qmpSendCommand(qmp, "system_reset")
	return err
}

func qmpQueryStatus(qmp string) (*qmpQueryStatusResponse, error) {
	cmd, err := qmpSendCommand(qmp, "query-status")
	if err != nil {
		return nil, err
	}

	rv := &qmpQueryStatusResponse{}
	if err := json.Unmarshal(*cmd, &rv); err != nil {
		return nil, err
	}

	return rv, nil
}
