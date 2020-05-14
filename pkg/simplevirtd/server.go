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
)

func listenAndServe() error {
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

	mon, err := ipc.RegisterHandlers(configDir, runtimeDir)
	if err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)

	exiting := false

	go func(l net.Listener, c chan os.Signal) {
		sig := <-c

		logutils.Error.Printf("caught signal %q: shutting down virtual machines.\n", sig)
		mon.Cleanup()

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

			mon.Cleanup()
			return err
		}
		go rpc.ServeConn(conn)
	}

	mon.Cleanup()
	return nil
}
