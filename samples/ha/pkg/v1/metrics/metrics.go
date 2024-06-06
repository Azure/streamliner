/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Azure/streamliner/samples/ha/pkg/config"
	adx "github.com/Azure/streamliner/samples/ha/pkg/v1/logger/adxIngest"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/rs/zerolog"
)

var ErrMetrics = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("metrics %w : %s", ErrMetrics, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("metrics %w : %s", err, text)
}

var (
	countersMetricsTableName = "counters_metrics_v2"
	gaugesMetricsTableName   = "gauges_metrics_v2"
	gatherInterval           = 60 * time.Second
)

// Metrics is a structure that holds the metrics data and related configurations.
// It includes fields for pipeline, stop signal, counter functions, gauges, initializers,
// ADX gauges and counters, ADX client, configuration, logger, counters, region, hostname, environment, name, role, and mutexes for host, gauge, and counter maps.
type Metrics struct {
	pipe         pipeline.Pipeline
	stop         chan bool
	counterFuncs map[string]func() uint64
	gauges       map[string]func() int64
	inits        map[interface{}]func()
	adxGauges    *adx.AzureDataExplorer
	adxCounters  *adx.AzureDataExplorer
	adxClient    *kusto.Client
	config       *config.Metrics
	log          *zerolog.Logger
	counters     map[string]uint64
	region       string
	hostName     string
	env          string
	name         string
	role         string
	hm           sync.Mutex
	gm           sync.Mutex
	cm           sync.Mutex
	isStarted    bool
}

// CounterValue is a structure that holds the counter value data.
// It includes fields for name, timestamp, host, environment, role, region, and value.
type CounterValue struct {
	Name      string `json:"name"`
	Timestamp string `json:"timestamp" kusto:"type:datetime"`
	Host      string `json:"host" kusto:"name:host"`
	Env       string `json:"env" kusto:"name:env"`
	Role      string `json:"role" kusto:"name:role"`
	Region    string `json:"region" kusto:"name:region"`
	Value     uint64 `json:"value"`
}

// GaugeValue is a structure that holds the gauge value data.
// It includes fields for name, timestamp, host, environment, role, region, and value.
type GaugeValue struct {
	Name      string `json:"name"`
	Timestamp string `json:"timestamp" kusto:"type:datetime"`
	Host      string `json:"host" kusto:"name:host"`
	Env       string `json:"env" kusto:"name:env"`
	Role      string `json:"role" kusto:"name:role"`
	Region    string `json:"region" kusto:"name:region"`
	Value     int64  `json:"value"`
}

type Option func(m *Metrics)

// Config is an Option that sets the configuration for the Metrics.
// It takes a pointer to a config.Metrics object and returns an Option.
// The returned Option, when called, sets the config field of a Metrics object.
func Config(cfg *config.Metrics) Option {
	return func(m *Metrics) { m.config = cfg }
}

// Logger is an Option that sets the logger for the Metrics.
// It takes a pointer to a zerolog.Logger object and returns an Option.
// The returned Option, when called, sets the log field of a Metrics object.
func Logger(log *zerolog.Logger) Option {
	return func(m *Metrics) { m.log = log }
}

// Next is an Option that sets the next pipeline for the Metrics.
// It takes a pipeline.Pipeline object and returns an Option.
// The returned Option, when called, sets the pipe field of a Metrics object.
func Next(next pipeline.Pipeline) Option {
	return func(m *Metrics) { m.pipe = next }
}

// Name is an Option that sets the name for the Metrics.
// It takes a string and returns an Option.
// The returned Option, when called, sets the name field of a Metrics object.
func Name(name string) Option {
	return func(m *Metrics) { m.name = name }
}

// Environment is an Option that sets the environment for the Metrics.
// It takes a string and returns an Option.
// The returned Option, when called, sets the env field of a Metrics object.
func Environment(name string) Option {
	return func(m *Metrics) { m.env = name }
}

// Region is an Option that sets the region for the Metrics.
// It takes a string and returns an Option.
// The returned Option, when called, sets the region field of a Metrics object.
func Region(name string) Option {
	return func(m *Metrics) { m.region = name }
}

