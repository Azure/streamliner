/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package chaos

import (
	"context"
	"errors"
	"fmt"
	"time"

	config "github.com/Azure/streamliner/samples/ha/pkg/config"

	"github.com/rs/zerolog"
)

var ErrChaos = errors.New("operation not permitted")

func ErrGenericError(text string) error {
	return fmt.Errorf("ChaosError %w : %s", ErrChaos, text)
}

type Service struct {
	config           *config.Chaos
	log              *zerolog.Logger
	name             string
	stopCh           chan bool
	isServiceStarted bool
	isChaosStarted   bool
}

type TaskOption func(o *Service)

func Config(config *config.Chaos) TaskOption {
	return func(s *Service) { s.config = config }
}

func Logger(log *zerolog.Logger) TaskOption {
	return func(s *Service) { s.log = log }
}

func Name(name string) TaskOption {
	return func(s *Service) { s.name = name }
}

func New(opts ...TaskOption) (*Service, error) {
	var log zerolog.Logger

	svc := &Service{
		stopCh:           make(chan bool),
		isServiceStarted: false,
		isChaosStarted:   false,
	}

	for _, o := range opts {
		o(svc)
	}

	if svc.name == "" {
		log = svc.log.With().Str("task", "task").Logger()
	} else {
		log = svc.log.With().Str("task", svc.name).Logger()
	}

	svc.log = &log

	if svc.config == nil {
		return nil, ErrGenericError("cannot start chaos without a configuration")
	}

	return svc, nil
}

// Start is the start of chaos experiment
// It will be trigger every interval, returning the returnErr during the specify interval
func (s *Service) Start(ctx context.Context) error {
	s.isServiceStarted = true
	s.isChaosStarted = false

	if s.config.Enabled {
		nextFailure := time.NewTicker(s.config.Failure.Every)
		s.log.Debug().Dict("details", zerolog.Dict().Str("service", s.name)).Msg("started")

		for {
			select {
			case <-nextFailure.C:
				nextFailure.Stop() // First thing we stop the ticker.

				if !s.isChaosStarted {
					s.log.Debug().Dict("details", zerolog.Dict().Dur("duration", s.config.Failure.Duration).Str("service", s.name)).Msg("chaos outage started")
					s.isChaosStarted = true

					if err := s.waitDuringChaos(ctx); err != nil {
						s.log.Err(err).Dict("details", zerolog.Dict().Dur("next", s.config.Failure.Every).Str("service", s.name)).Msg("waiting for caos")
					}

					s.log.Debug().Dict("details", zerolog.Dict().Dur("next", s.config.Failure.Every).Str("service", s.name)).Msg("chaos outage ended")
					s.isChaosStarted = false
				}

				nextFailure.Reset(s.config.Failure.Every)

			case <-s.stopCh:
				s.isChaosStarted = false

				nextFailure.Stop()

				return nil
			}
		}
	}

	return nil
}

func (s *Service) waitDuringChaos(ctx context.Context) error {
	timeoutChannel := make(chan bool)

	go func() {
		<-time.After(s.config.Failure.Duration)
		timeoutChannel <- true
	}()

	select {
	case <-timeoutChannel:
		return nil
	case <-ctx.Done():
		return nil
	}
}

func (s *Service) Fn(fn func() error, returnErr error) error {
	if s.isChaosStarted {
		return returnErr
	}

	return fn()
}

func (s *Service) Stop(ctx context.Context) error {
	if s.config.Enabled && s.isServiceStarted {
		s.log.Debug().Dict("details", zerolog.Dict().Str("service", s.name)).Msg("stopped")
		s.stopCh <- true
	}

	return nil
}

func (s *Service) Name() string {
	return s.name
}
