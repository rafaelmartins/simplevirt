package monitor

import (
	"net"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
	"github.com/rafaelmartins/simplevirt/pkg/netdev"
	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

type NIC struct {
	ID     string `json:"id"`
	Bridge string `json:"bridge"`
	iface  *net.Interface
}

func newNICs(vm string, config *qemu.VirtualMachine) []*NIC {
	nics := []*NIC{}

	var err error
	for i, nic := range config.NICs {
		if nic.Bridge == "" {
			continue
		}

		logutils.Notice.Printf("monitor: %s: %s: create qtap", vm, nic.Bridge)

		tap, err := netdev.CreateQtap(config.RunAs)
		if err != nil {
			logutils.LogError(err)
			break
		}

		if err = netdev.AddDevToBridge(nic.Bridge, tap); err != nil {
			netdev.DestroyQtap(tap)
			logutils.LogError(err)
			break
		}

		logutils.Notice.Printf("monitor: %s: %s: %s: done", vm, nic.Bridge, tap.Name)

		nics = append(nics, &NIC{ID: tap.Name, Bridge: nic.Bridge, iface: tap})
		config.NICs[i].SetDevice(tap.Name)
	}

	if err != nil {
		CleanupNICs(vm, nics)
		return nil
	}

	return nics
}

func (n *NIC) Cleanup(vm string) {
	logutils.Notice.Printf("monitor: %s: %s: %s: cleanup", vm, n.Bridge, n.ID)

	var err error
	if err = netdev.RemoveDevFromBridge(n.Bridge, n.iface); err != nil {
		logutils.LogError(err)
	} else if err = netdev.DestroyQtap(n.iface); err != nil {
		logutils.LogError(err)
	}

	if err == nil {
		logutils.Notice.Printf("monitor: %s: %s: %s: cleanup: done", vm, n.Bridge, n.ID)
	}
}

func CleanupNICs(vm string, nics []*NIC) {
	for _, nic := range nics {
		nic.Cleanup(vm)
	}
}
