/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package monitor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	config "github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/consumers/eventhub"
	consumer "github.com/Azure/streamliner/samples/ha/pkg/v1/consumers/eventhub/consumer"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/metrics"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/health"

	"github.com/rs/zerolog"
)

var ErrMonitor = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("monitor %w : %s", ErrMonitor, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("monitor %s : %w", text, err)
}

const (
	METRICS_IS_SECONDARY_STARTED = "monitor_secondary_started"
	METRICS_IS_PRIMARY_STARTED   = "monitor_primary_started"
)

type Task struct {
	config      *config.Monitor
	log         *zerolog.Logger
	originalLog *zerolog.Logger
	pipe        pipeline.Pipeline
	name        string
	ctx         context.Context
	cancel      context.CancelFunc

	isEHConsumerStarted bool
	connectionString    string
	consumer            eventhub.Consumer
	consumerIncarnation int64

	health            *health.Task
	healthChannel     chan *healthstream.Message
	stopHealthMonitor chan bool
	// Metrics
	met *metrics.Metrics
}

type TaskOption func(o *Task)

func Config(config *config.Monitor) TaskOption {
	return func(t *Task) { t.config = config }
}

func Logger(log *zerolog.Logger) TaskOption {
	return func(t *Task) { t.log = log }
}

func Start(next pipeline.Pipeline) TaskOption {
	return func(t *Task) { t.pipe = next }
}

func ConnectionString(connectionString string) TaskOption {
	return func(t *Task) { t.connectionString = connectionString }
}

func HealthChannel(healthCh chan *healthstream.Message) TaskOption {
	return func(t *Task) { t.healthChannel = healthCh }
}

func Metrics(met *metrics.Metrics) TaskOption {
	return func(t *Task) { t.met = met }
}

func Name(name string) TaskOption {
	return func(t *Task) { t.name = name }
}

func (t *Task) Name() string {
	return t.name
}

func New(opts ...TaskOption) (*Task, error) {
	var log zerolog.Logger

	var err error

	proc := &Task{
		stopHealthMonitor: make(chan bool),
	}

	for _, o := range opts {
		o(proc)
	}

	proc.originalLog = proc.log

	if proc.name == "" {
		log = proc.log.With().Str("task", "task").Logger()
	} else {
		log = proc.log.With().Str("task", proc.name).Logger()
	}

	proc.log = &log

	if proc.config == nil {
		return nil, ErrGenericError("cannot start without a configuration")
	}

	if proc.healthChannel == nil {
		return nil, ErrGenericError("cannot start without a healthChannel")
	}

	if proc.met == nil {
		return nil, ErrGenericError("cannot start without metrics server")
	}

	proc.health, err = health.New(
		health.Config(&proc.config.Health),
		health.Logger(proc.log),
		health.Name("health"),
		health.Role(proc.config.Role),
		health.Region(proc.config.Region),
		health.Channel(proc.healthChannel),
		health.Metrics(proc.met),
	)

	if err != nil {
		return nil, ErrGenericErrorWrap("creating health object", err)
	}

	proc.met.GaugeSetFunc(METRICS_IS_PRIMARY_STARTED, func() int64 {
		if proc.isPrimary() && proc.isEHConsumerStarted {
			return 1
		}
		return 0
	})

	proc.met.GaugeSetFunc(METRICS_IS_SECONDARY_STARTED, func() int64 {
		if !proc.isPrimary() && proc.isEHConsumerStarted {
			return 1
		}
		return 0
	})

	return proc, nil
}

func (t *Task) Start(ctx context.Context) error {
	go func() {
		if err := t.health.Start(ctx); err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict()).Msg("error starting health")
		}
	}()

	go func() {
		if err := t.startMonitor(ctx); err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict()).Msg("error starting monitor")
		}
	}()

	if t.isPrimary() {
		err := t.startEventHubConsumer(ctx)
		if err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict()).Msg("error starting primary eventhub consumer")
		}
	}

	t.log.Info().Dict("details", zerolog.Dict()).Msg("started")

	return nil
}

