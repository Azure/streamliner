/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package tasks

import (
	"context"

	"github.com/Azure/streamliner/samples/ha/pkg/config"
	v1 "github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"

	"github.com/rs/zerolog"
)

type Options struct {
	config    *config.Bootstrap
	log       *zerolog.Logger
	pipe      v1.Pipeline
	name      string
	isStarted bool
	stop      chan bool
	ctx       context.Context
	cancel    context.CancelFunc
}

type OptionsInterface interface {
	Name() string
	SetName(name string)
	Cancel() context.CancelFunc
	SetCancel(cancel context.CancelFunc)
	Context() context.Context
	SetContext(ctx context.Context)
	GetStop() chan bool
	IsStarted() bool
	SetStarted(isStarted bool)
	Pipeline() v1.Pipeline
	SetPipeline(pipe v1.Pipeline)
	Config() *config.Bootstrap
	SetConfig(config *config.Bootstrap)
	Logger() *zerolog.Logger
	SetLogger(log *zerolog.Logger)
}

func (b *Options) NewOptions() OptionsInterface {
	return &Options{
		stop: make(chan bool),
	}
}

func (b *Options) Name() string {
	return b.name
}

func (b *Options) SetName(name string) {
	b.name = name
}

func (o *Options) Cancel() context.CancelFunc {
	return o.cancel
}

func (o *Options) SetCancel(cancel context.CancelFunc) {
	o.cancel = cancel
}

func (o *Options) Context() context.Context {
	return o.ctx
}

func (o *Options) SetContext(ctx context.Context) {
	o.ctx = ctx
}

func (o *Options) GetStop() chan bool {
	return o.stop
}

func (o *Options) SetStop(stop chan bool) {
	o.stop = stop
}

func (o *Options) IsStarted() bool {
	return o.isStarted
}

func (o *Options) SetStarted(isStarted bool) {

	o.isStarted = isStarted
}

func (o *Options) Pipeline() v1.Pipeline {
	return o.pipe
}

func (o *Options) SetPipeline(pipe v1.Pipeline) {
	o.pipe = pipe
}

func (o *Options) Config() *config.Bootstrap {
	return o.config
}

func (o *Options) SetConfig(config *config.Bootstrap) {
	o.config = config
}

func (o *Options) Logger() *zerolog.Logger {
	return o.log
}

func (o *Options) SetLogger(log *zerolog.Logger) {
	o.log = log
}
