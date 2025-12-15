package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/config"
	appui "github.com/sachamama/sacha/internal/ui/app"
	logsui "github.com/sachamama/sacha/internal/ui/logs"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type cliFlags struct {
	profile string
	region  string
	service string
	verbose bool
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var flags cliFlags

	cmd := &cobra.Command{
		Use:   "sacha",
		Short: "Keyboard-first AWS TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.PersistentFlags().StringVar(&flags.profile, "profile", "", "AWS profile")
	cmd.PersistentFlags().StringVar(&flags.region, "region", "", "AWS region")
	cmd.PersistentFlags().StringVar(&flags.service, "service", "", "AWS service (cloudwatch-logs)")
	cmd.PersistentFlags().BoolVar(&flags.verbose, "verbose", false, "enable verbose logging")

	return cmd
}

func run(ctx context.Context, flags cliFlags) error {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if flags.verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	fileCfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	envCfg := config.FromEnv()
	runtime := config.Resolve(config.Flags{
		Profile: flags.profile,
		Region:  flags.region,
		Service: flags.service,
	}, envCfg, fileCfg)

	loader := awsx.NewLoader()

	awsCfg, err := loader.Load(ctx, runtime.Profile, runtime.Region)
	if err != nil {
		return err
	}
	if runtime.Region == "" {
		runtime.Region = awsCfg.Region
	}

	services := map[string]awsx.Service{
		"cloudwatch-logs": logsui.CloudWatchLogsService{},
	}

	appModel, err := appui.NewModel(loader, services, runtime, awsCfg, &log.Logger)
	if err != nil {
		return err
	}

	p := tea.NewProgram(appModel, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}

	if finalModel, ok := result.(appui.Model); ok {
		runtime = finalModel.Runtime()
	}

	fileCfg.LastRegion = runtime.Region
	fileCfg.LastService = runtime.Service
	return config.Save(cfgPath, fileCfg)
}