func (t *Task) startMonitor(ctx context.Context) error {
	t.log.Info().Dict("details", zerolog.Dict().
		Str("module", "monitor"),
	).Msg("started")

	for {
		select {
		case <-t.stopHealthMonitor:
			t.log.Debug().Dict("details", zerolog.Dict().
				Str("module", "monitor"),
			).Msg("started")

			return nil
		case msg := <-t.healthChannel:
			switch msg.Type {
			case healthstream.Failover:
				t.health.SetMyHealth(health.BAD)

			case healthstream.Success:
				t.health.SetMyHealth(health.GOOD)

			case healthstream.TakeoverOn: // This should only happens when the read timeout(after primary fails to write)
				// Start secondary
				err := t.startEventHubConsumer(ctx)
				if err != nil {
					t.log.Err(err).Dict("details", zerolog.Dict().
						Str("module", "monitor"),
					).Msg("starting secondary eventhub consumer")

					continue
				}

				t.log.Info().Dict("details", zerolog.Dict().
					Str("msg", msg.Msg),
				).Msg("takeOver on")

			case healthstream.TakeoverOff:

				err := t.stopEventHubConsumer(ctx)
				if err != nil {
					t.log.Err(err).Dict("details", zerolog.Dict().
						Str("module", "monitor"),
					).Msg("stoping secondary eventhub consumer")
				}
			}
		}
	}
}

func (t *Task) startEventHubConsumer(ctx context.Context) error {
	var err error

	ctx, cancel := context.WithCancel(ctx)
	t.ctx = ctx
	t.cancel = cancel

	t.consumer, err = t.CreateEventHubConsumer()
	if err != nil {
		return ErrGenericErrorWrap("creating eventhub consumer", err)
	}

	go func() {
		if err := t.consumer.Start(ctx); err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict()).Msg("error starting consumer")
			t.isEHConsumerStarted = false
		}
	}()

	t.isEHConsumerStarted = true

	return nil
}

func (t *Task) Stop(ctx context.Context) error {
	_ = t.health.Stop(ctx)
	t.stopHealthMonitor <- true
	err := t.stopEventHubConsumer(ctx)
	if err != nil {
		t.log.Err(err).Dict("details", zerolog.Dict()).
			Msg("consumer stop error")
	}

	t.log.Info().Dict("details", zerolog.Dict()).Msg("stopped")

	return nil
}

func (t *Task) stopEventHubConsumer(ctx context.Context) error {
	defer func() {
		// GC object to avoid leaks
		t.consumer = nil
		t.isEHConsumerStarted = false

		runtime.GC()
	}()

	if t.consumer != nil && t.isEHConsumerStarted {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		t.cancel()

		err := t.consumer.Stop(ctx)
		if err != nil {
			return err
		}
	} else {
		return ErrGenericError("cannot stop what it has not been started")
	}
	return nil
}

func (t *Task) CreateEventHubConsumer() (eventhub.Consumer, error) {
	incarnationStr := fmt.Sprintf("%06d", t.consumerIncarnation)
	t.consumerIncarnation += 1

	c, err := consumer.New(
		consumer.Config(&config.Get().EventHub),
		consumer.Logger(t.originalLog),
		consumer.ConnectionString(os.Getenv("AZURE_EVENTHUB_CONNECTIONSTRING")),
		consumer.Next(t.pipe),
		consumer.Name(fmt.Sprintf("eh-consumer-%s", incarnationStr)),
		consumer.HealthChannel(t.healthChannel),
		consumer.Metrics(t.met),
	)
	if err != nil {
		return nil, ErrGenericErrorWrap("creating eventhub consumer", err)
	}

	return c, nil
}

func (t *Task) printMemoryUsage(title string) {
	var m runtime.MemStats

	runtime.ReadMemStats(&m)
	t.log.Debug().Dict("details", zerolog.Dict().
		Uint64("alloc", m.Alloc/1024/1024).
		Uint64("totalAlloc", m.TotalAlloc/1024/1024).
		Uint64("sys", m.Sys/1024/1024).
		Uint32("numGC", m.NumGC),
	).Msg(title)
}

func (t *Task) isPrimary() bool {
	return strings.EqualFold(t.config.Role, "primary")
}
