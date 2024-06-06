/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package cache

import (
	"context"
	"encoding/json"
	"time"
)

type Cache interface {
	UpdateCacheFromAdx(context.Context) error
	Get(key string) Data
}

const (
	ADDED    = "added"
	REMOVED  = "removed"
	MODIFIED = "modified"
)

type Data struct {
	// TODO: Struct not working for time.Time
	PreciseTimeStamp     time.Time `kusto:"timestamp" json:"timestamp"`
	VMId                 string    `kusto:"vmId" json:"vmId"`
	VMScaleSetResourceID string    `kusto:"VMScaleSetResourceId" json:"VMScaleSetResourceId"`
	Vmtype               string    `kusto:"vmType" json:"vmType"`
}

func (d *Data) String() string {
	buf, _ := json.Marshal(d)
	return string(buf)
}

func (d *Data) Bytes() []byte {
	buf, _ := json.Marshal(d)
	return buf
}
