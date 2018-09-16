package qemu

import (
	"testing"

	. "github.com/rafaelmartins/simplevirt/pkg/testutils"
)

var (
	n []string = nil
)

func TestBuildCmdDrive(t *testing.T) {
	val, err := buildCmdDrive(1, &drive{})
	AssertError(t, err, "qemu: drive[1].file: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdDrive(1, &drive{
		File: "foo.img",
	})
	AssertError(t, err, "qemu: drive[1].file: path must be absolute")
	AssertEqual(t, val, n)

	val, err = buildCmdDrive(1, &drive{
		File: "/foo.img",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none",
	})

	val, err = buildCmdDrive(1, &drive{
		File: "/fo,o,img",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/fo,,o,,img,if=virtio,media=disk,cache=none",
	})

	val, err = buildCmdDrive(1, &drive{
		File:      "/foo.img",
		Interface: "ide",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=ide,media=disk,cache=none",
	})

	val, err = buildCmdDrive(1, &drive{
		File:      "/foo.img",
		Interface: "bola",
	})
	AssertError(t, err, "qemu: drive[1].interface: invalid value (bola). valid choices are: 'ide', 'scsi', 'sd', 'mtd', 'floppy', 'pflash', 'virtio', 'none'")
	AssertEqual(t, val, n)

	val, err = buildCmdDrive(1, &drive{
		File:  "/foo.img",
		Media: "cdrom",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=virtio,media=cdrom,cache=none",
	})

	val, err = buildCmdDrive(1, &drive{
		File:  "/foo.img",
		Media: "bola",
	})
	AssertError(t, err, "qemu: drive[1].media: invalid value (bola). valid choices are: 'disk', 'cdrom'")
	AssertEqual(t, val, n)

	val, err = buildCmdDrive(1, &drive{
		File:  "/foo.img",
		Cache: "writeback",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=writeback",
	})

	val, err = buildCmdDrive(1, &drive{
		File:  "/foo.img",
		Cache: "bola",
	})
	AssertError(t, err, "qemu: drive[1].cache: invalid value (bola). valid choices are: 'none', 'writeback', 'unsafe', 'directsync', 'writethrough'")
	AssertEqual(t, val, n)

	val, err = buildCmdDrive(1, &drive{
		File:   "/foo.img",
		Format: "raw",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none,format=raw",
	})

	val, err = buildCmdDrive(1, &drive{
		File:   "/foo.img",
		Format: "bola",
	})
	AssertError(t, err, "qemu: drive[1].format: invalid value (bola). valid choices are: 'raw'")
	AssertEqual(t, val, n)

	val, err = buildCmdDrive(1, &drive{
		File:     "/foo.img",
		Snapshot: true,
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none,snapshot=on",
	})
}

func TestBuildCmdDrives(t *testing.T) {
	val, err := buildCmdDrives(nil)
	AssertError(t, err, "qemu: drive: at least one drive must be defined")
	AssertEqual(t, val, n)

	val, err = buildCmdDrives([]*drive{})
	AssertError(t, err, "qemu: drive: at least one drive must be defined")
	AssertEqual(t, val, n)

	val, err = buildCmdDrives([]*drive{
		&drive{},
	})
	AssertError(t, err, "qemu: drive[1].file: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdDrives([]*drive{
		&drive{File: "/foo.img"},
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none",
	})

	val, err = buildCmdDrives([]*drive{
		&drive{},
		&drive{},
	})
	AssertError(t, err, "qemu: drive[1].file: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdDrives([]*drive{
		&drive{File: "/foo.img"},
		&drive{},
	})
	AssertError(t, err, "qemu: drive[2].file: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdDrives([]*drive{
		&drive{File: "/foo.img"},
		&drive{File: "/bar.img"},
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none",
		"-drive", "file=/bar.img,if=virtio,media=disk,cache=none",
	})
}

func TestBuildCmdNIC(t *testing.T) {
	val, err := buildCmdNIC(1, &nic{})
	AssertError(t, err, "qemu: nic[1].mac_address: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdNIC(1, &nic{
		MACAddr: "bola",
	})
	AssertError(t, err, "qemu: nic[1].mac_address: invalid value (address bola: invalid MAC address)")
	AssertEqual(t, val, n)

	val, err = buildCmdNIC(1, &nic{
		MACAddr: "52:54:00:fc:70:3b",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-nic", "user,mac=52:54:00:fc:70:3b,model=virtio",
	})

	val, err = buildCmdNIC(1, &nic{
		MACAddr: "52:54:00:fc:70:3b",
		Model:   "e1000",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-nic", "user,mac=52:54:00:fc:70:3b,model=e1000",
	})

	val, err = buildCmdNIC(1, &nic{
		MACAddr:     "52:54:00:fc:70:3b",
		NetUserArgs: map[string]string{"foo": "bar"},
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-nic", "user,foo=bar,mac=52:54:00:fc:70:3b,model=virtio",
	})

	val, err = buildCmdNIC(1, &nic{
		MACAddr: "52:54:00:fc:70:3b",
		Bridge:  "br0",
	})
	AssertError(t, err, "qemu: nic[1]: missing device")
	AssertEqual(t, val, n)

	val, err = buildCmdNIC(1, &nic{
		MACAddr: "52:54:00:fc:70:3b",
		Bridge:  "br0",
		device:  "qtap0",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-nic", "tap,ifname=qtap0,script=no,mac=52:54:00:fc:70:3b,model=virtio",
	})

	val, err = buildCmdNIC(1, &nic{
		MACAddr: "52:54:00:fc:70:3b",
		Bridge:  "br0",
		device:  "qtap0",
		Model:   "e1000",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-nic", "tap,ifname=qtap0,script=no,mac=52:54:00:fc:70:3b,model=e1000",
	})
}

