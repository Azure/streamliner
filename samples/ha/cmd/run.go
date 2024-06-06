/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	adxconnection "github.com/Azure/streamliner/samples/ha/pkg/adxConnection"
	config "github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/app"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/logger"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/metrics"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/publishers/eventgrid/publisher"
	adxCache "github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache/adx"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache/store/memory"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/monitor"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/translate"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "A brief description of your command",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// You can bind cobra and viper in a few locations, but PersistencePreRunE on the root command works well
		return initialize(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// build initial ADX connection to reuse by all clients.
		kcsb := adxconnection.NewConnectionStringbuilder(config.Get().Metrics.ADX.Endpoint)

		kcsb.AttachPolicyClientOptions(&azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries: 5,
				RetryDelay: 5 * time.Second,
				StatusCodes: []int{
					http.StatusRequestTimeout,      // 408
					http.StatusTooManyRequests,     //     429
					http.StatusInternalServerError, // 500
					http.StatusBadGateway,          //          502
					http.StatusServiceUnavailable,  //  503
					http.StatusGatewayTimeout,      //      504
				},
			},
		})

		c := &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    0,
				IdleConnTimeout: 0,
				Dial: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}

		client, err := kusto.New(kcsb, kusto.WithHttpClient(c))
		if err != nil {
			return fmt.Errorf("error creating kusto client, %w", err)
		}

		zerolog.DurationFieldUnit = time.Second

		log, err := logger.NewhaLog(
			logger.Config(config.Get()),
			logger.AdxClient(client),
		)

		if err != nil {
			return fmt.Errorf("error starting zerolog, %w", err)
		}

		// Create healthChannel
		healthCh := make(chan *healthstream.Message)

		metricsServer, err := metrics.New(
			metrics.Config(&config.Get().Metrics),
			metrics.Logger(&log),
			metrics.Name("metrics"),
			metrics.Environment(config.Get().Environment),
			metrics.Region(config.Get().Monitor.Region),
			metrics.Role(config.Get().Monitor.Role),
			metrics.AdxClient(client),
		)
		if err != nil {
			return fmt.Errorf("error starting metrics task, %w", err)
		}

		cacheTask, err := adxCache.New(
			adxCache.Config(&config.Get().Translate),
			adxCache.Logger(&log),
			adxCache.Name("cache"),
			adxCache.WithStore(memory.New(&log)),
			adxCache.HealthChannel(healthCh),
			adxCache.Metrics(metricsServer),
			adxCache.AdxLogClient(client),
			adxCache.Environment(config.Get().Environment),
			adxCache.Region(config.Get().Monitor.Region),
			adxCache.Role(config.Get().Monitor.Role),
			adxCache.Context(cmd.Context()),
		)
		if err != nil {
			return fmt.Errorf("error starting cache task, %w", err)
		}

		egPublisher, err := publisher.New(
			publisher.Config(&config.Get().EventGrid),
			publisher.Logger(&log),
			publisher.Name("eg-publisher"),
			publisher.HealthChannel(healthCh),
			publisher.Metrics(metricsServer),
		)

		if err != nil {
			return fmt.Errorf("error starting grid publisher, %w", err)
		}

		translator, err := translate.New(
			translate.Config(config.Get()),
			translate.Logger(&log),
			translate.Cache(cacheTask),
			translate.Next(egPublisher),
			translate.Name("translator"),
			translate.HealthChannel(healthCh),
			translate.Metrics(metricsServer),
			translate.AdxClient(client),
		)
		if err != nil {
			return fmt.Errorf("error starting translator, %w", err)
		}

		monitor, err := monitor.New(
			monitor.Config(&config.Get().Monitor),
			monitor.Logger(&log),
			monitor.ConnectionString(os.Getenv("AZURE_EVENTHUB_CONNECTIONSTRING")),
			monitor.Start(translator),
			monitor.Name("monitor"),
			monitor.HealthChannel(healthCh),
			monitor.Metrics(metricsServer),
		)
		if err != nil {
			return fmt.Errorf("error starting monitor, %w", err)
		}

		a := app.New(
			app.Name("ha"),
			app.Version("v0.0.2"),
			app.Logger(&log),
			app.Context(cmd.Context()),
			app.Signal(syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT),
			app.Task(monitor, cacheTask, translator, egPublisher, metricsServer),
		)

		appError := a.Run()

		if client != nil {
			client.Close()
		}
		return fmt.Errorf("error starting app, %w", appError)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().AddFlagSet(config.GetTranslateFlags())
	runCmd.Flags().AddFlagSet(config.GetMetricsFlags())
	runCmd.Flags().AddFlagSet(config.GetLoggingFlags())
	runCmd.Flags().AddFlagSet(config.GetEventGridFlags())
	runCmd.Flags().AddFlagSet(config.GetMonitorFlags())
	runCmd.Flags().AddFlagSet(config.GetAPIFlags())
}

func initialize(cmd *cobra.Command) error {
	config.BindFlags(cmd, viper.GetViper())
	return nil
}
