/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package memory

import (
	"fmt"
	"sync"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache"

	"github.com/rs/zerolog"
)

var (
	ErrSwapFailed              = fmt.Errorf("swap failed")
	ErrTemporaryStoreExists    = fmt.Errorf("temp store already exists")
	ErrTemporaryStoreNotExists = fmt.Errorf("temp store does not exist")
	ErrTemporaryStoreSizeZero  = fmt.Errorf("temp store size is zero")
)

func ErrSwapErrorWrap(text string, err error) error {
	return fmt.Errorf("%w : %s", err, text)
}

type Memory struct {
	pool       sync.Pool
	store      MemStore
	tempStore  MemStore
	log        *zerolog.Logger
	locker     sync.RWMutex
	tempLocker sync.Mutex
}

type MemStore map[string]cache.Data

func New(logger *zerolog.Logger) *Memory {
	log := logger.With().Str("store", "in-memory").Logger()

	newStore := &Memory{
		store: make(MemStore),
		log:   &log,
	}

	newStore.pool = sync.Pool{
		New: func() interface{} {
			return make(MemStore)
		},
	}

	return newStore
}

func (m *Memory) Get(key string) cache.Data {
	m.locker.RLock()
	defer m.locker.RUnlock()
	data, found := m.store[key]

	if !found {
		return cache.Data{}
	}

	return data
}

func (m *Memory) CreateBufferStore() error {
	if m.tempStore != nil {
		return ErrTemporaryStoreExists
	}

	tempStore, ok := m.pool.Get().(MemStore)
	if tempStore == nil {
		return ErrSwapErrorWrap("pool returned nil", ErrSwapFailed)
	}

	if !ok {
		return ErrSwapErrorWrap("pool returned not MemStore", ErrSwapFailed)
	}

	tempStore.Clear()

	m.tempLocker.Lock()

	m.tempStore = tempStore

	m.tempLocker.Unlock()

	return nil
}

func (m *Memory) AddRecordToTemporaryStore(rec cache.Data) error {
	if m.tempStore == nil {
		if err := m.CreateBufferStore(); err != nil {
			return err
		}
	}

	m.tempLocker.Lock()
	defer m.tempLocker.Unlock()

	m.tempStore[rec.VMId] = rec

	return nil
}

func (m *Memory) GetTemporaryStoreSize() int {
	var size int

	if m.tempStore == nil {
		if err := m.CreateBufferStore(); err != nil {
			return 0
		}
	}

	m.tempLocker.Lock()
	defer m.tempLocker.Unlock()

	size = len(m.tempStore)

	return size
}

func (m *Memory) GetStoreSize() int {
	var size int

	if m.store == nil {
		return 0
	}

	m.locker.Lock()
	defer m.locker.Unlock()

	size = len(m.store)

	return size
}

func (m *Memory) SwapWithTemporaryStore() error {
	if m.tempStore == nil {
		return ErrTemporaryStoreNotExists
	}

	if m.GetTemporaryStoreSize() == 0 {
		return ErrTemporaryStoreSizeZero
	}

	m.tempLocker.Lock()
	m.locker.Lock()

	defer func() {
		m.locker.Unlock()
		m.tempLocker.Unlock()
	}()

	oldStore := m.store
	m.store = m.tempStore

	m.pool.Put(oldStore)

	m.tempStore = nil

	return nil
}

func (m *Memory) DiscardTemporaryStore() {
	if m.tempStore == nil {
		return
	}

	m.tempLocker.Lock()
	defer m.tempLocker.Unlock()

	ptr := m.tempStore
	m.tempStore = nil
	m.pool.Put(ptr)
}

func (s MemStore) Clear() {
	for k := range s {
		delete(s, k)
	}
}
