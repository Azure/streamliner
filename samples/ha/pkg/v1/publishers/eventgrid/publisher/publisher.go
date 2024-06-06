/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package publisher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	config "github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/metrics"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/publishers/eventgrid/chaos"

	"github.com/avast/retry-go"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

var ErrChaos = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("publisher %s : %w", text, ErrChaos)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("publisher %s : %w", text, err)
}

const (
	METRICS_PUBLISHER_EVENTGRID_PRIMARY_SENT_ERRORS   = "publisher_eventgrid_primary_sent_error"
	METRICS_PUBLISHER_EVENTGRID_SECONDARY_SENT_ERRORS = "publisher_eventgrid_secondary_sent_error"
	METRICS_PUBLISHER_EVENTGRID_SUCCESSFUL_SENDS      = "publisher_eventgrid_successful_sends"
)

type Task struct {
	config                *config.EventGrid
	log                   *zerolog.Logger
	pipe                  pipeline.Pipeline
	name                  string
	healthChannel         chan *healthstream.Message
	pool                  fastjson.ParserPool
	chaosPrimary          *chaos.Service
	chaosSecondary        *chaos.Service
	retryOptionsPrimary   []retry.Option
	retryOptionsSecondary []retry.Option
	met                   *metrics.Metrics
}

type TaskOption func(o *Task)

func Config(config *config.EventGrid) TaskOption {
	return func(t *Task) { t.config = config }
}

func Logger(log *zerolog.Logger) TaskOption {
	return func(t *Task) { t.log = log }
}

func Next(next pipeline.Pipeline) TaskOption {
	return func(t *Task) { t.pipe = next }
}

func Name(name string) TaskOption {
	return func(t *Task) { t.name = name }
}

func HealthChannel(healthCh chan *healthstream.Message) TaskOption {
	return func(t *Task) { t.healthChannel = healthCh }
}

func Metrics(met *metrics.Metrics) TaskOption {
	return func(t *Task) { t.met = met }
}

func New(opts ...TaskOption) (*Task, error) {
	var err error

	var log zerolog.Logger

	task := &Task{}

	for _, o := range opts {
		o(task)
	}

	if task.name == "" {
		log = task.log.With().Str("task", "task").Logger()
	} else {
		log = task.log.With().Str("task", task.name).Logger()
	}

	task.log = &log

	if task.config == nil {
		return nil, ErrGenericError("cannot start translator without a configuration")
	}

	task.config.Primary.Key = os.Getenv("EVENT_GRID_PRIMARY_KEY")
	task.config.Secondary.Key = os.Getenv("EVENT_GRID_SECONDARY_KEY")

	if task.config.Primary.Key == "" || task.config.Secondary.Key == "" {
		return nil, ErrGenericError("cannot find EVENT_GRID_PRIMARY_KEY or EVENT_GRID_SECONDARY_KEY env variables")
	}

	// Create caos
	chaosPrimary, err := chaos.New(
		chaos.Config(&task.config.Primary.Chaos),
		chaos.Logger(task.log),
		chaos.Name("chaos-primary-grid"),
	)
	if err != nil {
		return nil, ErrGenericErrorWrap("creating primary grid chaos experiment", err)
	}

	task.chaosPrimary = chaosPrimary
	// Create caos
	chaosSecondary, err := chaos.New(
		chaos.Config(&task.config.Secondary.Chaos),
		chaos.Logger(task.log),
		chaos.Name("chaos-secondary-grid"),
	)
	if err != nil {
		return nil, ErrGenericErrorWrap("creating second grid chaos experiment", err)
	}

	task.chaosSecondary = chaosSecondary

	task.retryOptionsPrimary = []retry.Option{
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(task.config.Primary.RetryOptions.Delay), // Initial Delay
		retry.MaxJitter(task.config.Primary.RetryOptions.MaxJitter),
		retry.Attempts(task.config.Primary.RetryOptions.Attempts),
		retry.OnRetry(func(n uint, err error) {
			task.log.Err(err).Dict("details", zerolog.Dict().Str("eventgrid", "primary")).Msg("retrying request")
		}),
	}

	task.retryOptionsSecondary = []retry.Option{
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(task.config.Secondary.RetryOptions.Delay), // Initial Delay
		retry.MaxJitter(task.config.Secondary.RetryOptions.MaxJitter),
		retry.Attempts(task.config.Secondary.RetryOptions.Attempts),
		retry.OnRetry(func(n uint, err error) {
			task.log.Err(err).Dict("details", zerolog.Dict().Str("eventgrid", "secondary")).Msg("retrying request")
		}),
	}

	return task, nil
}

