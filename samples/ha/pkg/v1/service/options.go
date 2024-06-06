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
)

type Options struct {
	Name    string
	Tasks   []tasks.Task
	ID      string
	Version string
	Ctx     context.Context
	Cancel  context.CancelFunc
	Signal  []os.Signal
	Logger  *zerolog.Logger
}
