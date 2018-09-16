package netdev

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

const (
	SIOCBRADDIF = 0x89a2
	SIOCBRDELIF = 0x89a3
)

var (
	reQtap       = regexp.MustCompile("qtap([0-9]+)")
	newQtapMutex = &sync.Mutex{}
)

type tapSyscall struct {
	Name  [syscall.IFNAMSIZ]byte
	Flags uint16
}

type bridgeSyscall struct {
	Name  [syscall.IFNAMSIZ]byte
	Index int32
}

func guessNextQtap() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var ids []int
	for _, iface := range ifaces {
		m := reQtap.FindStringSubmatch(iface.Name)

		if len(m) == 0 {
			continue
		}

		id, err := strconv.Atoi(m[1])
		if err != nil {
			return "", err
		}
		ids = append(ids, id)
	}

	for i := 0; i <= len(ids); i++ {
		found := false
		for _, j := range ids {
			if i == j {
				found = true
			}
		}
		if !found {
			return fmt.Sprintf("qtap%d", i), nil
		}
	}

	return "", fmt.Errorf("netdev: failed to find next available qtap interface")
}

func CreateQtap(owner string) (*net.Interface, error) {
	if owner == "" {
		return nil, fmt.Errorf("netdev: virtual machine owner not defined")
	}

	newQtapMutex.Lock()
	defer newQtapMutex.Unlock()

	name, err := guessNextQtap()
	if err != nil {
		return nil, err
	}

	us, err := user.Lookup(owner)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(us.Uid)
	if err != nil {
		return nil, err
	}

	gr, err := user.LookupGroup("simplevirt")
	if err != nil {
		return nil, err
	}
	gid, err := strconv.Atoi(gr.Gid)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	arg := tapSyscall{Flags: syscall.IFF_TAP | syscall.IFF_NO_PI}
	copy(arg.Name[:syscall.IFNAMSIZ-1], name)

	fd := file.Fd()

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TUNSETIFF),
		uintptr(unsafe.Pointer(&arg))); errno != 0 {
		return nil, os.NewSyscallError("netdev: failed ioctl TUNSETIFF", errno)
	}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TUNSETOWNER),
		uintptr(uid)); errno != 0 {
		return nil, os.NewSyscallError("netdev: failed ioctl TUNSETOWNER", errno)
	}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TUNSETGROUP),
		uintptr(gid)); errno != 0 {
		return nil, os.NewSyscallError("netdev: failed ioctl TUNSETGROUP", errno)
	}

	// this should be the last syscall, because we want the device to be
	// cleaned up if one of the previous syscalls failed. and if this one
	// fails, the device will be cleaned too.
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TUNSETPERSIST),
		uintptr(1)); errno != 0 {
		return nil, os.NewSyscallError("netdev: failed ioctl TUNSETPERSIST", errno)
	}

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	return iface, nil
}

func DestroyQtap(dev *net.Interface) error {
	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	iface := tapSyscall{Flags: 0x0002 | 0x1000}
	copy(iface.Name[:syscall.IFNAMSIZ-1], dev.Name)

	fd := file.Fd()

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TUNSETIFF),
		uintptr(unsafe.Pointer(&iface))); errno != 0 {
		return os.NewSyscallError("netdev: failed ioctl TUNSETIFF", errno)
	}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TUNSETPERSIST),
		uintptr(0)); errno != 0 {
		return os.NewSyscallError("netdev: failed ioctl TUNSETPERSIST", errno)
	}

	return nil
}

func devToBridge(bridge string, dev *net.Interface, add bool) error {
	fd, err := syscall.Socket(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	if !add {
		return devUpDown(uintptr(fd), dev, add)
	}

	bridgeIface, err := net.InterfaceByName(bridge)
	if err != nil {
		return err
	}

	arg := bridgeSyscall{Index: int32(dev.Index)}
	copy(arg.Name[:syscall.IFNAMSIZ-1], bridgeIface.Name)

	operation := SIOCBRDELIF
	if add {
		operation = SIOCBRADDIF
	}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(operation),
		uintptr(unsafe.Pointer(&arg))); errno != 0 {
		if add {
			return os.NewSyscallError("netdev: failed ioctl SIOCBRADDIF", errno)
		} else {
			return os.NewSyscallError("netdev: failed ioctl SIOCBRDELIF", errno)
		}
	}

	if add {
		return devUpDown(uintptr(fd), dev, add)
	}

	return nil
}

func devUpDown(fd uintptr, dev *net.Interface, up bool) error {
	var arg tapSyscall
	copy(arg.Name[:syscall.IFNAMSIZ-1], dev.Name)

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.SIOCGIFFLAGS),
		uintptr(unsafe.Pointer(&arg))); errno != 0 {
		return os.NewSyscallError("netdev: failed ioctl SIOCGIFFLAGS", errno)
	}

	if up {
		arg.Flags |= syscall.IFF_UP | syscall.IFF_PROMISC
	} else {
		arg.Flags = arg.Flags &^ syscall.IFF_UP
	}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.SIOCSIFFLAGS),
		uintptr(unsafe.Pointer(&arg))); errno != 0 {
		return os.NewSyscallError("netdev: failed ioctl SIOCSIFFLAGS", errno)
	}

	return nil
}

func AddDevToBridge(bridge string, dev *net.Interface) error {
	return devToBridge(bridge, dev, true)
}

func RemoveDevFromBridge(bridge string, dev *net.Interface) error {
	return devToBridge(bridge, dev, false)
}
