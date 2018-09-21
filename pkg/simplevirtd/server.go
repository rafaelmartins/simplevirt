package simplevirtd

import (
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"

	"github.com/rafaelmartins/simplevirt/pkg/ipc"
	"github.com/rafaelmartins/simplevirt/pkg/logutils"
	"github.com/rafaelmartins/simplevirt/pkg/qemu"
)

func listenAndServe() error {
	ipc.RegisterHandlers(configDir, runtimeDir)

	if err := qemu.AutoStart(configDir, runtimeDir); err != nil {
		return err
	}

	gr, err := user.LookupGroup("simplevirt")
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(gr.Gid)
	if err != nil {
		return err
	}

	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer listener.Close()

	if err := os.Chmod(socket, 0660); err != nil {
		return err
	}

	if err := os.Chown(socket, -1, gid); err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP)

	exiting := false

	go func(l net.Listener, c chan os.Signal) {
		sig := <-c
		if sig == syscall.SIGHUP {
			logutils.Notice.Printf("caught signal %q: shutting down.\n", sig)
		} else {
			logutils.Notice.Printf("caught signal %q: shutting down virtual machines.\n", sig)
			qemu.Cleanup()
		}

		exiting = true
		l.Close()

		os.Exit(0)
	}(listener, sigChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if exiting {
				return nil
			}
			return err
		}
		go rpc.ServeConn(conn)
	}

	qemu.Cleanup()

	return nil
}
