/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type AppBase struct {
	*Options
}

func New(opts ...AppOption) *AppBase {
	a := AppBase{}

	if id, err := uuid.NewUUID(); err == nil {
		a.id = id.String()
	}

	for _, opt := range opts {
		opt(a.Options)
	}

	return &a
}

func (a *AppBase) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}

	return nil
}

// Run executes all Start hooks registered with the application's Lifecycle.
func (a *AppBase) Run() error {
	var err error

	eg, ctx := errgroup.WithContext(a.ctx)
	wg := sync.WaitGroup{}

	for _, task := range a.tasks {
		task := task

		// The first call to return a non-nil error cancels the group's context. The error will be returned by Wait.
		eg.Go(func() error {
			<-ctx.Done() // wait for stop signal
			stopCtx, cancel := context.WithCancel(a.ctx)
			defer cancel()
			err := task.Stop(stopCtx)

			zerolog.Dict().Str("task", task.Name())

			a.Log().Info().Str("task", task.Name()).Dict("details", zerolog.Dict()).Msgf("stopped")

			if err != nil {
				return ErrGenericErrorWrap("stopping task", err)
			}

			return nil
		})

		wg.Add(1)
		eg.Go(func() error {
			wg.Done() // this is to ensure server start has begun running before register, so defer is not needed
			a.Log().Info().Str("task", task.Name()).Dict("details", zerolog.Dict()).Msgf("starting")

			if err := task.Start(ctx); err != nil {
				return ErrGenericErrorWrap(fmt.Sprintf("starting task %s", task.Name()), err)
			}
			return nil
		})
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, a.sigs...)

	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case <-c:
			a.Log().Info().Msg("received stop signal")
			return a.Stop()
		}
	})

	if err = eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		a.Log().Error().Msg(err.Error())
		return ErrGenericErrorWrap("while waiting for context to be canceled", err)
	}

	return ErrGenericErrorWrap("cannot run", err)
}
