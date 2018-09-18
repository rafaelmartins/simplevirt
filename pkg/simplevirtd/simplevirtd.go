package simplevirtd

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/rafaelmartins/simplevirt/pkg/logutils"
)

var (
	configDir  string
	runtimeDir string
	socket     string
	syslogF    bool
)

func init() {
	cmd.Flags().StringVarP(&configDir, "configdir", "c", "/etc/simplevirt", "Directory with configuration files")
	cmd.Flags().StringVarP(&runtimeDir, "runtimedir", "m", "/run/simplevirt", "Directory to store QEMU runtime files")
	cmd.Flags().StringVarP(&socket, "socket", "s", "/run/simplevirtd.sock", "Unix socket to listen")
	cmd.Flags().BoolVar(&syslogF, "syslog", false, "Use syslog for logging instead of standard error output")
}

var cmd = &cobra.Command{
	Use:          "simplevirtd",
	Short:        "Simple virtual machine manager for Linux (QEMU/KVM) - Daemon",
	Long:         "Simple virtual machine manager for Linux (QEMU/KVM) - Daemon",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if syslogF {
			if err := logutils.UseSyslog("simplevirtd"); err != nil {
				return err
			}
		}

		logutils.Notice.Println("starting simplevirtd")

		u, err := user.Lookup("root")
		if err != nil {
			return err
		}

		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			return err
		}

		if os.Geteuid() != uid {
			return fmt.Errorf("simplevirtd should be run as root")
		}

		if socket == "" {
			return fmt.Errorf("empty Unix socket is invalid")
		}

		if runtimeDir == "" {
			return fmt.Errorf("empty runtime directory is invalid")
		}
		if _, err := os.Stat(runtimeDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(runtimeDir, 0777); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if err := listenAndServe(); err != nil {
			return err
		}

		return nil
	},
}

func Execute() {
	cmd.Execute()
}
