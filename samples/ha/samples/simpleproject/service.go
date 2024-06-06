/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package main

import (
	"context"
	"os"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/service"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

type MyService struct {
	service.Options
}

// Context implements service.Service.
func (s *MyService) Context() context.Context {
	return s.Options.Ctx
}

func (s *MyService) GetCancelFunc() context.CancelFunc {
	return s.Options.Cancel
}

// GetApp implements service.Service.
func (s *MyService) GetApp() *cli.App {
	panic("unimplemented")
}

// ID implements service.Service.
func (s *MyService) ID() string {
	return s.Options.ID
}

// Logger implements service.Service.
func (s *MyService) Logger() *zerolog.Logger {
	return s.Options.Logger
}

// Name implements service.Service.
func (s *MyService) Name() string {
	return s.Options.Name
}

// SetLogger implements service.Service.
func (s *MyService) SetLogger(log *zerolog.Logger) {
	s.Options.Logger = log
}

// SetName implements service.Service.
func (s *MyService) SetName(name string) {
	s.Options.Name = name
}

// SetSignal implements service.Service.
func (s *MyService) SetSignal(sigs ...os.Signal) {
	s.Options.Signal = append(s.Options.Signal, sigs...)
}

// SetTasks implements service.Service.
func (s *MyService) SetTasks(tasks ...tasks.Task) {
	s.Options.Tasks = append(s.Options.Tasks, tasks...)
}

// Signal implements service.Service.
func (s *MyService) Signal() []os.Signal {
	return s.Options.Signal
}

// Tasks implements service.Service.
func (s *MyService) Tasks() []tasks.Task {
	return s.Options.Tasks
}

// Version implements service.Service.
func (s *MyService) Version() string {
	return s.Options.Version
}

// setContext implements service.Service.
func (s *MyService) SetContext(ctx context.Context) {
	s.Options.Ctx = ctx
}

func (s *MyService) NewService() service.Service {
	return s
}
