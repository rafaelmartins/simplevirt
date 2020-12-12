package simplevirtd

import (
	"log"
	"os"
	"os/user"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/rafaelmartins/simplevirt/internal/logutils"
	"github.com/rafaelmartins/simplevirt/internal/version"
)

var (
	configDir  string
	runtimeDir string
	socket     string
	syslogF    bool
	logLevel   string
)

func init() {
	cmd.Flags().StringVarP(&configDir, "configdir", "c", "/etc/simplevirt", "Directory with configuration files")
	cmd.Flags().StringVarP(&runtimeDir, "runtimedir", "m", "/run/simplevirt", "Directory to store QEMU runtime files")
	cmd.Flags().StringVarP(&socket, "socket", "s", "/run/simplevirtd.sock", "Unix socket to listen")
	cmd.Flags().BoolVar(&syslogF, "syslog", false, "Use syslog for logging instead of standard error output")
	cmd.Flags().StringVarP(&logLevel, "loglevel", "l", "WARNING", "Log level for non-syslog logging (CRITICAL, ERROR, WARNING, NOTICE)")
}

var cmd = &cobra.Command{
	Use:          "simplevirtd",
	Short:        "Simple virtual machine manager for Linux (QEMU/KVM) - Daemon",
	Long:         "Simple virtual machine manager for Linux (QEMU/KVM) - Daemon",
	Version:      version.Version,
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		if syslogF {
			if err := logutils.UseSyslog("simplevirtd"); err != nil {
				log.Fatal(err)
			}
		} else {
			if logLevel == "" {
				log.Fatal("empty log level is invalid")
			}
			if err := logutils.SetLevel(logLevel); err != nil {
				log.Fatal(err)
			}
		}

		logutils.Notice.Printf("starting simplevirtd %s\n", version.Version)

		u, err := user.Lookup("root")
		if err != nil {
			logutils.Error.Fatal(err)
		}

		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			logutils.Error.Fatal(err)
		}

		if os.Geteuid() != uid {
			logutils.Error.Fatal("simplevirtd should be run as root")
		}

		if socket == "" {
			logutils.Error.Fatal("empty Unix socket is invalid")
		}

		if runtimeDir == "" {
			logutils.Error.Fatal("empty runtime directory is invalid")
		}
		if _, err := os.Stat(runtimeDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(runtimeDir, 0777); err != nil {
					logutils.Error.Fatal(err)
				}
			} else {
				logutils.Error.Fatal(err)
			}
		}

		if err := listenAndServe(); err != nil {
			logutils.Error.Fatal(err)
		}
	},
}

func Execute() {
	cmd.Execute()
}
