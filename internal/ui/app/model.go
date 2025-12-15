package app

import (
	"context"
	"fmt"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rs/zerolog"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	loader   awsx.Loader
	services map[string]awsx.Service

	cfg     sdkaws.Config
	runtime config.RuntimeConfig

	service tea.Model

	logger *zerolog.Logger

	regionSelector  optionSelector
	serviceSelector optionSelector

	width    int
	height   int
	showHelp bool
	status   string
}

func NewModel(loader awsx.Loader, services map[string]awsx.Service, runtime config.RuntimeConfig, cfg sdkaws.Config, logger *zerolog.Logger) (Model, error) {
	m := Model{
		loader:   loader,
		services: services,
		runtime:  runtime,
		cfg:      cfg,
		logger:   logger,
	}
	m.regionSelector = newOptionSelector("Select Region", awsRegions)
	m.serviceSelector = newOptionSelector("Select Service", serviceNames(services))
	if err := m.activateService(runtime.Service); err != nil {
		return Model{}, err
	}
	return m, nil
}

func (m Model) Init() tea.Cmd {
	if m.service != nil {
		return m.service.Init()
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.service != nil {
			var cmd tea.Cmd
			m.service, cmd = m.service.Update(msg)
			return m, cmd
		}
	case tea.KeyMsg:
		if m.regionSelector.active {
			return m.handleRegionSelector(msg)
		}
		if m.serviceSelector.active {
			return m.handleServiceSelector(msg)
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !isTailing(m.service) {
				return m, tea.Quit
			}
		case "r":
			m.regionSelector.open(awsRegions, m.runtime.Region)
			return m, m.regionSelector.input.Focus()
		case "s":
			m.serviceSelector.open(serviceNames(m.services), m.runtime.Service)
			return m, m.serviceSelector.input.Focus()
		case "?":
			m.showHelp = !m.showHelp
		}
	}

	if m.service != nil {
		var cmd tea.Cmd
		m.service, cmd = m.service.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	header := fmt.Sprintf("profile: %s | region: %s | service: %s", emptyIf(m.runtime.Profile, "default"), emptyIf(m.runtime.Region, "sdk-default"), m.runtime.Service)
	if m.regionSelector.active {
		return m.overlayView(header, m.regionSelector.View(minWidth(m.width, 60)))
	}
	if m.serviceSelector.active {
		return m.overlayView(header, m.serviceSelector.View(minWidth(m.width, 60)))
	}
	if m.showHelp {
		return header + "\n" + helpView()
	}
	body := ""
	if m.service != nil {
		body = m.service.View()
	}
	status := m.status
	if status == "" {
		status = "Keys: arrows/jk move, / search, space select, a select all, t tail, r region, s service, ? help, q stop tail, ctrl+c quit"
	}
	return fmt.Sprintf("%s\n%s\n%s", header, body, status)
}

func (m *Model) activateService(name string) error {
	svc, ok := m.services[name]
	if !ok {
		return fmt.Errorf("unknown service %q", name)
	}
	model, err := svc.Init(context.Background(), m.cfg, awsx.ServiceOptions{
		Logger: newLoggerAdapter(m.logger),
	})
	if err != nil {
		return err
	}
	m.runtime.Service = name
	m.service = model
	return nil
}

func (m *Model) changeRegion(region string) (tea.Cmd, error) {
	cfg, err := m.loader.Load(context.Background(), m.runtime.Profile, region)
	if err != nil {
		return nil, err
	}
	m.cfg = cfg
	m.runtime.Region = region
	cmds := []tea.Cmd{}
	if err := m.activateService(m.runtime.Service); err != nil {
		return nil, err
	}
	if m.service != nil {
		cmds = append(cmds, m.service.Init())
	}
	if m.width > 0 && m.height > 0 {
		cmds = append(cmds, func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		})
	}
	return tea.Batch(cmds...), nil
}

func helpView() string {
	return "Navigation: arrows/j/k | Search: / | Select: space, a | Actions: t tail, r region, s service | Tail stop: q or esc | Quit app: ctrl+c"
}

func emptyIf(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// logger adapter to satisfy ServiceLogger without exposing zerolog directly.
type loggerAdapter struct {
	logger *zerolog.Logger
}

func newLoggerAdapter(l *zerolog.Logger) loggerAdapter {
	return loggerAdapter{logger: l}
}

func (l loggerAdapter) Debug(msg string, kv ...interface{}) {
	if l.logger == nil {
		return
	}
	fields := mapToFields(kv)
	l.logger.Debug().Fields(fields).Msg(msg)
}

func (l loggerAdapter) Info(msg string, kv ...interface{}) {
	if l.logger == nil {
		return
	}
	fields := mapToFields(kv)
	l.logger.Info().Fields(fields).Msg(msg)
}

func (l loggerAdapter) Error(msg string, kv ...interface{}) {
	if l.logger == nil {
		return
	}
	fields := mapToFields(kv)
	l.logger.Error().Fields(fields).Msg(msg)
}

func mapToFields(kv []interface{}) map[string]interface{} {
	fields := map[string]interface{}{}
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		fields[key] = kv[i+1]
	}
	return fields
}

type tailAware interface {
	Tailing() bool
}

func isTailing(m tea.Model) bool {
	if t, ok := m.(tailAware); ok {
		return t.Tailing()
	}
	return false
}

// Runtime exposes the current runtime configuration after user interaction.
func (m Model) Runtime() config.RuntimeConfig {
	return m.runtime
}

func (m Model) handleRegionSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	choice, cmd := m.regionSelector.update(msg)
	if choice != "" {
		changeCmd, err := m.changeRegion(choice)
		if err != nil {
			m.status = err.Error()
			return m, cmd
		}
		return m, tea.Batch(cmd, changeCmd)
	}
	return m, cmd
}

func (m Model) handleServiceSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	choice, cmd := m.serviceSelector.update(msg)
	if choice != "" {
		if err := m.activateService(choice); err != nil {
			m.status = err.Error()
			return m, cmd
		}
		initCmds := []tea.Cmd{}
		if m.service != nil {
			initCmds = append(initCmds, m.service.Init())
		}
		if m.width > 0 && m.height > 0 {
			initCmds = append(initCmds, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			})
		}
		return m, tea.Batch(append(initCmds, cmd)...)
	}
	return m, cmd
}