// Role is an Option that sets the role for the Metrics.
// It takes a string and returns an Option.
// The returned Option, when called, sets the role field of a Metrics object.
func Role(name string) Option {
	return func(m *Metrics) { m.role = name }
}

// AdxClient is an Option that sets the ADX client for the Metrics.
// It takes a pointer to a kusto.Client object and returns an Option.
// The returned Option, when called, sets the adxClient field of a Metrics object.
func AdxClient(client *kusto.Client) Option {
	return func(m *Metrics) { m.adxClient = client }
}

// Name returns the name of the Metrics.
// It does not take any parameters and returns a string.
// The returned string is the name field of the Metrics object.
func (m *Metrics) Name() string {
	return m.name
}

// New creates a new Metrics processor.
// It takes a configuration and an ADX client as parameters.
// The function initializes the processor with the provided configuration and ADX client.
// It also sets up the ADX counters and gauges tables.
// If there is an error during the setup, it wraps the error with a generic error message and returns it.
// If the setup is successful, it returns the initialized processor.
func New(opts ...Option) (*Metrics, error) {
	var log zerolog.Logger

	proc := &Metrics{
		stop:         make(chan bool),
		counters:     make(map[string]uint64),
		counterFuncs: make(map[string]func() uint64),
		gauges:       make(map[string]func() int64),
		inits:        make(map[interface{}]func()),
	}

	for _, o := range opts {
		o(proc)
	}

	if proc.name == "" {
		log = proc.log.With().Str("task", "task").Logger()
	} else {
		log = proc.log.With().Str("task", proc.name).Logger()
	}

	proc.log = &log
	if proc.config == nil {
		return nil, ErrGenericError("cannot start translator without a configuration")
	}

	if proc.adxClient == nil {
		return nil, ErrGenericError("cannot start without adx information")
	}

	hostname, err := os.Hostname()

	if err != nil {
		proc.hostName = "noname"
	} else {
		proc.hostName = hostname
	}

	adxCounters, err := adx.New(
		adx.Config(&proc.config.ADX),
		adx.CreateTables(true),
		adx.IngestionType(adx.QueuedIngestion),
		adx.KustoClient(proc.adxClient),
		adx.Name("Metrics-Ingestor"),
		adx.Logger(proc.log),
		adx.TableName(&countersMetricsTableName),
		adx.DataSample(CounterValue{}),
	)

	if err != nil {
		return nil, ErrGenericErrorWrap("creating counters tables and mappings in adx", err)
	}

	adxGauges, err := adx.New(
		adx.Config(&proc.config.ADX),
		adx.CreateTables(true),
		adx.IngestionType(adx.QueuedIngestion),
		adx.KustoClient(proc.adxClient),
		adx.Name("Metrics-Ingestor"),
		adx.Logger(proc.log),
		adx.TableName(&gaugesMetricsTableName),
		adx.DataSample(GaugeValue{}),
	)

	if err != nil {
		return nil, ErrGenericErrorWrap("creating gauges tables and mappings in adx", err)
	}

	proc.adxCounters = adxCounters
	proc.adxGauges = adxGauges

	return proc, nil
}

// Snapshot returns a copy of the values of all registered counters and gauges.
// It locks the metrics maps to prevent concurrent access, then iterates over the initializer functions and calls them.
// After that, it unlocks the metrics maps and returns the counters and gauges maps.
func (m *Metrics) Snapshot() (c map[string]uint64, g map[string]int64) {
	m.hm.Lock()
	defer m.hm.Unlock()

	m.gm.Lock()
	defer m.gm.Unlock()

	m.cm.Lock()
	defer m.cm.Unlock()

	for _, init := range m.inits {
		init()
	}

	c = make(map[string]uint64, len(m.counters)+len(m.counterFuncs))
	for n, v := range m.counters {
		c[n] = v
	}

	for n, f := range m.counterFuncs {
		c[n] = f()
	}

	g = make(map[string]int64, len(m.gauges))
	for n, f := range m.gauges {
		g[n] = f()
	}

	return
}

// Reset removes all existing counters and gauges.
func (m *Metrics) Reset() {
	m.hm.Lock()
	defer m.hm.Unlock()

	m.gm.Lock()
	defer m.gm.Unlock()

	m.cm.Lock()
	defer m.cm.Unlock()

	m.counters = make(map[string]uint64)
	m.counterFuncs = make(map[string]func() uint64)
	m.gauges = make(map[string]func() int64)
	m.inits = make(map[interface{}]func())
}

