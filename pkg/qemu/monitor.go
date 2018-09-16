package qemu

import (
	"net"
)

func monitorSendCommand(monitor string, command string) error {
	conn, err := net.Dial("unix", monitor)
	if err != nil {
		return err
	}
	if _, err := conn.Write([]byte(command + "\n")); err != nil {
		return err
	}
	return nil
}

func monitorPowerdown(monitor string) error {
	return monitorSendCommand(monitor, "system_powerdown")
}

func monitorReset(monitor string) error {
	return monitorSendCommand(monitor, "system_reset")
}
