/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package logger

import (
	"io"
	"os"
	"time"

	"github.com/Azure/streamliner/samples/ha/pkg/config"
	adx "github.com/Azure/streamliner/samples/ha/pkg/v1/logger/adxIngest"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/rs/zerolog"
)

var (
	haLogsTablename = "logs_ha_v2"
)

type haLogs struct {
	Level     string
	Timestamp time.Time `json:"timestamp" kusto:"type:datetime"`
	Host      string
	Env       string
	Region    string
	Role      string
	Details   string
	Message   string
	Caller    string
	Task      string
	Error     string
}

type haLogger struct {
	bootstrap *config.Bootstrap
	adxClient *kusto.Client
}

type LogOption func(o *haLogger)

func Config(config *config.Bootstrap) LogOption {
	return func(l *haLogger) {
		l.bootstrap = config
	}
}

func AdxClient(client *kusto.Client) LogOption {
	return func(l *haLogger) { l.adxClient = client }
}

func NewhaLog(opts ...LogOption) (zerolog.Logger, error) {
	var multi zerolog.LevelWriter

	zerolog.TimestampFieldName = "timestamp"
	writers := []io.Writer{}
	log := &haLogger{}

	for _, o := range opts {
		o(log)
	}

	var consoleWriter io.Writer
	if log.bootstrap.Logging.Console.Pretty {
		consoleWriter = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		writers = append(writers, consoleWriter)
	} else {
		writers = append(writers, os.Stdout)
	}

	if log.bootstrap.Logging.ADX.Enabled {
		aWriter, err := adx.New(
			adx.Config(&config.Get().Logging.ADX),
			adx.CreateTables(true),
			adx.IngestionType(adx.QueuedIngestion),
			adx.KustoClient(log.adxClient),
			adx.Name("Log-Ingestor"),
			adx.Logger(&zerolog.Logger{}),
			adx.TableName(&haLogsTablename),
			adx.DataSample(haLogs{}),
		)
		if err != nil {
			return zerolog.Logger{}, err
		}

		writers = append(writers, aWriter)
	}

	multi = zerolog.MultiLevelWriter(writers...)

	hostname, _ := os.Hostname()

	return zerolog.New(multi).With().
		Timestamp().
		Caller().
		Str("host", hostname).
		Str("role", log.bootstrap.Monitor.Role).
		Str("region", log.bootstrap.Monitor.Region).
		Str("env", log.bootstrap.Environment).
		Logger(), nil
}
