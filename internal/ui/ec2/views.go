package ec2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/ec2"
)

var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("213")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	dimText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("44"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)

	stateRunning    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // green
	stateStopped    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	statePending    = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // yellow
	stateTerminated = lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // gray
)

func stateStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return stateRunning
	case "stopped":
		return stateStopped
	case "pending", "stopping", "shutting-down":
		return statePending
	case "terminated":
		return stateTerminated
	default:
		return dimText
	}
}

func (m Model) renderLeft() string {
	b := &strings.Builder{}

	fmt.Fprintln(b, titleStyle.Render("EC2 Instances"))

	if m.loading {
		fmt.Fprintln(b, dimText.Render("Loading..."))
		return b.String()
	}

	// Search input
	if m.searching {
		fmt.Fprintln(b, m.search.View())
	} else if m.search.Value() != "" {
		fmt.Fprintln(b, statusStyle.Render(fmt.Sprintf("Filter: %s", m.search.Value())))
	} else {
		fmt.Fprintln(b, dimText.Render("Press / to filter"))
	}

	visibleHeight := m.listHeight()
	m.renderInstanceList(b, visibleHeight)

	// Status footer
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Total: %d instances", len(m.instances))))

	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}

	return b.String()
}

func (m Model) renderInstanceList(b *strings.Builder, visibleHeight int) {
	instances := m.filteredInstances()
	if len(instances) == 0 {
		fmt.Fprintln(b, dimText.Render("No instances found"))
		return
	}

	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.listOffset+visibleHeight, len(instances))

	for i := m.listOffset; i < endIdx; i++ {
		inst := instances[i]
		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}

		state := inst.State
		line := fmt.Sprintf("  %s  %s  %s", name, state, inst.InstanceType)

		maxWidth := m.width*2/5 - 6
		truncated := false
		if maxWidth > 0 && len(line) > maxWidth {
			line = line[:maxWidth-3] + "..."
			truncated = true
		}

		if i == m.cursor {
			line = cursorStyle.Render(line)
		} else if !truncated {
			styledState := stateStyle(state).Render(state)
			line = fmt.Sprintf("  %s  %s  %s", name, styledState, dimText.Render(inst.InstanceType))
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(instances) {
		if m.loadingMore {
			fmt.Fprintln(b, dimText.Render("  ⟳ loading more..."))
		} else {
			fmt.Fprintln(b, dimText.Render("  ↓ more below"))
		}
	}
}

func (m Model) renderRight() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Instance Details"))

	instances := m.filteredInstances()
	if len(instances) == 0 || m.cursor >= len(instances) {
		fmt.Fprintln(b, dimText.Render("No instance selected"))
		return b.String()
	}

	inst := instances[m.cursor]

	fmt.Fprintln(b)
	if inst.Name != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Name:"), inst.Name)
	}
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("ID:"), inst.InstanceID)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("State:"), stateStyle(inst.State).Render(inst.State))
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Type:"), inst.InstanceType)

	if inst.PrivateIP != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Private IP:"), inst.PrivateIP)
	}
	if inst.PublicIP != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Public IP:"), inst.PublicIP)
	}
	if inst.AvailabilityZone != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("AZ:"), inst.AvailabilityZone)
	}
	if inst.VpcID != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("VPC:"), inst.VpcID)
	}
	if inst.SubnetID != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Subnet:"), inst.SubnetID)
	}
	if inst.Architecture != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Arch:"), inst.Architecture)
	}
	if inst.Platform != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Platform:"), inst.Platform)
	}
	if inst.ImageID != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("AMI:"), inst.ImageID)
	}
	if inst.KeyName != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Key:"), inst.KeyName)
	}
	if !inst.LaunchTime.IsZero() {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Launched:"), inst.LaunchTime.Format("2006-01-02 15:04:05"))
	}

	if len(inst.SecurityGroups) > 0 {
		fmt.Fprintln(b)
		fmt.Fprintln(b, titleStyle.Render("Security Groups"))
		for _, sg := range inst.SecurityGroups {
			fmt.Fprintf(b, "  %s (%s)\n", sg.Name, sg.ID)
		}
	}

	if inst.IAMProfile != "" {
		fmt.Fprintln(b)
		fmt.Fprintln(b, dimText.Render("IAM Profile"))
		fmt.Fprintf(b, "  %s\n", inst.IAMProfile)
	}

	return b.String()
}

func (m Model) renderExpandedInstance() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Instance Details"))

	instances := m.filteredInstances()
	if m.expandedInst < len(instances) {
		inst := instances[m.expandedInst]
		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}
		fmt.Fprintf(b, "%s %s\n", dimText.Render("Instance:"), name)
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, m.expandedView.View())
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("↑↓/j/k scroll, pgup/pgdn page, esc close"))

	maxWidth := m.width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	return popupStyle.Width(maxWidth).Render(b.String())
}

// initExpandedInstanceView creates a viewport for displaying expanded instance details.
func initExpandedInstanceView(inst ec2.Instance, width, height int) viewport.Model {
	maxWidth := width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	viewportHeight := height - 14
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	var b strings.Builder

	if inst.Name != "" {
		fmt.Fprintf(&b, "Name: %s\n", inst.Name)
	}
	fmt.Fprintf(&b, "Instance ID: %s\n", inst.InstanceID)
	fmt.Fprintf(&b, "State: %s\n", inst.State)
	fmt.Fprintf(&b, "Type: %s\n", inst.InstanceType)

	if inst.PrivateIP != "" {
		fmt.Fprintf(&b, "Private IP: %s\n", inst.PrivateIP)
	}
	if inst.PublicIP != "" {
		fmt.Fprintf(&b, "Public IP: %s\n", inst.PublicIP)
	}
	if inst.AvailabilityZone != "" {
		fmt.Fprintf(&b, "Availability Zone: %s\n", inst.AvailabilityZone)
	}
	if inst.VpcID != "" {
		fmt.Fprintf(&b, "VPC: %s\n", inst.VpcID)
	}
	if inst.SubnetID != "" {
		fmt.Fprintf(&b, "Subnet: %s\n", inst.SubnetID)
	}
	if inst.Architecture != "" {
		fmt.Fprintf(&b, "Architecture: %s\n", inst.Architecture)
	}
	if inst.Platform != "" {
		fmt.Fprintf(&b, "Platform: %s\n", inst.Platform)
	}
	if inst.ImageID != "" {
		fmt.Fprintf(&b, "AMI: %s\n", inst.ImageID)
	}
	if inst.KeyName != "" {
		fmt.Fprintf(&b, "Key Pair: %s\n", inst.KeyName)
	}
	if !inst.LaunchTime.IsZero() {
		fmt.Fprintf(&b, "Launch Time: %s\n", inst.LaunchTime.Format("2006-01-02 15:04:05"))
	}
	if inst.IAMProfile != "" {
		fmt.Fprintf(&b, "IAM Profile: %s\n", inst.IAMProfile)
	}

	if len(inst.SecurityGroups) > 0 {
		fmt.Fprintln(&b, "\nSecurity Groups:")
		for _, sg := range inst.SecurityGroups {
			fmt.Fprintf(&b, "  %s (%s)\n", sg.Name, sg.ID)
		}
	}

	if len(inst.Tags) > 0 {
		fmt.Fprintln(&b, "\nTags:")
		keys := make([]string, 0, len(inst.Tags))
		for k := range inst.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s=%s\n", k, inst.Tags[k])
		}
	}

	vp := viewport.New(maxWidth-4, viewportHeight)
	vp.SetContent(b.String())
	return vp
}
