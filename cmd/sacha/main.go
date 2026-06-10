package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/config"
	appui "github.com/sachamama/sacha/internal/ui/app"
	dynamodbui "github.com/sachamama/sacha/internal/ui/dynamodb"
	ec2ui "github.com/sachamama/sacha/internal/ui/ec2"
	lambdaui "github.com/sachamama/sacha/internal/ui/lambda"
	logsui "github.com/sachamama/sacha/internal/ui/logs"
	s3ui "github.com/sachamama/sacha/internal/ui/s3"
	sqsui "github.com/sachamama/sacha/internal/ui/sqs"
	ssmui "github.com/sachamama/sacha/internal/ui/ssm"
	"github.com/sachamama/sacha/internal/version"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type cliFlags struct {
	profile  string
	region   string
	service  string
	endpoint string
	verbose  bool
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
		Use:     "sacha",
		Short:   "Keyboard-first AWS TUI",
		Version: version.Short(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.SetVersionTemplate(version.Info() + "\n")
	cmd.PersistentFlags().StringVar(&flags.profile, "profile", "", "AWS profile")
	cmd.PersistentFlags().StringVar(&flags.region, "region", "", "AWS region")
	cmd.PersistentFlags().StringVar(&flags.service, "service", "", "AWS service (cloudwatch-logs, s3, dynamodb, ec2, lambda, sqs, ssm)")
	cmd.PersistentFlags().StringVar(&flags.endpoint, "endpoint", "", "custom AWS endpoint URL (e.g. http://localhost:4566)")
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
		Profile:  flags.profile,
		Region:   flags.region,
		Service:  flags.service,
		Endpoint: flags.endpoint,
	}, envCfg, fileCfg)

	loader := awsx.NewLoader(runtime.Endpoint)

	awsCfg, err := loader.Load(ctx, runtime.Profile, runtime.Region)
	if err != nil {
		return err
	}
	if runtime.Region == "" {
		runtime.Region = awsCfg.Region
	}

	services := map[string]awsx.Service{
		"cloudwatch-logs": logsui.CloudWatchLogsService{},
		"s3":              s3ui.S3Service{},
		"dynamodb":        dynamodbui.DynamoDBService{},
		"ec2":             ec2ui.EC2Service{},
		"lambda":          lambdaui.LambdaService{},
		"sqs":             sqsui.SQSService{},
		"ssm":             ssmui.SSMService{},
	}

	if _, ok := services[runtime.Service]; !ok {
		if flags.service != "" {
			return fmt.Errorf("unknown service %q; available services: %s", runtime.Service, sortedServiceNames(services))
		}
		runtime.Service = config.DefaultService()
	}

	// persist writes the latest selection to disk immediately so a region or
	// service change survives abnormal termination (terminal closed, SIGHUP,
	// crash), not just a clean exit.
	persist := func(rt config.RuntimeConfig) error {
		fileCfg.LastProfile = rt.Profile
		fileCfg.LastRegion = rt.Region
		fileCfg.LastService = rt.Service
		return config.Save(cfgPath, fileCfg)
	}

	appModel, err := appui.NewModel(loader, services, runtime, awsCfg, &log.Logger, persist)
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

	return persist(runtime)
}

func sortedServiceNames(services map[string]awsx.Service) string {
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