func TestBuildCmdNICs(t *testing.T) {
	val, err := buildCmdNICs(nil)
	AssertError(t, err, "qemu: nic: at least one NIC must be defined")
	AssertEqual(t, val, n)

	val, err = buildCmdNICs([]*nic{})
	AssertError(t, err, "qemu: nic: at least one NIC must be defined")
	AssertEqual(t, val, n)

	val, err = buildCmdNICs([]*nic{
		&nic{},
	})
	AssertError(t, err, "qemu: nic[1].mac_address: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdNICs([]*nic{
		&nic{MACAddr: "52:54:00:fc:70:3b"},
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-nic", "user,mac=52:54:00:fc:70:3b,model=virtio",
	})

	val, err = buildCmdNICs([]*nic{
		&nic{},
		&nic{},
	})
	AssertError(t, err, "qemu: nic[1].mac_address: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdNICs([]*nic{
		&nic{MACAddr: "52:54:00:fc:70:3b"},
		&nic{},
	})
	AssertError(t, err, "qemu: nic[2].mac_address: parameter is required")
	AssertEqual(t, val, n)

	val, err = buildCmdNICs([]*nic{
		&nic{MACAddr: "52:54:00:fc:70:3b"},
		&nic{MACAddr: "52:54:00:fc:70:3c"},
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-nic", "user,mac=52:54:00:fc:70:3b,model=virtio",
		"-nic", "user,mac=52:54:00:fc:70:3c,model=virtio",
	})
}

func TestBuildCmdVirtualMachine(t *testing.T) {
	val, err := buildCmdVirtualMachine(nil)
	AssertError(t, err, "qemu: virtualmachine: not defined")
	AssertEqual(t, val, n)

	val, err = buildCmdVirtualMachine(&virtualmachine{})
	AssertError(t, err, "qemu: drive: at least one drive must be defined")
	AssertEqual(t, val, n)

	val, err = buildCmdVirtualMachine(&virtualmachine{
		Drives: []*drive{
			&drive{File: "/foo.img"},
		},
	})
	AssertError(t, err, "qemu: nic: at least one NIC must be defined")
	AssertEqual(t, val, n)

	val, err = buildCmdVirtualMachine(&virtualmachine{
		Drives: []*drive{
			&drive{File: "/foo.img"},
		},
		NICs: []*nic{
			&nic{MACAddr: "52:54:00:fc:70:3b"},
		},
		RAM: "10.5A",
	})
	AssertError(t, err, "qemu: virtualmachine: invalid RAM size (10.5A)")
	AssertEqual(t, val, n)

	val, err = buildCmdVirtualMachine(&virtualmachine{
		Drives: []*drive{
			&drive{File: "/foo.img"},
		},
		NICs: []*nic{
			&nic{MACAddr: "52:54:00:fc:70:3b"},
		},
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-display", "none",
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none",
		"-nic", "user,mac=52:54:00:fc:70:3b,model=virtio",
	})

	val, err = buildCmdVirtualMachine(&virtualmachine{
		Drives: []*drive{
			&drive{File: "/foo.img"},
		},
		NICs: []*nic{
			&nic{MACAddr: "52:54:00:fc:70:3b"},
		},
		RAM: "400M",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-m", "size=400M",
		"-display", "none",
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none",
		"-nic", "user,mac=52:54:00:fc:70:3b,model=virtio",
	})

	val, err = buildCmdVirtualMachine(&virtualmachine{
		Drives: []*drive{
			&drive{File: "/foo.img"},
		},
		NICs: []*nic{
			&nic{MACAddr: "52:54:00:fc:70:3b"},
		},
		RAM: "4.5G",
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-m", "size=4.5G",
		"-display", "none",
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none",
		"-nic", "user,mac=52:54:00:fc:70:3b,model=virtio",
	})

	val, err = buildCmdVirtualMachine(&virtualmachine{
		name:    "bola",
		monitor: "/run/bola.sock",
		pidfile: "/run/bola.pid",
		Drives: []*drive{
			&drive{File: "/foo.img"},
		},
		NICs: []*nic{
			&nic{MACAddr: "52:54:00:fc:70:3b"},
		},
		Boot: map[string]string{
			"order": "cd",
		},
		MachineType:    "pc",
		EnableKVM:      true,
		RunAs:          "nobody",
		CPUModel:       "host",
		CPUs:           4,
		RAM:            "4.5G",
		VNCDisplay:     "127.0.0.1:1",
		AdditionalArgs: []string{"-asd", "qwe"},
	})
	AssertNonError(t, err)
	AssertEqual(t, val, []string{
		"-name", "bola",
		"-monitor", "unix:/run/bola.sock,server,nowait",
		"-daemonize",
		"-pidfile", "/run/bola.pid",
		"-M", "pc",
		"-enable-kvm",
		"-runas", "nobody",
		"-cpu", "host",
		"-smp", "cpus=4",
		"-m", "size=4.5G",
		"-boot", "order=cd",
		"-display", "vnc=127.0.0.1:1",
		"-drive", "file=/foo.img,if=virtio,media=disk,cache=none",
		"-nic", "user,mac=52:54:00:fc:70:3b,model=virtio",
		"-asd", "qwe",
	})
}
