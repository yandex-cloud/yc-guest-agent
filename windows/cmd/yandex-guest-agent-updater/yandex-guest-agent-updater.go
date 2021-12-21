package main

import (
	"context"
	"fmt"
	"log"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/windows/internal/updater"
	"os"
	"os/signal"
	"syscall"

	"github.com/blang/semver/v4"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "dry-run",
	Short: "check state of guest agent and report desired action when running update",
	Long: "possible actions are:\n" +
		"\tNoop: no action will be performed.\n" +
		"\tDownload: download missing version in filerepo cache.\n" +
		"\tUpdate: guest agent will be updated\n" +
		"\tDownloadAndUpdate: guest agent will be downloaded and updated.\n" +
		"\tInstall: guest agent will be installed.\n" +
		"\tDownloadAndInstall: guest agent will be downloaded and installed.\n" +
		"\tUnknown: something wrong with current configuration, re-run with debug log-level for more info.",
	Run: func(cmd *cobra.Command, args []string) {
		u := mustInit()
		defer func() { _ = u.Close() }()

		state, err := u.Check()
		if err != nil {
			log.Fatal(fmt.Errorf("failed to run check: %w", err))
		}

		cmd.Println(state)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "update or install guest agent",
	Run: func(cmd *cobra.Command, args []string) {
		u := mustInit()
		defer func() { _ = u.Close() }()

		err := u.Update()
		if err != nil {
			log.Fatal(fmt.Errorf("failed to update: %w", err))
		}
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start guest agent service",
	Run: func(cmd *cobra.Command, args []string) {
		u := mustInit()
		defer func() { _ = u.Close() }()

		err := u.Start()
		if err != nil {
			log.Fatal(fmt.Errorf("failed to start guest agent service: %w", err))
		}
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop guest agent service",
	Run: func(cmd *cobra.Command, args []string) {
		u := mustInit()
		defer func() { _ = u.Close() }()

		err := u.Stop()
		if err != nil {
			log.Fatal(fmt.Errorf("failed to stop guest agent service: %w", err))
		}
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "remove guest agent",
	Run: func(cmd *cobra.Command, args []string) {
		u := mustInit()
		defer func() { _ = u.Close() }()

		err := u.Remove()
		if err != nil {
			log.Fatal(fmt.Errorf("failed to remove guest agent: %w", err))
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version of guest agent updater",
	Run: func(cmd *cobra.Command, args []string) {
		_, err := semver.Parse(version)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to parse (%v) guest agent updater version: %w", version, err))
		}

		cmd.Println(version)
	},
}

var (
	version  = "devel"
	logLevel = "info"
	rootCmd  = &cobra.Command{SilenceUsage: true, Use: "yandex-guest-agent-updater"}
)

func mustInit() *updater.GuestAgent {
	c, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()

	lgr, err := logger.NewLogger(logLevel, false)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to create logger: %w", err))
	}
	ctx := logger.NewContext(c, lgr)

	u, err := updater.New(ctx)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to create updater: %w", err))
	}

	if err = u.Init(); err != nil {
		log.Fatal(fmt.Errorf("failed to init updater: %w", err))
	}

	return u
}

func main() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "--log-level debug")

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal("agent execution failed: %w", err)
	}
}
