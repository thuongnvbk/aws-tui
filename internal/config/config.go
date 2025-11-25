package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Profiles       []Profile         `yaml:"profiles"`
	DefaultProfile string            `yaml:"default_profile"`
	Resources      ResourceConfig    `yaml:"resources"`
	UI             UIConfig          `yaml:"ui"`
	Keybindings    KeybindingsConfig `yaml:"keybindings"`
	Actions        ActionsConfig     `yaml:"actions"`
	Logging        LoggingConfig     `yaml:"logging"`
}

type Profile struct {
	Name        string `yaml:"name"`
	Region      string `yaml:"region"`
	Description string `yaml:"description"`
}

type ResourceConfig struct {
	EC2    EC2Config    `yaml:"ec2"`
	EKS    EKSConfig    `yaml:"eks"`
	ECS    ECSConfig    `yaml:"ecs"`
	S3     S3Config     `yaml:"s3"`
	RDS    RDSConfig    `yaml:"rds"`
	Lambda LambdaConfig `yaml:"lambda"`
}

type EC2Config struct {
	Enabled bool     `yaml:"enabled"`
	Filters []Filter `yaml:"filters"`
}

type Filter struct {
	Key    string   `yaml:"key"`
	Values []string `yaml:"values"`
}

type EKSConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Clusters []string `yaml:"clusters"`
}

type ECSConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Clusters []string `yaml:"clusters"`
}

type S3Config struct {
	Enabled  bool     `yaml:"enabled"`
	Prefixes []string `yaml:"prefixes"`
}

type RDSConfig struct {
	Enabled bool `yaml:"enabled"`
}

type LambdaConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Prefixes []string `yaml:"prefixes"`
}

type UIConfig struct {
	Theme           string `yaml:"theme"`
	RefreshInterval int    `yaml:"refresh_interval"`
	ShowCounts      bool   `yaml:"show_counts"`
	DateFormat      string `yaml:"date_format"`
}

type KeybindingsConfig struct {
	Quit          string `yaml:"quit"`
	Refresh       string `yaml:"refresh"`
	Help          string `yaml:"help"`
	Search        string `yaml:"search"`
	Filter        string `yaml:"filter"`
	SwitchProfile string `yaml:"switch_profile"`
	SwitchRegion  string `yaml:"switch_region"`
	ActionMenu    string `yaml:"action_menu"`
	Copy          string `yaml:"copy"`
	Logs          string `yaml:"logs"`
}

type ActionsConfig struct {
	EC2    []Action `yaml:"ec2"`
	EKS    []Action `yaml:"eks"`
	Lambda []Action `yaml:"lambda"`
}

type Action struct {
	Name     string `yaml:"name"`
	Command  string `yaml:"command"`
	Confirm  bool   `yaml:"confirm"`
	Template string `yaml:"template"`
}

type LoggingConfig struct {
	Enabled bool   `yaml:"enabled"`
	File    string `yaml:"file"`
	Level   string `yaml:"level"`
}

func DefaultConfig() *Config {
	return &Config{
		Profiles: []Profile{
			{Name: "default", Region: "us-east-1", Description: "Default Profile"},
		},
		DefaultProfile: "default",
		Resources: ResourceConfig{
			EC2:    EC2Config{Enabled: true},
			EKS:    EKSConfig{Enabled: true},
			ECS:    ECSConfig{Enabled: true},
			S3:     S3Config{Enabled: true},
			RDS:    RDSConfig{Enabled: true},
			Lambda: LambdaConfig{Enabled: true},
		},
		UI: UIConfig{
			Theme:           "dark",
			RefreshInterval: 30,
			ShowCounts:      true,
			DateFormat:      "2006-01-02 15:04:05",
		},
		Keybindings: KeybindingsConfig{
			Quit:          "q",
			Refresh:       "r",
			Help:          "?",
			Search:        "/",
			Filter:        "f",
			SwitchProfile: "p",
			SwitchRegion:  "R",
			ActionMenu:    "a",
			Copy:          "c",
			Logs:          "l",
		},
		Logging: LoggingConfig{
			Enabled: true,
			Level:   "info",
		},
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return DefaultConfig(), nil
		}
		path = filepath.Join(homeDir, ".aws-tui", "config.yaml")
	}

	path = expandPath(path)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[1:])
		}
	}
	return path
}

func (c *Config) GetProfile(name string) *Profile {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i]
		}
	}
	return nil
}

func (c *Config) GetDefaultProfile() *Profile {
	return c.GetProfile(c.DefaultProfile)
}
