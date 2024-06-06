/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package tasks

import (
	"context"
)

type Pipeline interface {
	// Next in the pipeline
	Next() Pipeline
}

type Task interface {
	Pipeline
	// NewTask
	NewTask() Task
	// Start the task
	Start(ctx context.Context) error
	// Stop the task
	Stop(ctx context.Context) error
}