// Note - If multiple locks must be concurrently held they should be
// acquired in this order hm, gm, cm or deadlock will result.

// A Counter is a monotonically increasing unsigned integer.

// Add increments the counter by one.
func (m *Metrics) CounterAdd(name string) {
	m.CounterAddN(name, 1)
}

// AddN increments the counter by N.
func (m *Metrics) CounterAddN(name string, delta uint64) {
	m.cm.Lock()
	m.counters[name] += delta
	m.cm.Unlock()
}

// SetFunc sets the counter's value to the lazily-called return value of the
// given function.
func (m *Metrics) CounterSetFunc(name string, f func() uint64) {
	m.cm.Lock()
	defer m.cm.Unlock()

	m.counterFuncs[name] = f
}

// SetBatchFunc sets the counter's value to the lazily-called return value of
// the given function, with an additional initializer function for a related
// batch of counters, all of which are keyed by an arbitrary value.
func (m *Metrics) CounterSetBatchFunc(name string, key interface{}, init func(), f func() uint64) {
	m.gm.Lock()
	defer m.gm.Unlock()

	m.cm.Lock()
	defer m.cm.Unlock()

	m.counterFuncs[name] = f
	if _, ok := m.inits[key]; !ok {
		m.inits[key] = init
	}
}

// Remove removes the given counter.
func (m *Metrics) CounterRemove(name string) {
	m.gm.Lock()
	defer m.gm.Unlock()

	m.cm.Lock()
	defer m.cm.Unlock()

	delete(m.counters, name)
	delete(m.counterFuncs, name)
	delete(m.inits, name)
}

// A Gauge is an instantaneous measurement of a value.
// Use a gauge to track metrics which increase and decrease (e.g., amount of free memory).
type Gauge string

// Set the gauge's value to the given value.
// It takes a name and a value as parameters. The function sets the gauge's value to the provided value.
func (m *Metrics) GaugeSet(name string, value int64) {
	m.gm.Lock()
	defer m.gm.Unlock()

	m.gauges[name] = func() int64 {
		return value
	}
}

// SetFunc sets the gauge's value to the lazily-called return value of the given function.
// It takes a name and a function as parameters. The function sets the gauge's value to the return value of the provided function.
func (m *Metrics) GaugeSetFunc(name string, f func() int64) {
	m.gm.Lock()
	defer m.gm.Unlock()

	m.gauges[name] = f
}

// SetBatchFunc sets the gauge's value to the lazily-called return value of the given function,
// with an additional initializer function for a related batch of gauges, all of which are keyed by an arbitrary value.
// It takes a name, a key, an initializer function, and a function as parameters.
// The function sets the gauge's value to the return value of the provided function and initializes a batch of gauges.
func (m *Metrics) GaugeSetBatchFunc(name string, key interface{}, init func(), f func() int64) {
	m.gm.Lock()
	defer m.gm.Unlock()

	m.gauges[name] = f
	if _, ok := m.inits[key]; !ok {
		m.inits[key] = init
	}
}

// Remove removes the given gauge.
// It takes a name as a parameter. The function removes the gauge with the provided name.
func (m *Metrics) GaugeRemove(name string) {
	m.gm.Lock()
	defer m.gm.Unlock()

	delete(m.gauges, name)
	delete(m.inits, name)
}

