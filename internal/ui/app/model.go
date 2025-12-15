package app

import (
	"context"
	"fmt"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rs/zerolog"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	loader   awsx.Loader
	services map[string]awsx.Service

	cfg     sdkaws.Config
	runtime config.RuntimeConfig

	service tea.Model

	logger *zerolog.Logger

	regionInput   textinput.Model
	selectRegion  bool
	selectService bool

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
	m.regionInput = textinput.New()
	m.regionInput.Placeholder = "region (e.g. us-east-1)"
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
		if m.selectRegion {
			switch msg.Type {
			case tea.KeyEnter:
				m.selectRegion = false
				region := m.regionInput.Value()
				cmd, err := m.changeRegion(region)
				if err != nil {
					m.status = err.Error()
					return m, nil
				}
				return m, cmd
			case tea.KeyEscape:
				m.selectRegion = false
				return m, nil
			}
			var cmd tea.Cmd
			m.regionInput, cmd = m.regionInput.Update(msg)
			return m, cmd
		}

		if m.selectService {
			switch msg.Type {
			case tea.KeyEnter:
				m.selectService = false
				// Only CloudWatch Logs is available for now.
				m.runtime.Service = "cloudwatch-logs"
			case tea.KeyEscape:
				m.selectService = false
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !isTailing(m.service) {
				return m, tea.Quit
			}
		case "r":
			m.selectRegion = true
			m.regionInput.SetValue(m.runtime.Region)
			return m, m.regionInput.Focus()
		case "s":
			m.selectService = true
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
	if m.showHelp {
		return header + "\n" + helpView()
	}
	if m.selectRegion {
		return header + "\nChange region:\n" + m.regionInput.View()
	}
	if m.selectService {
		return header + "\nSelect service:\n- CloudWatch Logs (current)\nPress Enter to confirm or Esc to cancel."
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
