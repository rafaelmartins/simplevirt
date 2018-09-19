package simplevirtctl

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rafaelmartins/simplevirt"
)

var (
	socket string

	client *Client
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&socket, "socket", "s", "/run/simplevirtd.sock", "Unix socket to connect")
}

var rootCmd = &cobra.Command{
	Use:          "simplevirtctl",
	Short:        "Simple virtual machine manager for Linux (QEMU/KVM) - Controller",
	Long:         "Simple virtual machine manager for Linux (QEMU/KVM) - Controller",
	Version:      simplevirt.Version,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if socket == "" {
			return fmt.Errorf("empty socket file name is invalid")
		}

		var err error
		client, err = NewClient(socket)
		if err != nil {
			return err
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if client != nil {
			return client.Close()
		}
		return nil
	},
}

var startCmd = &cobra.Command{
	Use:   "start NAME",
	Short: "Starts a virtual machine",
	Long:  "This command starts a virtual machine, if not running.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rv, err := client.Handler.StartVM(args[0])
		if err != nil {
			return err
		}

		if rv != 0 {
			os.Exit(rv)
		}

		return nil
	},
}

var shutdownCmd = &cobra.Command{
	Use:   "shutdown NAME",
	Short: "Shutdown a virtual machine",
	Long:  "This command shutdown a virtual machine, if running.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rv, err := client.Handler.ShutdownVM(args[0])
		if err != nil {
			return err
		}

		if rv != 0 {
			os.Exit(rv)
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status [NAME] ...",
	Short: "List status of virtual machines",
	Long:  "This command lists the status of a virtual machine, or of all availeble virtual machines.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vms, err := client.Handler.ListVMs()
		if err != nil {
			return err
		}
		vvms := vms
		if len(args) == 1 {
			vvms = args
		}

		size := 0
		for _, vm := range vvms {
			if len(vm) > size {
				size = len(vm)
			}
		}

		for _, vm := range vvms {
			status, err := client.Handler.GetVMStatus(vm)
			if err != nil {
				return err
			}
			fmt.Printf("%-*s: %s\n", size, vm, status)
		}

		return nil
	},
}

func Execute() {
	rootCmd.AddCommand(
		startCmd,
		shutdownCmd,
		statusCmd,
	)
	rootCmd.Execute()
}
