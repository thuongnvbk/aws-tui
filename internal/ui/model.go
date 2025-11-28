package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aws-tui/internal/aws"
	"aws-tui/internal/config"

	"github.com/atotto/clipboard"
	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ResourceType int

const (
	ResourceEC2 ResourceType = iota
	ResourceEKS
	ResourceECS
	ResourceS3
	ResourceRDS
	ResourceLambda
	ResourceMSK
	resourceTypeCount
)

func (r ResourceType) String() string {
	switch r {
	case ResourceEC2:
		return "EC2"
	case ResourceEKS:
		return "EKS"
	case ResourceECS:
		return "ECS"
	case ResourceS3:
		return "S3"
	case ResourceRDS:
		return "RDS"
	case ResourceLambda:
		return "Lambda"
	case ResourceMSK:
		return "MSK"
	default:
		return "Unknown"
	}
}

type ViewMode int

const (
	ViewList ViewMode = iota
	ViewDetail
	ViewHelp
	ViewProfileSelect
	ViewRegionSelect
)

type Model struct {
	cfg          *config.Config
	client       *aws.Client
	width        int
	height       int
	refreshEvery time.Duration
	resourceType ResourceType
	viewMode     ViewMode
	list         list.Model
	detail       viewport.Model
	spinner      spinner.Model
	help         help.Model
	keys         keyMap
	loading      bool
	err          error
	statusMsg    string

	// Data
	ec2Instances    []aws.EC2Instance
	eksClusters     []aws.EKSCluster
	ecsClusters     []aws.ECSCluster
	s3Buckets       []aws.S3Bucket
	rdsInstances    []aws.RDSInstance
	lambdaFunctions []aws.LambdaFunction
	mskClusters     []aws.MSKCluster

	// Selection
	selectedIndex int
	profiles      []string
	regions       []string

	// MSK Topics
	mskUsername string
	mskPassword string
	inputField  int // 0 = username, 1 = password
}

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding
	Refresh  key.Binding
	Quit     key.Binding
	Help     key.Binding
	Profile  key.Binding
	Region   key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Copy     key.Binding
	Topics   key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:  key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev type")),
		Right: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next type")),
		Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/detail")),
		// Add an explicit "b" shortcut so users have an obvious back button in addition to Esc.
		Back:     key.NewBinding(key.WithKeys("esc", "b"), key.WithHelp("esc/b", "back")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Profile:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "switch profile")),
		Region:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "switch region")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next resource")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("S-tab", "prev resource")),
		Copy:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy ID")),
		Topics:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "fetch topics")),
	}
}

func buildKeyMap(cfg config.KeybindingsConfig) keyMap {
	km := defaultKeyMap()

	if cfg.Quit != "" {
		km.Quit = key.NewBinding(key.WithKeys(cfg.Quit), key.WithHelp(cfg.Quit, "quit"))
	}
	if cfg.Refresh != "" {
		km.Refresh = key.NewBinding(key.WithKeys(cfg.Refresh), key.WithHelp(cfg.Refresh, "refresh"))
	}
	if cfg.Help != "" {
		km.Help = key.NewBinding(key.WithKeys(cfg.Help), key.WithHelp(cfg.Help, "help"))
	}
	if cfg.SwitchProfile != "" {
		km.Profile = key.NewBinding(key.WithKeys(cfg.SwitchProfile), key.WithHelp(cfg.SwitchProfile, "switch profile"))
	}
	if cfg.SwitchRegion != "" {
		km.Region = key.NewBinding(key.WithKeys(cfg.SwitchRegion), key.WithHelp(cfg.SwitchRegion, "switch region"))
	}
	if cfg.Copy != "" {
		km.Copy = key.NewBinding(key.WithKeys(cfg.Copy), key.WithHelp(cfg.Copy, "copy ID"))
	}

	return km
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Refresh, k.Profile, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Back, k.Tab, k.ShiftTab},
		{k.Refresh, k.Profile, k.Region},
		{k.Copy},
		{k.Help, k.Quit},
	}
}

