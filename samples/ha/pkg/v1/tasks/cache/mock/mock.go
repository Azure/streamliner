/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package mock

import (
	"context"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache/store"

	"github.com/Azure/azure-kusto-go/kusto"
)

type Store struct {
	Data  []cache.Data
	Store store.Store
}

func (d *Store) UpdateCacheFromAdx(ctx context.Context) error {
	return nil
}
func (d *Store) Start(ctx context.Context) error {
	return nil
}
func (d *Store) Stop(ctx context.Context) error {
	return nil
}
func (d *Store) GetRecsFromQuery(rows *kusto.RowIterator) ([]cache.Data, error) {
	return []cache.Data{}, nil
}
func (d *Store) Name() string {
	return ""
}

func (d *Store) Get(key string) cache.Data {
	return cache.Data{}
}
