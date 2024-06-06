/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package health

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	config "github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/metrics"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/health/store"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var ErrHealth = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("health %w : %s", ErrHealth, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("health %s : %w", text, err)
}

type State int

const (
	GOOD State = iota
	BAD
	UNKNOWN
)

const (
	METRICS_HEALTH_MY_HEALTH   = "health_my_health"
	METRICS_HEALTH_PEER_HEALTH = "health_peer_health"
)

func (s State) String() string {
	switch s {
	case GOOD:
		return "Good"
	case BAD:
		return "Bad"
	case UNKNOWN:
		return "Unknown"
	default:
		return ""
	}
}

type Task struct {
	config *config.Health
	log    *zerolog.Logger
	pipe   pipeline.Pipeline
	name   string

	client         *container.Client
	role           string
	myHealth       State
	myHealthLock   sync.RWMutex
	peerHealth     State
	peerHealthLock sync.RWMutex
	store          *store.BlobStore

	isStarted     bool
	stop          chan bool
	uuid          string
	region        string
	healthChannel chan *healthstream.Message
	met           *metrics.Metrics
}

type TaskOption func(o *Task)

func Config(config *config.Health) TaskOption {
	return func(t *Task) { t.config = config }
}

func Logger(log *zerolog.Logger) TaskOption {
	return func(t *Task) { t.log = log }
}

func Next(next pipeline.Pipeline) TaskOption {
	return func(t *Task) { t.pipe = next }
}

func Region(name string) TaskOption {
	return func(t *Task) { t.region = name }
}

func Name(name string) TaskOption {
	return func(t *Task) { t.name = name }
}

func Role(role string) TaskOption {
	return func(t *Task) { t.role = role }
}

func Channel(healthCh chan *healthstream.Message) TaskOption {
	return func(t *Task) { t.healthChannel = healthCh }
}

func Metrics(met *metrics.Metrics) TaskOption {
	return func(t *Task) { t.met = met }
}

func New(opts ...TaskOption) (*Task, error) {
	var log zerolog.Logger

	task := &Task{
		stop:       make(chan bool),
		uuid:       uuid.New().String(),
		myHealth:   BAD,
		peerHealth: GOOD,
	}

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

	if task.region == "" || task.uuid == "" {
		return nil, ErrGenericError("cannot start health wihtout a region or uuid")
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, ErrGenericErrorWrap("creating azure credentials", err)
	}

	blobClient, err := azblob.NewClient(task.config.Checkpoint.URL, credential, nil)

	if err != nil {
		return nil, ErrGenericErrorWrap("creating client for azure storage", err)
	}

	containerClient := blobClient.ServiceClient().NewContainerClient(task.config.Checkpoint.ContainerName)
	task.client = containerClient

	if _, err := containerClient.Create(context.Background(), nil); err != nil {
		if !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
			return nil, ErrGenericErrorWrap("container client error", err)
		}
	} else {
		task.log.Info().Dict("details", zerolog.Dict().Str("container", task.config.Checkpoint.ContainerName)).Msg("container created")
	}

	task.store, err = store.NewBlobStore(containerClient)
	if err != nil {
		return nil, ErrGenericErrorWrap("blob store client", err)
	}

	// We always start healthy
	task.SetMyHealth(GOOD)

	task.met.GaugeSetFunc(METRICS_HEALTH_MY_HEALTH, func() int64 {
		if task.GetMyHealth() == GOOD {
			return 1
		}

		return 0
	})

	task.met.GaugeSetFunc(METRICS_HEALTH_PEER_HEALTH, func() int64 {
		if task.GetPeerHealth() == GOOD {
			return 1
		}

		return 0
	})

	return task, nil
}

func (t *Task) Start(ctx context.Context) error {
	t.isStarted = true
	t.log.Info().Dict("details", zerolog.Dict().
		Str("role", t.role).
		Dur("checkpointInterval", t.config.Interval)).Msg("started")

	if strings.EqualFold(t.role, "primary") {
		timer := time.NewTicker(t.config.Interval)

		for {
			select {
			case <-timer.C:
				err := t.WriteCheckpointHealth(ctx)
				if err != nil {
					t.log.Info().Dict("details", zerolog.Dict().
						Str("role", t.role)).Err(err).Msg("setCheckpoint")

					t.isStarted = false

					return err
				}
			case <-t.stop:
				return nil
			}
		}
	} else { // Secondary reads only
		timer := time.NewTicker(t.config.Interval)
		for {
			select {
			case <-timer.C:
				err := t.ReadCheckpointHealth(ctx)
				if err != nil {
					t.log.Info().Dict("details", zerolog.Dict().
						Str("role", t.role)).Err(err).Msg("readCheckpoint")
					t.isStarted = false

					return err
				}

			case <-t.stop:

				return nil
			}
		}
	}
}

func (t *Task) WriteCheckpointHealth(ctx context.Context) error {
	var err error
	if t.myHealth == GOOD {
		_, _, err = t.store.SetCheckpoint(ctx, store.Checkpoint{
			Region: t.region,
			UUID:   t.uuid,
		})
		if err != nil {
			return ErrGenericErrorWrap("setting health checkpoint", err)
		}
	}

	return nil
}

func (t *Task) ReadCheckpointHealth(ctx context.Context) error {
	lastModifyTime, err := t.store.GetCheckpointLastModify(ctx)
	if err != nil {
		return ErrGenericErrorWrap("getting last modify time", err)
	}

	if time.Now().After(lastModifyTime.Add(t.config.PreemptionHoldTime)) {
		t.log.Info().Dict("details", zerolog.Dict().
			Str("role", t.role).
			Time("lastModifyTime", lastModifyTime),
		).Msg("preemption holdtime has pass setting peer health to bad")

		t.SetPeerHealth(BAD)

		return nil
	}

	t.SetPeerHealth(GOOD)

	return nil
}

func (t *Task) Stop(ctx context.Context) error {
	if t.isStarted {
		t.stop <- true
	}

	t.log.Info().Dict("details", zerolog.Dict().
		Str("role", t.role)).Msg("stopped")

	return nil
}

func (t *Task) Name() string {
	return t.name
}

func (t *Task) SetMyHealth(newState State) {
	t.myHealthLock.Lock()
	defer t.myHealthLock.Unlock()

	if t.myHealth == newState {
		return
	}

	t.myHealth = newState
	t.log.Info().Dict("details", zerolog.Dict().
		Str("role", t.role).
		Str("myHealth", newState.String()),
	).Msg("health change")
}

func (t *Task) GetMyHealth() State {
	t.myHealthLock.RLock()
	defer t.myHealthLock.RUnlock()

	return t.myHealth
}

func (t *Task) SetPeerHealth(newState State) {
	t.peerHealthLock.Lock()
	defer t.peerHealthLock.Unlock()

	if t.peerHealth == newState {
		return
	}

	t.peerHealth = newState
	t.log.Info().Dict("details", zerolog.Dict().
		Str("role", t.role).
		Str("peerHealth", newState.String()),
	).Msg("health change")

	if t.peerHealth == GOOD {
		t.healthChannel <- &healthstream.Message{
			Type:   healthstream.TakeoverOff,
			Origin: "health",
			Msg:    "peer health changed to good",
		}

		return
	}

	t.healthChannel <- &healthstream.Message{
		Type:   healthstream.TakeoverOn,
		Origin: "health",
		Msg:    "peer health changed to bad",
	}
}

func (t *Task) GetPeerHealth() State {
	t.peerHealthLock.RLock()
	defer t.peerHealthLock.RUnlock()

	return t.peerHealth
}
