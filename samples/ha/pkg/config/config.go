/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	EnvPrefix        = "TEMPO"
	StrategyBalanced = "balanced"
	StreategyGreedy  = "greedy"

	PRIMARY_ROLE   = "Primary"
	SECONDARY_ROLE = "Secondary"
)

var (
	cfgFile    string
	globalConf Bootstrap
)

type Bootstrap struct {
	Environment string    `yaml:"environment"`
	EventHub    EventHub  `yaml:"eventhub" json:"eventhub"`
	Translate   Translate `yaml:"translate" json:"translate"`
	Logging     Logging   `yaml:"logging" json:"logging"`
	Debug       bool      `yaml:"debug" json:"debug"`
	Metrics     Metrics   `yaml:"metrics" json:"metrics"`
	EventGrid   EventGrid `yaml:"eventgrid" json:"eventgrid"`
	Monitor     Monitor   `yaml:"monitor" json:"monitor"`
	API         API       `yaml:"api" json:"api"`
}
type API struct {
	Path                string        `yaml:"path" json:"path"`
	Port                int           `yaml:"port" json:"port"`
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	MemProfileFrequency time.Duration `yaml:"memprofilefrequency" json:"memprofilefrequency"`
}

type Monitor struct {
	Role   string `yaml:"role" json:"role"`
	Region string `yaml:"region" json:"region"`
	Health Health `yaml:"health" json:"health"`
}

type Health struct {
	Interval           time.Duration `yaml:"interval" json:"interval"`
	PreemptionHoldTime time.Duration `yaml:"preemptionholdtime" json:"preemptionholdtime"`
	Checkpoint         Checkpoint    `yaml:"checkpoint" json:"checkpoint"`
}

type EventGrid struct {
	Primary   EventGridOptions `yaml:"primary" json:"primary"`
	Secondary EventGridOptions `yaml:"secondary" json:"secondary"`
}

type EventGridOptions struct {
	Endpoint     string       `yaml:"endpoint" json:"endpoint"`
	Topic        string       `yaml:"topic" json:"topic"`
	Chaos        Chaos        `yaml:"chaos" json:"chaos"`
	Key          string       `yaml:"key" json:"key"`
	RetryOptions RetryOptions `yaml:"retry" json:"retry"`
}

type Customer struct {
	Subscriptions []string `yaml:"Subscriptions" json:"Subscriptions"`
}

type Metrics struct {
	ADX           ADXLogging    `yaml:"adx" json:"adx"`
	FlushInterval time.Duration `yaml:"flushinterval" json:"flushinterval"`
}

type InMem struct {
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	Interval time.Duration `yaml:"interval" json:"interval"`
	Retain   time.Duration `yaml:"retain" json:"retain"`
}

type Prometheus struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Port    int    `yaml:"port" json:"port"`
	Path    string `yaml:"path" json:"path"`
}

type Statsd struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Service string `yaml:"service" json:"service"`
}

type Logging struct {
	Console ConsoleLogging `yaml:"console" json:"console"`
	ADX     ADXLogging     `yaml:"adx" json:"adx"`
	Fluentd FluentdLogging `yaml:"fluentd" json:"fluentd"`
}

type ConsoleLogging struct {
	Pretty bool `yaml:"pretty" json:"pretty"`
}

type FluentdLogging struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	SocketPath string `yaml:"socketpath" json:"socketpath"`
}

type ADXLogging struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	Endpoint     string        `yaml:"endpoint" json:"endpoint"`
	Database     string        `yaml:"database" json:"database"`
	RetryOptions RetryOptions  `yaml:"retry" json:"retry"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
}

type Translate struct {
	MessageExpiration time.Duration `yaml:"messageexpiration" json:"messageexpiration"`
	ADX               ADXTranslate  `yaml:"adx" json:"adx"`
	Chaos             Chaos         `yaml:"caos" json:"caos"`
	Logging           ADXLogging    `yaml:"logging" json:"logging"`
}

type Failure struct {
	Duration   time.Duration `yaml:"duration" json:"duration"`
	Every      time.Duration `yaml:"every" json:"every"`
	PerMessage int           `yaml:"permessage" json:"permessage"`
}

type Chaos struct {
	Enabled bool    `yaml:"enabled" json:"enabled"`
	Failure Failure `yaml:"failure" json:"failure"`
}

type ADXTranslate struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	Endpoint        string        `yaml:"endpoint" json:"endpoint"`
	Database        string        `yaml:"database" json:"database"`
	UpdateFrequency time.Duration `yaml:"updatefrequency" json:"updatefrequency"`
	RetryOptions    RetryOptions  `yaml:"retry" json:"retry"`
}

type EventHub struct {
	Namespace     string     `yaml:"eventHubNamespace" json:"eventHubNamespace"`
	Name          string     `yaml:"eventHubName" json:"eventHubName"`
	ConsumerGroup string     `yaml:"consumerGroup" json:"consumerGroup"`
	Checkpoint    Checkpoint `yaml:"checkpoint" json:"checkpoint"`
	Strategy      string     `yaml:"strategy" json:"strategy"`
}

type Checkpoint struct {
	URL           string `yaml:"url" json:"url"`
	ContainerName string `yaml:"containername" json:"containername"`
}

type RetryOptions struct {
	Delay     time.Duration `yaml:"delay" json:"delay"`
	MaxJitter time.Duration `yaml:"maxjitter" json:"maxjitter" `
	Attempts  uint          `yaml:"attempts" json:"attempts"`
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func BindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			_ = v.BindEnv(f.Name, fmt.Sprintf("%s_%s", EnvPrefix, envVarSuffix))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func Get() *Bootstrap {
	return &globalConf
}

func GetConfigFileName() string {
	return cfgFile
}

func init() {
	globalConf = Bootstrap{}
}