func (t *Task) Start(ctx context.Context) error {
	go func() {
		if err := t.chaosPrimary.Start(ctx); err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict()).Msg("error publishing to primary grid")
		}
	}()

	go func() {
		if err := t.chaosSecondary.Start(ctx); err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict()).Msg("error publishing to secondary grid")
		}
	}()

	t.log.Info().Dict("details", zerolog.Dict()).Msg("started")

	return nil
}
func (t *Task) Stop(ctx context.Context) error {
	_ = t.chaosPrimary.Stop(ctx)
	_ = t.chaosSecondary.Stop(ctx)

	t.log.Info().Dict("details", zerolog.Dict()).Msg("stopped")

	return nil
}

func (t *Task) Next(gridMsgs *pipeline.Messages) {
	for _, gridMsg := range *gridMsgs {
		err := t.publishToGrid(gridMsg)
		if err != nil {
			t.sendHealthFailure(err)
			return
		}

		t.sendHealthSuccess()
	}
}
func (t *Task) sendHealthSuccess() {
	t.healthChannel <- &healthstream.Message{
		Type:   healthstream.Success,
		Msg:    "health success",
		Origin: t.name,
	}
}

func (t *Task) sendHealthFailure(err error) {
	t.healthChannel <- &healthstream.Message{
		Type:   healthstream.Failover,
		Msg:    fmt.Sprintf("health failure: %s", err.Error()),
		Origin: t.name,
	}
}

func (t *Task) publishToGrid(msg pipeline.Message) error {
	parser := t.pool.Get()

	jsonGridMsg, err := parser.Parse(msg.String())
	if err != nil {
		return ErrGenericErrorWrap("parsing message from grid", err)
	}

	scheduleEvents, err := jsonGridMsg.Array()
	if err != nil {
		return ErrGenericErrorWrap("parsing array", err)
	}

	// Change topic to the grid
	t.setTopicForPayload(&scheduleEvents, &t.config.Primary)

	err = t.sendToGrid([]byte(jsonGridMsg.String()), t.config.Primary.Endpoint, t.config.Primary.Key, t.retryOptionsPrimary, t.chaosPrimary)

	if err != nil {
		t.log.Err(err).Dict("details", zerolog.Dict().Str("eventgrid", "primary")).Msg("error sending")
		t.met.CounterAdd(METRICS_PUBLISHER_EVENTGRID_PRIMARY_SENT_ERRORS)

		// Send to secondary
		t.setTopicForPayload(&scheduleEvents, &t.config.Secondary)

		err = t.sendToGrid([]byte(jsonGridMsg.String()), t.config.Secondary.Endpoint, t.config.Secondary.Key, t.retryOptionsSecondary, t.chaosSecondary)

		if err != nil {
			t.met.CounterAdd(METRICS_PUBLISHER_EVENTGRID_SECONDARY_SENT_ERRORS)

			t.log.Err(err).Dict("details", zerolog.Dict().Str("eventgrid", "secondary")).Msg("error sending, giving up")

			return ErrGenericErrorWrap("giving up", err)
		}
	}

	t.met.CounterAdd(METRICS_PUBLISHER_EVENTGRID_SUCCESSFUL_SENDS)

	t.pool.Put(parser)
	t.sendHealthSuccess()

	return nil
}

func (t *Task) setTopicForPayload(scheduleEvents *[]*fastjson.Value, conf *config.EventGridOptions) {
	for _, scheduleEvent := range *scheduleEvents {
		scheduleEvent.Set("topic", fastjson.MustParse(fmt.Sprintf("%q", conf.Topic)))
	}
}

func (t *Task) sendToGrid(msg []byte, endpoint, key string, retryOptions []retry.Option, chaosFailure *chaos.Service) error {
	request := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(request)

	request.Header.SetMethod(fasthttp.MethodPost)
	request.Header.Set("Content-Type", "application/cloudevents+json")
	request.Header.Set("aeg-sas-key", key)
	request.Header.Set("User-Agent", "github.com/Azure/streamliner/samples/ha")
	request.SetRequestURI(endpoint)
	request.SetBody(msg)

	response := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(response)

	client := &fasthttp.Client{WriteTimeout: time.Second * 10}

	err := retry.Do(
		func() error {
			err := chaosFailure.Fn(func() error {
				if err := client.Do(request, response); err != nil {
					return ErrGenericErrorWrap("error executing request", err)
				}
				return nil
			}, ErrGenericError("returning from function"))

			if err != nil {
				return ErrGenericErrorWrap("chaos experiment", err) // Only send last error
			}
			return nil
		},
		retryOptions...,
	)
	if err != nil {
		t.log.Err(err).Dict("details", zerolog.Dict().Str("endpoint", endpoint)).Msg("event grid unreachable")
		return ErrGenericErrorWrap("executing retries", err)
	}

	if response.StatusCode() != fasthttp.StatusOK {
		body := response.Body()
		return ErrGenericError(fmt.Sprintf("sending to grid: %s", string(body)))
	}

	return nil
}

func (t *Task) Name() string {
	return t.name
}
