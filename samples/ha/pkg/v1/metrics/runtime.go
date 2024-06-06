/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package metrics

import (
	"runtime"
	"runtime/pprof"
)

const (
	RUNTIME_MEMSTATS_ALLOC        = "runtime.MemStats.Alloc"
	RUNTIME_MEMSTATS_BUCKHASHSYS  = "runtime.MemStats.BuckHashSys"
	RUNTIME_MEMSTATS_FREES        = "runtime.MemStats.Frees"
	RUNTIME_MEMSTATS_HEAPALLOC    = "runtime.MemStats.HeapAlloc"
	RUNTIME_MEMSTATS_HEAPIDLE     = "runtime.MemStats.HeapIdle"
	RUNTIME_MEMSTATS_HEAPINUSE    = "runtime.MemStats.HeapInuse"
	RUNTIME_MEMSTATS_HEAPOBJECTS  = "runtime.MemStats.HeapObjects"
	RUNTIME_MEMSTATS_HEAPRELEASED = "runtime.MemStats.HeapReleased"
	RUNTIME_MEMSTATS_HEAPSYS      = "runtime.MemStats.HeapSys"
	RUNTIME_MEMSTATS_LASTGC       = "runtime.MemStats.LastGC"
	RUNTIME_MEMSTATS_LOOKUPS      = "runtime.MemStats.Lookups"
	RUNTIME_MEMSTATS_MALLOCS      = "runtime.MemStats.Mallocs"
	RUNTIME_MEMSTATS_MCACHEINUSE  = "runtime.MemStats.MCacheInuse"
	RUNTIME_MEMSTATS_MCACHESYS    = "runtime.MemStats.MCacheSys"
	RUNTIME_MEMSTATS_MSPANINUSE   = "runtime.MemStats.MSpanInuse"
	RUNTIME_MEMSTATS_MSPANSYS     = "runtime.MemStats.MSpanSys"
	RUNTIME_MEMSTATS_NEXTGC       = "runtime.MemStats.NextGC"
	RUNTIME_MEMSTATS_NUMGC        = "runtime.MemStats.NumGC"
	RUNTIME_MEMSTATS_PAUSENS      = "runtime.MemStats.PauseNs"
	RUNTIME_MEMSTATS_PAUSETOTALNS = "runtime.MemStats.PauseTotalNs"
	RUNTIME_MEMSTATS_STACKINUSE   = "runtime.MemStats.StackInuse"
	RUNTIME_MEMSTATS_STACKSYS     = "runtime.MemStats.StackSys"
	RUNTIME_MEMSTATS_SYS          = "runtime.MemStats.Sys"
	RUNTIME_MEMSTATS_TOTALALLOC   = "runtime.MemStats.TotalAlloc"
	RUNTIME_MEMSTATS_NUMCGOCALL   = "runtime.NumCgoCall"
	RUNTIME_MEMSTATS_NUMGOROUTINE = "runtime.NumGoroutine"
	RUNTIME_MEMSTATS_NUMTHREAD    = "runtime.NumThread"
	RUNTIME_MEMSTATS_READMEMSTATS = "runtime.ReadMemStats"
)

var (
	memStats            runtime.MemStats
	frees               uint64
	lookups             uint64
	mallocs             uint64
	numGC               uint32
	numCgoCalls         int64
	threadCreateProfile = pprof.Lookup("threadcreate")
)

