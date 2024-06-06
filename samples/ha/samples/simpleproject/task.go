/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package main

import (
	"context"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks"
)

type OneTask struct {
	tasks.Options
}

// Next implements tasks.Task.
func (o *OneTask) Next() tasks.Pipeline {
	return nil
}

// Start implements tasks.Task.
func (o *OneTask) Start(ctx context.Context) error {
	o.Logger().Info().Msg("OneTask started")
	return nil
}

// Stop implements tasks.Task.
func (o *OneTask) Stop(ctx context.Context) error {
	o.Logger().Info().Msg("OneTask stopped")
	return nil
}

func (o *OneTask) NewTask() tasks.Task {
	return &OneTask{}
}