type listItem struct {
	title    string
	desc     string
	status   string
	resource interface{}
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

type fetchCompleteMsg struct {
	resourceType ResourceType
	err          error
	ec2          []aws.EC2Instance
	eks          []aws.EKSCluster
	ecs          []aws.ECSCluster
	s3           []aws.S3Bucket
	rds          []aws.RDSInstance
	lambda       []aws.LambdaFunction
	msk          []aws.MSKCluster
}

type actionCompleteMsg struct {
	action string
	err    error
}

type topicsFetchCompleteMsg struct {
	topics []aws.KafkaTopic
	err    error
}

type refreshTickMsg struct{}

func NewModel(cfg *config.Config, client *aws.Client) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	h := help.New()

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("170"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("240"))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "EC2 Instances"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	regionSet := map[string]struct{}{}
	if client.Region != "" {
		regionSet[client.Region] = struct{}{}
	}

	var profiles []string
	for _, p := range cfg.Profiles {
		profiles = append(profiles, p.Name)
		if p.Region != "" {
			regionSet[p.Region] = struct{}{}
		}
	}

	// fallback core regions if nothing collected
	if len(regionSet) == 0 {
		for _, r := range []string{
			"us-east-1", "us-east-2", "us-west-1", "us-west-2",
			"eu-west-1", "eu-west-2", "eu-central-1",
			"ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
		} {
			regionSet[r] = struct{}{}
		}
	}

	var regions []string
	for r := range regionSet {
		regions = append(regions, r)
	}

	refreshEvery := time.Duration(cfg.UI.RefreshInterval) * time.Second
	if cfg.UI.RefreshInterval <= 0 {
		refreshEvery = 0
	}

	return Model{
		cfg:          cfg,
		client:       client,
		refreshEvery: refreshEvery,
		resourceType: ResourceEC2,
		viewMode:     ViewList,
		list:         l,
		spinner:      s,
		help:         h,
		keys:         buildKeyMap(cfg.Keybindings),
		loading:      true,
		profiles:     profiles,
		regions:      regions,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		m.fetchResources(),
	}
	if m.refreshEvery > 0 {
		cmds = append(cmds, m.scheduleRefreshTick())
	}
	return tea.Batch(cmds...)
}

func (m Model) fetchResources() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		msg := fetchCompleteMsg{resourceType: m.resourceType}

		switch m.resourceType {
		case ResourceEC2:
			if !m.cfg.Resources.EC2.Enabled {
				return msg
			}
			filters := buildEC2Filters(m.cfg.Resources.EC2.Filters)
			msg.ec2, msg.err = m.client.ListEC2Instances(ctx, filters)
		case ResourceEKS:
			if !m.cfg.Resources.EKS.Enabled {
				return msg
			}
			msg.eks, msg.err = m.client.ListEKSClusters(ctx)
			if len(m.cfg.Resources.EKS.Clusters) > 0 {
				msg.eks = filterEKSClusters(msg.eks, m.cfg.Resources.EKS.Clusters)
			}
		case ResourceECS:
			if !m.cfg.Resources.ECS.Enabled {
				return msg
			}
			msg.ecs, msg.err = m.client.ListECSClusters(ctx)
			if len(m.cfg.Resources.ECS.Clusters) > 0 {
				msg.ecs = filterECSClusters(msg.ecs, m.cfg.Resources.ECS.Clusters)
			}
		case ResourceS3:
			if !m.cfg.Resources.S3.Enabled {
				return msg
			}
			msg.s3, msg.err = m.client.ListS3Buckets(ctx)
			if len(m.cfg.Resources.S3.Prefixes) > 0 {
				msg.s3 = filterS3Buckets(msg.s3, m.cfg.Resources.S3.Prefixes)
			}
		case ResourceRDS:
			if !m.cfg.Resources.RDS.Enabled {
				return msg
			}
			msg.rds, msg.err = m.client.ListRDSInstances(ctx)
		case ResourceLambda:
			if !m.cfg.Resources.Lambda.Enabled {
				return msg
			}
			msg.lambda, msg.err = m.client.ListLambdaFunctions(ctx)
			if len(m.cfg.Resources.Lambda.Prefixes) > 0 {
				msg.lambda = filterLambdaFunctions(msg.lambda, m.cfg.Resources.Lambda.Prefixes)
			}
		case ResourceMSK:
			if !m.cfg.Resources.MSK.Enabled {
				return msg
			}
			msg.msk, msg.err = m.client.ListMSKClusters(ctx)
			if len(m.cfg.Resources.MSK.Prefixes) > 0 {
				msg.msk = filterMSKClusters(msg.msk, m.cfg.Resources.MSK.Prefixes)
			}
		}

		return msg
	}
}

