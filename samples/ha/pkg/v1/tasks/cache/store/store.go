/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package store

import "github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache"

type Store interface {
	// Get : Gets the record based on the key, returns false on not found and nil record
	Get(key string) cache.Data

	// Temp store functions create a temporary store that can be swapped with the main store at a later time
	CreateBufferStore() error
	// AddRecordToTemporaryStore : Adds a record to the temporary buffer store
	AddRecordToTemporaryStore(rec cache.Data) error
	// GetTemporaryStoreSize : Gets the size of the temporary buffer store
	GetTemporaryStoreSize() int
	// GetStoreSize : Gets the size of the temporary buffer store
	GetStoreSize() int
	// SwapWithTemporaryStore : Swaps the temporary buffer store with the main store
	SwapWithTemporaryStore() error
	// DiscardTemporaryStore : Discards the temporary buffer store
	DiscardTemporaryStore()
}
