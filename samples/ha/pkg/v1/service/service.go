/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package service

import (
	"context"
	"os"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

type Flags interface {
	Flags() []cli.Flag
}

type Service interface {
	// NewService
	NewService() Service
	// Name of the service
	Name() string
	SetName(name string)
	// Tasks associated to the service
	Tasks() []tasks.Task
	SetTasks(tasks ...tasks.Task)
	// ID of the service
	ID() string
	// Version of the service
	Version() string
	// Context
	Context() context.Context
	SetContext(ctx context.Context)
	GetCancelFunc() context.CancelFunc
	// Signals
	Signal() []os.Signal
	SetSignal(sigs ...os.Signal)
	// Logger
	Logger() *zerolog.Logger
	SetLogger(log *zerolog.Logger)
	// GetApp urfave/cli app
	GetApp() *cli.App
}
