/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package ha

import (
	v1 "github.com/Azure/streamliner/samples/ha/pkg/v1/app"
)

type HA struct {
	*v1.AppBase
}

func New(opts ...v1.AppOption) *HA {
	app := v1.New(opts...)

	return &HA{app}
}

func (ef *HA) Start() error {
	return ef.Run()
}