func (m Model) scheduleRefreshTick() tea.Cmd {
	if m.refreshEvery == 0 {
		return nil
	}
	return tea.Tick(m.refreshEvery, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) fetchTopics(cluster *aws.MSKCluster) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		username := m.mskUsername
		password := m.mskPassword

		// If no credentials provided and cluster has secret ARNs, try to fetch from Secrets Manager
		if username == "" && password == "" && len(cluster.SecretArns) > 0 {
			// Try the first secret ARN
			creds, err := m.client.GetMSKCredentials(ctx, cluster.SecretArns[0])
			if err == nil && creds != nil {
				username = creds.Username
				password = creds.Password
			}
			// If fetching fails, we'll try to connect without credentials (for unauthenticated clusters)
		}

		topics, err := m.client.ListTopics(ctx, cluster, username, password)
		return topicsFetchCompleteMsg{
			topics: topics,
			err:    err,
		}
	}
}

func buildEC2Filters(cfgFilters []config.Filter) []types.Filter {
	var filters []types.Filter
	for _, f := range cfgFilters {
		if f.Key == "" || len(f.Values) == 0 {
			continue
		}
		filters = append(filters, types.Filter{
			Name:   sdkaws.String(fmt.Sprintf("tag:%s", f.Key)),
			Values: f.Values,
		})
	}
	return filters
}