// Start starts the metrics collection.
// It takes a context as a parameter. The function starts the metrics collection.
func (m *Metrics) Start(ctx context.Context) error {
	var wg sync.WaitGroup

	var countersBytes *bytes.Buffer

	var gaugesBytes *bytes.Buffer

	wg.Add(1)

	go func() {
		defer wg.Done()

		if m.config.ADX.Enabled {
			m.log.Info().Dict("details", zerolog.Dict().Str("module", "metrics")).Msg("started")

			gatherTicker := time.NewTicker(gatherInterval)
			flushTicker := time.NewTicker(m.config.FlushInterval)

			countersBytes = &bytes.Buffer{}
			gaugesBytes = &bytes.Buffer{}

			for {
				select {
				case <-gatherTicker.C:
					m.RuntimeSnapshot()

					counters, gauges := m.Snapshot()

					err := m.EncodeCounterMetrics(countersBytes, counters)
					if err != nil {
						m.log.Err(err).Dict("details", zerolog.Dict().Str("module", "metrics")).Msg("encoding counters")
					}

					err = m.EncodeGaugeMetrics(gaugesBytes, gauges)
					if err != nil {
						m.log.Err(err).Dict("details", zerolog.Dict().Str("module", "metrics")).Msg("encoding gauges")
					}

				case <-flushTicker.C:
					err := m.WriteBufferToAdx(m.adxGauges, gaugesBytes)
					if err != nil {
						m.log.Err(err).Dict("details", zerolog.Dict().Str("module", "metrics")).Msg("writing gauges to adx")
					}

					err = m.WriteBufferToAdx(m.adxCounters, countersBytes)
					if err != nil {
						m.log.Err(err).Dict("details", zerolog.Dict().Str("module", "metrics")).Msg("writing counters to adx")
					}

					// Reset metrics buffer
					countersBytes = &bytes.Buffer{}
					gaugesBytes = &bytes.Buffer{}

				case <-m.stop:
					gatherTicker.Stop()
					flushTicker.Stop()
					m.log.Info().Str("module", "metrics").Msg("stopped")

					return
				}
			}
		}
	}()

	wg.Wait()

	m.isStarted = true

	return nil
}

// EncodeGaugeMetrics encodes the gauge metrics into a JSON format and writes it to the provided buffer.
// It takes a buffer and a map of gauges as parameters. Each gauge is encoded into a GaugeValue object.
// If there is an error during encoding, it wraps the error with a generic error message and returns it.
func (m *Metrics) EncodeGaugeMetrics(buf *bytes.Buffer, gauges map[string]int64) error {
	gaugesEncoder := json.NewEncoder(buf)

	for k, v := range gauges {
		obj := GaugeValue{
			Value:     v,
			Name:      k,
			Timestamp: time.Now().Format(time.RFC3339),
			Host:      m.hostName,
			Role:      m.role,
			Region:    m.region,
			Env:       m.env,
		}
		if err := gaugesEncoder.Encode(obj); err != nil {
			return ErrGenericErrorWrap("gauges encoding error", err)
		}
	}

	return nil
}

// WriteBufferToAdx writes the contents of the provided buffer to the Azure Data Explorer (ADX).
// It takes a destination ADX instance and a buffer as parameters. The buffer's bytes are written to the ADX.
// If there is an error during writing, it wraps the error with a generic error message and returns it.
func (m *Metrics) WriteBufferToAdx(dst *adx.AzureDataExplorer, buf *bytes.Buffer) error {
	if n, err := dst.Write(buf.Bytes()); err != nil {
		return ErrGenericErrorWrap(fmt.Sprintf(" wrote %d bytes to adx", n), err)
	}

	return nil
}

// EncodeCounterMetrics encodes the counter metrics into a JSON format and writes it to the provided buffer.
// It takes a buffer and a map of counters as parameters. Each counter is encoded into a CounterValue object.
// If there is an error during encoding, it wraps the error with a generic error message and returns it.
func (m *Metrics) EncodeCounterMetrics(buf *bytes.Buffer, counters map[string]uint64) error {
	countersEncoder := json.NewEncoder(buf)

	for k, v := range counters {
		obj := CounterValue{
			Value:     v,
			Name:      k,
			Timestamp: time.Now().Format(time.RFC3339),
			Host:      m.hostName,
			Role:      m.role,
			Region:    m.region,
			Env:       m.env,
		}
		if err := countersEncoder.Encode(obj); err != nil {
			return ErrGenericErrorWrap("counters encoding error", err)
		}
	}

	return nil
}

// Stop stops the metrics collection. It checks if the metrics collection has started, and if so, it sends a stop signal.
// It takes a context as a parameter. If the metrics collection is already stopped, it does nothing.
func (m *Metrics) Stop(ctx context.Context) error {
	if m.isStarted {
		m.isStarted = false
		m.stop <- true
	}

	return nil
}
