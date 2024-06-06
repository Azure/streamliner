/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package tasks

import (
	"context"
)

type Base struct {
	*Options
}

func (b *Base) Name() string {
	return b.name
}

func New(opts ...BaseOption) (*Base, error) {
	t := &Base{
		Options: &Options{
			stop: make(chan bool),
		},
	}

	for _, o := range opts {
		o(t.Options)
	}

	if t.config == nil {
		t.log.Fatal().Msg("cannot start translator without a configuration")
	}

	return t, nil
}

func (b *Base) Start(ctx context.Context) error {
	b.isStarted = true
	return nil
}

func (b *Base) Stop(ctx context.Context) error {
	if b.isStarted {
		b.isStarted = false
	}

	return nil
}