func (m *Metrics) RuntimeSnapshot() {
	runtime.ReadMemStats(&memStats) // This takes 50-200us.

	m.CounterAdd(RUNTIME_MEMSTATS_READMEMSTATS)

	m.GaugeSet(RUNTIME_MEMSTATS_ALLOC, int64(memStats.Alloc))
	m.GaugeSet(RUNTIME_MEMSTATS_BUCKHASHSYS, int64(memStats.BuckHashSys))

	m.GaugeSet(RUNTIME_MEMSTATS_FREES, int64(memStats.Frees-frees))
	m.GaugeSet(RUNTIME_MEMSTATS_HEAPALLOC, int64(memStats.HeapAlloc))
	m.GaugeSet(RUNTIME_MEMSTATS_HEAPIDLE, int64(memStats.HeapIdle))
	m.GaugeSet(RUNTIME_MEMSTATS_HEAPINUSE, int64(memStats.HeapInuse))
	m.GaugeSet(RUNTIME_MEMSTATS_HEAPOBJECTS, int64(memStats.HeapObjects))
	m.GaugeSet(RUNTIME_MEMSTATS_HEAPRELEASED, int64(memStats.HeapReleased))
	m.GaugeSet(RUNTIME_MEMSTATS_HEAPSYS, int64(memStats.HeapSys))
	m.GaugeSet(RUNTIME_MEMSTATS_LASTGC, int64(memStats.LastGC))
	m.GaugeSet(RUNTIME_MEMSTATS_LOOKUPS, int64(memStats.Lookups-lookups))
	m.GaugeSet(RUNTIME_MEMSTATS_MALLOCS, int64(memStats.Mallocs-mallocs))
	m.GaugeSet(RUNTIME_MEMSTATS_MCACHEINUSE, int64(memStats.MCacheInuse))
	m.GaugeSet(RUNTIME_MEMSTATS_SYS, int64(memStats.MCacheSys))
	m.GaugeSet(RUNTIME_MEMSTATS_MSPANINUSE, int64(memStats.MCacheInuse))
	m.GaugeSet(RUNTIME_MEMSTATS_NEXTGC, int64(memStats.NextGC))
	m.GaugeSet(RUNTIME_MEMSTATS_NUMGC, int64(memStats.NumGC-numGC))

	// <https://code.google.com/p/go/source/browse/src/pkg/runtime/mgc0.c>
	i := numGC % uint32(len(memStats.PauseNs))
	ii := memStats.NumGC % uint32(len(memStats.PauseNs))

	if memStats.NumGC-numGC >= uint32(len(memStats.PauseNs)) {
		for i = 0; i < uint32(len(memStats.PauseNs)); i++ {
			m.GaugeSet(RUNTIME_MEMSTATS_PAUSENS, int64(memStats.PauseNs[i]))
		}
	} else {
		if i > ii {
			for ; i < uint32(len(memStats.PauseNs)); i++ {
				m.GaugeSet(RUNTIME_MEMSTATS_PAUSENS, int64(memStats.PauseNs[i]))
			}
			i = 0
		}
		for ; i < ii; i++ {
			m.GaugeSet(RUNTIME_MEMSTATS_PAUSENS, int64(memStats.PauseNs[i]))
		}
	}

	frees = memStats.Frees
	lookups = memStats.Lookups
	mallocs = memStats.Mallocs
	numGC = memStats.NumGC

	m.GaugeSet(RUNTIME_MEMSTATS_PAUSETOTALNS, int64(memStats.PauseTotalNs))
	m.GaugeSet(RUNTIME_MEMSTATS_STACKINUSE, int64(memStats.StackInuse))
	m.GaugeSet(RUNTIME_MEMSTATS_STACKSYS, int64(memStats.StackSys))
	m.GaugeSet(RUNTIME_MEMSTATS_SYS, int64(memStats.Sys))
	m.GaugeSet(RUNTIME_MEMSTATS_TOTALALLOC, int64(memStats.TotalAlloc))

	currentNumCgoCalls := runtime.NumCgoCall()
	m.GaugeSet(RUNTIME_MEMSTATS_NUMCGOCALL, currentNumCgoCalls-numCgoCalls)
	numCgoCalls = currentNumCgoCalls

	m.GaugeSet(RUNTIME_MEMSTATS_NUMGOROUTINE, int64(runtime.NumGoroutine()))
	m.GaugeSet(RUNTIME_MEMSTATS_NUMTHREAD, int64(threadCreateProfile.Count()))
}
