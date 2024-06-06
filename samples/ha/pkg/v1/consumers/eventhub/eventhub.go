/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package eventhub

import (
	"context"
)

type Consumer interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
