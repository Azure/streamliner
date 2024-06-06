/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package app

import (
	"context"
	"os"

	v1 "github.com/Azure/streamliner/samples/ha/pkg/v1/tasks"

	"github.com/rs/zerolog"
)

type Options struct {
	ctx     context.Context
	cancel  func()
	id      string
	name    string
	version string

	tasks []v1.Task
	log   *zerolog.Logger
	sigs  []os.Signal
}

type AppOption func(o *Options)

// ID with service id.
func ID(id string) AppOption {
	return func(o *Options) { o.id = id }
}

// Name with service name.
func Name(name string) AppOption {
	return func(o *Options) { o.name = name }
}

// Version with service version.
func Version(version string) AppOption {
	return func(o *Options) { o.version = version }
}

// Context with service context.
func Context(parentCtx context.Context) AppOption {
	ctx, cancel := context.WithCancel(parentCtx)

	return func(o *Options) {
		o.ctx = ctx
		o.cancel = cancel
	}
}

// Signal with exit signals.
func Signal(sigs ...os.Signal) AppOption {
	return func(o *Options) { o.sigs = sigs }
}

// Process to start.
func Task(tasks ...v1.Task) AppOption {
	return func(o *Options) { o.tasks = tasks }
}

// Signal with exit signals.
func Logger(log *zerolog.Logger) AppOption {
	return func(o *Options) { o.log = log }
}

// App Option interface
type AppOptions interface {
	ID() string
	Name() string
	Version() string
	Log() *zerolog.Logger
}

// Implement AppOptions interface
// ID returns app instance id.
func (o *Options) ID() string { return o.id }

// Name returns service name.
func (o *Options) Name() string { return o.name }

// Version returns app version.
func (o *Options) Version() string { return o.version }

// Log returns app logger.
func (o *Options) Log() *zerolog.Logger { return o.log }
