package monitor

import (
	"fmt"
	"net"
	"strings"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
	"github.com/rafaelmartins/simplevirt/pkg/netdev"
	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

type NIC struct {
	ID     string `json:"id"`
	Bridge string `json:"bridge"`
	iface  *net.Interface
}

func newNICs(vm string, config *qemu.VirtualMachine) ([]*NIC, error) {
	nics := []*NIC{}

	errs := []string{}
	for i, nic := range config.NICs {
		if nic.Bridge == "" {
			continue
		}

		logutils.Notice.Printf("monitor: %s: %s: create qtap", vm, nic.Bridge)

		tap, err := netdev.CreateQtap(config.RunAs)
		if err != nil {
			errs = append(errs, logutils.LogError(err).Error())
			break
		}

		if err := netdev.AddDevToBridge(nic.Bridge, tap); err != nil {
			errs = append(errs, logutils.LogError(err).Error())
			if err2 := netdev.DestroyQtap(tap); err2 != nil {
				errs = append(errs, logutils.LogError(err2).Error())
			}
			break
		}

		logutils.Notice.Printf("monitor: %s: %s: %s: done", vm, nic.Bridge, tap.Name)

		nics = append(nics, &NIC{ID: tap.Name, Bridge: nic.Bridge, iface: tap})
		config.NICs[i].SetDevice(tap.Name)
	}

	if len(errs) > 0 {
		if err2 := CleanupNICs(vm, nics); err2 != nil {
			errs = append(errs, logutils.LogError(err2).Error())
		}

		return nil, fmt.Errorf(strings.Join(errs, "\n"))
	}

	return nics, nil
}

func (n *NIC) Cleanup(vm string) error {
	logutils.Notice.Printf("monitor: %s: %s: %s: cleanup", vm, n.Bridge, n.ID)

	errs := []string{}
	if err := netdev.RemoveDevFromBridge(n.Bridge, n.iface); err != nil {
		errs = append(errs, logutils.LogError(err).Error())
	}
	if err := netdev.DestroyQtap(n.iface); err != nil {
		errs = append(errs, logutils.LogError(err).Error())
	}

	if len(errs) > 0 {
		logutils.Notice.Printf("monitor: %s: %s: %s: cleanup: done", vm, n.Bridge, n.ID)
		return fmt.Errorf(strings.Join(errs, "\n"))
	}

	return nil
}

func CleanupNICs(vm string, nics []*NIC) error {
	errs := []string{}
	for _, nic := range nics {
		if err := nic.Cleanup(vm); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "\n"))
	}

	return nil
}
