/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package config

import (
	"time"

	"github.com/spf13/pflag"
)

const DefaultAPIPort = 8888

func GetMonitorFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("monitor parameters", pflag.ExitOnError)

	flags.StringVar(&globalConf.Monitor.Role, "monitor.role", "", "role of the event mapper")
	flags.StringVar(&globalConf.Monitor.Region, "monitor.region", "", "region of the event mapper")

	flags.DurationVar(&globalConf.Monitor.Health.Interval, "monitor.health.interval", time.Minute, "Interval read/write checkpoints to the blob store")

	flags.DurationVar(&globalConf.Monitor.Health.PreemptionHoldTime, "monitor.health.preemptionholdtime", time.Minute*3, "Time to wait before backup start processing messages in the event of a failure")
	flags.StringVar(&globalConf.Monitor.Health.Checkpoint.URL, "monitor.health.checkpoint.url", "", "Azure Storage account url ")
	flags.StringVar(&globalConf.Monitor.Health.Checkpoint.ContainerName, "monitor.health.checkpoint.containername", "", "Azure Storage container name for health checkpoint")

	return flags
}

func GetTranslateFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("Translate parameters", pflag.ExitOnError)

	flags.StringVar(&globalConf.EventHub.ConsumerGroup, "eventhub.consumerGroup", "", "Name of the consumer group for the Azure Event Hub")
	flags.StringVar(&globalConf.EventHub.Name, "eventHub.name", "", "Name of the Azure Event Hub")
	flags.StringVar(&globalConf.EventHub.Namespace, "eventhub.namespace", "", "Name of the Azure Event Hub Namespace")
	flags.StringVar(&globalConf.EventHub.Checkpoint.URL, "eventhub.checkpoint.url", "", "Azure Storage account url ")
	flags.StringVar(&globalConf.EventHub.Checkpoint.ContainerName, "eventhub.checkpoint.containername", "", "Azure Storage container name for checkpoint")
	flags.StringVar(&globalConf.EventHub.Strategy, "eventhub.strategy", "balanced", "Azure EventHub processor strategy (balanced|greedy)")

	flags.BoolVar(&globalConf.Translate.ADX.Enabled, "translate.adx.enabled", true, "Enable translation via ADX")
	flags.StringVar(&globalConf.Translate.ADX.Endpoint, "translate.adx.endpoint", "", "Azure Data Explorer endpoint url")
	flags.StringVar(&globalConf.Translate.ADX.Database, "translate.adx.database", "", "Azure Data Explorer database to query")
	flags.DurationVar(&globalConf.Translate.ADX.UpdateFrequency, "translate.adx.updatefrequency", time.Minute, "Azure Data Explorer update frequency")

	flags.DurationVar(&globalConf.Translate.ADX.RetryOptions.Delay, "translate.adx.retry.delay", time.Second*3, "Delay between retries")
	flags.DurationVar(&globalConf.Translate.ADX.RetryOptions.MaxJitter, "translate.adx.retry.maxjitter", time.Second*7, "Max jitter between retries")
	flags.UintVar(&globalConf.Translate.ADX.RetryOptions.Attempts, "translate.adx.retry.attempts", 5, "number of attempts to retry the operation")

	flags.IntVar(&globalConf.Translate.Chaos.Failure.PerMessage, "translate.chaos.failure.permessage", 10, "when to fail per messaage")
	flags.BoolVar(&globalConf.Translate.Chaos.Enabled, "translate.chaos.enabled", false, "enable chaos engineering")

	flags.BoolVar(&globalConf.Translate.Logging.Enabled, "translate.logging.enabled", true, "Enable translation logging via ADX")
	flags.StringVar(&globalConf.Translate.Logging.Endpoint, "translate.logging.endpoint", "", "Azure Data Explorer logging endpoint url")
	flags.StringVar(&globalConf.Translate.Logging.Database, "translate.logging.database", "", "Azure Data Explorer database to send logs")
	flags.DurationVar(&globalConf.Translate.Logging.Timeout, "translate.logging.timeout", 5*time.Minute, "Azure Data Explorer timeout")

	flags.DurationVar(&globalConf.Translate.Logging.RetryOptions.Delay, "translate.logging.retry.delay", time.Second*3, "Delay between retries")
	flags.DurationVar(&globalConf.Translate.Logging.RetryOptions.MaxJitter, "translate.logging.retry.maxjitter", time.Second*7, "Max jitter between retries")
	flags.UintVar(&globalConf.Translate.Logging.RetryOptions.Attempts, "translate.logging.retry.attempts", 5, "number of attempts to retry the operation")

	flags.DurationVar(&globalConf.Translate.MessageExpiration, "translate.messageexpiration", 5*time.Minute, "Maxium time allow for a message to be in eventhub before expiration")

	return flags
}

func GetEventGridFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("eventgrid parameters", pflag.ExitOnError)

	flags.StringVar(&globalConf.EventGrid.Primary.Endpoint, "eventgrid.primary.endpoint", "", "Endpoint of the primary event grid")
	flags.StringVar(&globalConf.EventGrid.Primary.Topic, "eventgrid.primary.topic", "", "Topic of the primary event grid")

	flags.DurationVar(&globalConf.EventGrid.Primary.Chaos.Failure.Duration, "eventgrid.primary.chaos.failure.duration", time.Minute*5, "Duration of the failure")
	flags.DurationVar(&globalConf.EventGrid.Primary.Chaos.Failure.Every, "eventgrid.primary.chaos.failure.every", time.Minute*10, "frequency of the failures")
	flags.BoolVar(&globalConf.EventGrid.Primary.Chaos.Enabled, "eventgrid.primary.chaos.enabled", false, "enable chaos engineering")

	flags.DurationVar(&globalConf.EventGrid.Primary.RetryOptions.Delay, "eventgrid.primary.retry.delay", time.Second*3, "Delay between retries")
	flags.DurationVar(&globalConf.EventGrid.Primary.RetryOptions.MaxJitter, "eventgrid.primary.retry.maxjitter", time.Second*7, "Max jitter between retries")
	flags.UintVar(&globalConf.EventGrid.Primary.RetryOptions.Attempts, "eventgrid.primary.retry.attempts", 5, "number of attempts to retry the operation")

	flags.StringVar(&globalConf.EventGrid.Secondary.Endpoint, "eventgrid.secondary.endpoint", "", "Endpoint of the secondary event grid")
	flags.StringVar(&globalConf.EventGrid.Secondary.Topic, "eventgrid.secondary.topic", "", "Topic of the primary event grid")

	flags.DurationVar(&globalConf.EventGrid.Secondary.Chaos.Failure.Duration, "eventgrid.secondary.chaos.failure.duration", time.Minute*5, "Duration of the failure")
	flags.DurationVar(&globalConf.EventGrid.Secondary.Chaos.Failure.Every, "eventgrid.secondary.chaos.failure.every", time.Minute*10, "frequency of the failures")
	flags.BoolVar(&globalConf.EventGrid.Secondary.Chaos.Enabled, "eventgrid.secondary.chaos.enabled", false, "enable chaos engineering")

	flags.DurationVar(&globalConf.EventGrid.Secondary.RetryOptions.Delay, "eventgrid.secondary.retry.delay", time.Second*3, "Delay between retries")
	flags.DurationVar(&globalConf.EventGrid.Secondary.RetryOptions.MaxJitter, "eventgrid.secondary.retry.maxjitter", time.Second*7, "Max jitter between retries")
	flags.UintVar(&globalConf.EventGrid.Secondary.RetryOptions.Attempts, "eventgrid.secondary.retry.attempts", 5, "number of attempts to retry the operation")

	return flags
}

func GetLoggingFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("Logging parameters", pflag.ExitOnError)

	flags.BoolVar(&globalConf.Logging.ADX.Enabled, "adx.logging.adx.enabled", true, "Enable Azure DAta Explorer logging")
	flags.StringVar(&globalConf.Logging.ADX.Endpoint, "logging.adx.endpoint", "", "Azure Data Explorer endpoint url")
	flags.StringVar(&globalConf.Logging.ADX.Database, "logging.adx.database", "", "Azure Data Explorer database to export logs")
	flags.DurationVar(&globalConf.Logging.ADX.Timeout, "logging.adx.timeout", 5*time.Minute, "Azure Data Explorer context timeout")

	flags.DurationVar(&globalConf.Logging.ADX.RetryOptions.Delay, "logging.adx.retry.delay", time.Second*3, "Delay between retries")
	flags.DurationVar(&globalConf.Logging.ADX.RetryOptions.MaxJitter, "logging.adx.retry.maxjitter", time.Second*7, "Max jitter between retries")
	flags.UintVar(&globalConf.Logging.ADX.RetryOptions.Attempts, "logging.adx.retry.attempts", 5, "number of attempts to retry the operation")

	flags.BoolVar(&globalConf.Logging.Fluentd.Enabled, "logging.fluentd.enabled", true, "Enable fluentd logging")
	flags.StringVar(&globalConf.Logging.Fluentd.SocketPath, "logging.fluentd.socketpath", "", "Fluentd socket path")

	flags.BoolVar(&globalConf.Logging.Console.Pretty, "logging.console.pretty", true, "Enable pretty printing in console")

	return flags
}

func GetMetricsFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("Metrics parameters", pflag.ExitOnError)

	// ADX metrics
	flags.BoolVar(&globalConf.Metrics.ADX.Enabled, "metrics.adx.enabled", true, "Enable adx metrics")
	//
	flags.StringVar(&globalConf.Metrics.ADX.Endpoint, "metrics.adx.endpoint", "", "Azure Data Explorer endpoint url")
	flags.StringVar(&globalConf.Metrics.ADX.Database, "metrics.adx.database", "", "Azure Data Explorer database to export metrics")

	flags.DurationVar(&globalConf.Metrics.ADX.RetryOptions.Delay, "metrics.adx.retry.delay", time.Second*3, "Delay between retries")
	flags.DurationVar(&globalConf.Metrics.ADX.RetryOptions.MaxJitter, "metrics.adx.retry.maxjitter", time.Second*7, "Max jitter between retries")
	flags.UintVar(&globalConf.Metrics.ADX.RetryOptions.Attempts, "metrics.adx.retry.attempts", 5, "number of attempts to retry the operation")

	flags.DurationVar(&globalConf.Metrics.ADX.Timeout, "metrics.adx.timeout", time.Minute*5, "Context timeout when writing to ADX")

	// Flush Interval
	flags.DurationVar(&globalConf.Metrics.FlushInterval, "metrics.flushinterval", time.Minute*5, "Flush interval for metrics")

	return flags
}

func GetGenericFlags() *pflag.FlagSet {
	defaults := pflag.NewFlagSet("defaults for all commands", pflag.ExitOnError)
	defaults.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config.yaml)")
	defaults.BoolVar(&globalConf.Debug, "debug", false, "endable debug output")

	defaults.StringVar(&globalConf.Environment, "environment", "dev", "environment")

	return defaults
}

func GetAPIFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("API parameters", pflag.ExitOnError)

	flags.BoolVar(&globalConf.API.Enabled, "translate.api.enabled", true, "Enable api for translations")
	flags.IntVar(&globalConf.API.Port, "translate.api.port", DefaultAPIPort, "Port to API listen")
	flags.DurationVar(&globalConf.API.MemProfileFrequency, "translate.api.memprofilefrequency", time.Minute*5, "Frequency to write memory profile")

	return flags
}