func filterEKSClusters(clusters []aws.EKSCluster, names []string) []aws.EKSCluster {
	allow := map[string]struct{}{}
	for _, n := range names {
		allow[n] = struct{}{}
	}
	var filtered []aws.EKSCluster
	for _, c := range clusters {
		if _, ok := allow[c.Name]; ok {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func filterECSClusters(clusters []aws.ECSCluster, names []string) []aws.ECSCluster {
	allow := map[string]struct{}{}
	for _, n := range names {
		allow[n] = struct{}{}
	}
	var filtered []aws.ECSCluster
	for _, c := range clusters {
		if _, ok := allow[c.Name]; ok {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func filterS3Buckets(buckets []aws.S3Bucket, prefixes []string) []aws.S3Bucket {
	var filtered []aws.S3Bucket
	for _, b := range buckets {
		if hasPrefix(b.Name, prefixes) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func filterLambdaFunctions(funcs []aws.LambdaFunction, prefixes []string) []aws.LambdaFunction {
	var filtered []aws.LambdaFunction
	for _, fn := range funcs {
		if hasPrefix(fn.Name, prefixes) {
			filtered = append(filtered, fn)
		}
	}
	return filtered
}

func filterMSKClusters(clusters []aws.MSKCluster, prefixes []string) []aws.MSKCluster {
	var filtered []aws.MSKCluster
	for _, c := range clusters {
		if hasPrefix(c.Name, prefixes) {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func hasPrefix(name string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func selectedResourceID(item listItem) string {
	switch res := item.resource.(type) {
	case aws.EC2Instance:
		return res.ID
	case aws.EKSCluster:
		return res.Name
	case aws.ECSCluster:
		return res.Arn
	case aws.S3Bucket:
		return res.Name
	case aws.RDSInstance:
		return res.Identifier
	case aws.LambdaFunction:
		return res.Name
	case aws.MSKCluster:
		return res.Arn
	default:
		return ""
	}
}

func (m *Model) updateListItems() {
	var items []list.Item

	switch m.resourceType {
	case ResourceEC2:
		for _, inst := range m.ec2Instances {
			items = append(items, listItem{
				title:    inst.Name,
				desc:     fmt.Sprintf("%s | %s | %s", inst.ID, inst.Type, inst.State),
				status:   inst.State,
				resource: inst,
			})
		}
	case ResourceEKS:
		for _, cluster := range m.eksClusters {
			items = append(items, listItem{
				title:    cluster.Name,
				desc:     fmt.Sprintf("%s | v%s | %d node groups", cluster.Status, cluster.Version, len(cluster.NodeGroups)),
				status:   cluster.Status,
				resource: cluster,
			})
		}
	case ResourceECS:
		for _, cluster := range m.ecsClusters {
			items = append(items, listItem{
				title:    cluster.Name,
				desc:     fmt.Sprintf("%s | %d tasks | %d services", cluster.Status, cluster.RunningTasksCount, cluster.ActiveServicesCount),
				status:   cluster.Status,
				resource: cluster,
			})
		}
	case ResourceS3:
		for _, bucket := range m.s3Buckets {
			items = append(items, listItem{
				title:    bucket.Name,
				desc:     fmt.Sprintf("Created: %s", bucket.CreationDate),
				resource: bucket,
			})
		}
	case ResourceRDS:
		for _, inst := range m.rdsInstances {
			items = append(items, listItem{
				title:    inst.Identifier,
				desc:     fmt.Sprintf("%s | %s | %s", inst.Engine, inst.InstanceClass, inst.Status),
				status:   inst.Status,
				resource: inst,
			})
		}
	case ResourceLambda:
		for _, fn := range m.lambdaFunctions {
			items = append(items, listItem{
				title:    fn.Name,
				desc:     fmt.Sprintf("%s | %dMB | %s", fn.Runtime, fn.MemorySize, fn.State),
				status:   fn.State,
				resource: fn,
			})
		}
	case ResourceMSK:
		for _, cluster := range m.mskClusters {
			items = append(items, listItem{
				title:    cluster.Name,
				desc:     fmt.Sprintf("%s | v%s | %d brokers", cluster.State, cluster.KafkaVersion, cluster.BrokerNodes),
				status:   cluster.State,
				resource: cluster,
			})
		}
	}

	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("%s (%d)", m.resourceType.String(), len(items))
}

func (m *Model) showDetail() {
	if m.list.SelectedItem() == nil {
		return
	}

	item := m.list.SelectedItem().(listItem)
	var lines []string

	switch res := item.resource.(type) {
	case aws.EC2Instance:
		lines = res.DetailLines()
	case aws.EKSCluster:
		lines = res.DetailLines()
	case aws.ECSCluster:
		lines = res.DetailLines()
	case aws.S3Bucket:
		lines = res.DetailLines()
	case aws.RDSInstance:
		lines = res.DetailLines()
	case aws.LambdaFunction:
		lines = res.DetailLines()
	case aws.MSKCluster:
		lines = res.DetailLines()
	}

	content := strings.Join(lines, "\n")
	m.detail = viewport.New(m.width-4, m.height-8)
	m.detail.SetContent(content)
	m.viewMode = ViewDetail
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-8)
		m.help.Width = msg.Width
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case fetchCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.err = nil
			switch msg.resourceType {
			case ResourceEC2:
				m.ec2Instances = msg.ec2
			case ResourceEKS:
				m.eksClusters = msg.eks
			case ResourceECS:
				m.ecsClusters = msg.ecs
			case ResourceS3:
				m.s3Buckets = msg.s3
			case ResourceRDS:
				m.rdsInstances = msg.rds
			case ResourceLambda:
				m.lambdaFunctions = msg.lambda
			case ResourceMSK:
				m.mskClusters = msg.msk
			}
			m.updateListItems()
			m.statusMsg = fmt.Sprintf("Loaded %s", m.resourceType.String())
		}

	case actionCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Action failed: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Action '%s' completed", msg.action)
			cmds = append(cmds, m.fetchResources())
		}

	case topicsFetchCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to fetch topics: %v", msg.err)
		} else {
			// Update the current MSK cluster with topics
			if m.resourceType == ResourceMSK && m.list.SelectedItem() != nil {
				item := m.list.SelectedItem().(listItem)
				if cluster, ok := item.resource.(aws.MSKCluster); ok {
					cluster.Topics = msg.topics
					// Create a new slice to avoid race conditions
					newClusters := make([]aws.MSKCluster, len(m.mskClusters))
					copy(newClusters, m.mskClusters)
					for i, c := range newClusters {
						if c.Arn == cluster.Arn {
							newClusters[i] = cluster
							break
						}
					}
					m.mskClusters = newClusters
					// Refresh the detail view
					m.showDetail()
					m.statusMsg = fmt.Sprintf("Loaded %d topics", len(msg.topics))
				}
			}
		}

	case refreshTickMsg:
		if !m.loading {
			m.loading = true
			cmds = append(cmds, m.fetchResources(), m.spinner.Tick)
		}
		if m.refreshEvery > 0 {
			cmds = append(cmds, m.scheduleRefreshTick())
		}

	case tea.KeyMsg:
		if m.viewMode == ViewHelp {
			if key.Matches(msg, m.keys.Help) || key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Quit) {
				m.viewMode = ViewList
			}
			return m, nil
		}

		if m.viewMode == ViewDetail {
			switch {
			case key.Matches(msg, m.keys.Back):
				m.viewMode = ViewList
			case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down):
				var cmd tea.Cmd
				m.detail, cmd = m.detail.Update(msg)
				cmds = append(cmds, cmd)
			case key.Matches(msg, m.keys.Topics):
				// Only handle topics for MSK resources
				if m.resourceType == ResourceMSK && m.list.SelectedItem() != nil {
					item := m.list.SelectedItem().(listItem)
					if cluster, ok := item.resource.(aws.MSKCluster); ok {
						m.loading = true
						m.statusMsg = "Fetching topics..."
						cmds = append(cmds, m.fetchTopics(&cluster), m.spinner.Tick)
					}
				}
			}
			return m, tea.Batch(cmds...)
		}

		if m.viewMode == ViewProfileSelect {
			if len(m.profiles) == 0 {
				m.viewMode = ViewList
				m.statusMsg = "No profiles configured"
				return m, nil
			}
			switch {
			case key.Matches(msg, m.keys.Back):
				m.viewMode = ViewList
			case key.Matches(msg, m.keys.Up):
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case key.Matches(msg, m.keys.Down):
				if m.selectedIndex < len(m.profiles)-1 {
					m.selectedIndex++
				}
			case key.Matches(msg, m.keys.Enter):
				if m.selectedIndex >= len(m.profiles) {
					return m, nil
				}
				profile := m.cfg.GetProfile(m.profiles[m.selectedIndex])
				if profile != nil {
					m.loading = true
					m.viewMode = ViewList
					m.err = nil
					profileName := profile.Name
					profileRegion := profile.Region
					resourceType := m.resourceType
					return m, func() tea.Msg {
						ctx := context.Background()
						if err := m.client.SwitchProfile(ctx, profileName, profileRegion); err != nil {
							return fetchCompleteMsg{resourceType: resourceType, err: err}
						}
						return m.fetchResources()()
					}
				}
			}
			return m, nil
		}

		if m.viewMode == ViewRegionSelect {
			if len(m.regions) == 0 {
				m.viewMode = ViewList
				m.statusMsg = "No regions configured"
				return m, nil
			}
			switch {
			case key.Matches(msg, m.keys.Back):
				m.viewMode = ViewList
			case key.Matches(msg, m.keys.Up):
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case key.Matches(msg, m.keys.Down):
				if m.selectedIndex < len(m.regions)-1 {
					m.selectedIndex++
				}
			case key.Matches(msg, m.keys.Enter):
				if m.selectedIndex >= len(m.regions) {
					return m, nil
				}
				region := m.regions[m.selectedIndex]
				m.loading = true
				m.viewMode = ViewList
				m.err = nil
				resourceType := m.resourceType
				return m, func() tea.Msg {
					ctx := context.Background()
					if err := m.client.SwitchRegion(ctx, region); err != nil {
						return fetchCompleteMsg{resourceType: resourceType, err: err}
					}
					return m.fetchResources()()
				}
			}
			return m, nil
		}

		// Main list view
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.viewMode = ViewHelp
		case key.Matches(msg, m.keys.Profile):
			m.viewMode = ViewProfileSelect
			m.selectedIndex = 0
		case key.Matches(msg, m.keys.Region):
			m.viewMode = ViewRegionSelect
			m.selectedIndex = 0
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			cmds = append(cmds, m.fetchResources(), m.spinner.Tick)
		case key.Matches(msg, m.keys.Enter):
			m.showDetail()
		case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Right):
			m.resourceType = (m.resourceType + 1) % resourceTypeCount
			m.loading = true
			m.err = nil
			cmds = append(cmds, m.fetchResources(), m.spinner.Tick)
		case key.Matches(msg, m.keys.ShiftTab), key.Matches(msg, m.keys.Left):
			if m.resourceType == 0 {
				m.resourceType = resourceTypeCount - 1
			} else {
				m.resourceType--
			}
			m.loading = true
			m.err = nil
			cmds = append(cmds, m.fetchResources(), m.spinner.Tick)
		case key.Matches(msg, m.keys.Copy):
			if m.list.SelectedItem() != nil {
				if id := selectedResourceID(m.list.SelectedItem().(listItem)); id != "" {
					if err := clipboard.WriteAll(id); err != nil {
						m.statusMsg = fmt.Sprintf("Copy failed: %v", err)
					} else {
						m.statusMsg = fmt.Sprintf("Copied: %s", id)
					}
				}
			}
		default:
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}
