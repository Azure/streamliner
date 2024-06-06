/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package eventgrid

import (
	"context"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"
)

type Publisher interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Next(msgs *pipeline.Messages) error
	Name() string
}
