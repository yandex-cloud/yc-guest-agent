package main

import (
	"context"
	"fmt"
	"log"
	"marketplace-yaga/linux/internal/guest"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/serial"

	"github.com/blang/semver/v4"
	"github.com/spf13/cobra"
)

const portName = "/dev/ttyS3"

func initAgent() (*guest.Server, error) {
	l, err := logger.NewLogger(logLevel, disableSerialSink)
	if err != nil {
		return nil, err
	}
	ctx := logger.NewContext(context.Background(), l)

	// it will try to lock COM4-port for exclusive use
	if err = serial.Init(portName); err != nil {
		return nil, err
	}

	s, err = guest.NewServer(ctx)
	if err != nil {
		return nil, err
	}

	return s, nil
}

var startCmd = &cobra.Command{
	Use:   "start",
	Args:  cobra.NoArgs,
	Short: "Start agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := initAgent()
		if err != nil {
			return err
		}

		return s.Run()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Args:  cobra.NoArgs,
	Short: "Print version",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := semver.Parse(version)
		if err != nil {
			return fmt.Errorf("wrong version format (probably built without correct ldflag): %w", err)
		}

		fmt.Println(v)

		return nil
	},
}

var (
	logLevel          string
	disableSerialSink bool
	s                 *guest.Server
	version           = "devel"
	rootCmd           = &cobra.Command{Use: "yandex-guest-agent"}
)

func main() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "")
	rootCmd.PersistentFlags().BoolVar(&disableSerialSink, "log-disable-serial", true, "")

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal("agent execution failed: ", err)
	}
}
