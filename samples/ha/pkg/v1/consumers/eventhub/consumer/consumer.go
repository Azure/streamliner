/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package eventhubconsumer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	config "github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/consumers/eventhub"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/metrics"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/checkpoints"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/rs/zerolog"
)

var ErrConsumer = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("eventhub consumer %w : %s", ErrConsumer, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("eventhub consumer %w : %s", err, text)
}

const (
	MAX_EVENTS                       = 64
	MAX_OUTPUT_QUEUE                 = 128
	METRICS_EVENTHUB_RECEIVED_EVENTS = "consumer_eventhub_received_events"
)

type Consumer struct {
	config             *config.EventHub
	log                *zerolog.Logger
	consumerCheckpoint *checkpoints.BlobStore
	consumerClient     *azeventhubs.ConsumerClient
	connectionString   string
	cancel             context.CancelFunc
	ctx                *context.Context
	pipe               pipeline.Pipeline
	name               string
	healthChannel      chan *healthstream.Message
	met                *metrics.Metrics
	startPosition      azeventhubs.StartPosition
}

type ConsumerOption func(o *Consumer)

func Config(config *config.EventHub) ConsumerOption {
	return func(c *Consumer) { c.config = config }
}

func Logger(log *zerolog.Logger) ConsumerOption {
	return func(c *Consumer) { c.log = log }
}

func Next(next pipeline.Pipeline) ConsumerOption {
	return func(c *Consumer) { c.pipe = next }
}

func ConnectionString(connectionString string) ConsumerOption {
	return func(c *Consumer) { c.connectionString = connectionString }
}

func Name(name string) ConsumerOption {
	return func(c *Consumer) { c.name = name }
}

func HealthChannel(healthCh chan *healthstream.Message) ConsumerOption {
	return func(c *Consumer) { c.healthChannel = healthCh }
}

func Metrics(met *metrics.Metrics) ConsumerOption {
	return func(c *Consumer) { c.met = met }
}

func StartPosition(position azeventhubs.StartPosition) ConsumerOption {
	return func(c *Consumer) { c.startPosition = position }
}

// New creates a new EventHub consumer
func New(opts ...ConsumerOption) (eventhub.Consumer, error) {
	var log zerolog.Logger

	var consumerClient *azeventhubs.ConsumerClient

	latest := true
	proc := &Consumer{
		startPosition: azeventhubs.StartPosition{
			Latest: &latest,
		},
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
		return nil, ErrGenericError("cannot start without a configuration")
	}

	if proc.met == nil {
		return nil, ErrGenericError("cannot start without metrics server")
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, ErrGenericErrorWrap("creating azure credentials", err)
	}

	blobClient, err := azblob.NewClient(proc.config.Checkpoint.URL, credential, nil)

	if err != nil {
		return nil, ErrGenericErrorWrap("creating storage account client", err)
	}

	containerClient := blobClient.ServiceClient().NewContainerClient(proc.config.Checkpoint.ContainerName)

	if _, err := containerClient.Create(context.Background(), nil); err != nil {
		if !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
			return nil, ErrGenericErrorWrap("creating storage container", err)
		}

		proc.log.Info().Dict("details", zerolog.Dict().Str("container", proc.config.Checkpoint.ContainerName)).Msg("container created")
	}

	consumerCheckpoint, err := checkpoints.NewBlobStore(containerClient, nil)

	if err != nil {
		return nil, ErrGenericErrorWrap("creating consumer checkpoing", err)
	}

	if os.Getenv("AZURE_EVENTHUB_CONNECTIONSTRING") != "" {
		connectionString := os.Getenv("AZURE_EVENTHUB_CONNECTIONSTRING")

		proc.log.Info().Msg("using AZURE_EVENTHUB_CONNNECTIONSTRING for credential")
		consumerClient, err = azeventhubs.NewConsumerClientFromConnectionString(connectionString, proc.config.Name, proc.config.ConsumerGroup, nil)

		if err != nil {
			return nil, ErrGenericErrorWrap("creating consumer client from connection string", err)
		}
	} else {
		consumerClient, err = azeventhubs.NewConsumerClient(proc.config.Namespace, proc.config.Name, proc.config.ConsumerGroup, credential, nil)

		if err != nil {
			return nil, ErrGenericErrorWrap("creating consumer client from default credentials", err)
		}
	}

	proc.consumerCheckpoint = consumerCheckpoint
	proc.consumerClient = consumerClient

	return proc, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = &ctx
	c.cancel = cancel

	opts := &azeventhubs.ProcessorOptions{}

	if c.config.Strategy == config.StreategyGreedy {
		opts.LoadBalancingStrategy = azeventhubs.ProcessorStrategyGreedy

		c.log.Info().Dict("details", zerolog.Dict()).Msgf("processor strategy greedy")
	} else { // By default we use balanced
		opts.LoadBalancingStrategy = azeventhubs.ProcessorStrategyBalanced

		c.log.Info().Dict("details", zerolog.Dict()).Msgf("processor strategy balanced")
	}

	opts.StartPositions.Default = c.startPosition

	processor, err := azeventhubs.NewProcessor(c.consumerClient, c.consumerCheckpoint, opts)
	if err != nil {
		return ErrGenericErrorWrap("creating eventhub processor", err)
	}

	go c.dispatchPartitionClients(processor)

	c.log.Info().Dict("details", zerolog.Dict().Str("module", "processor")).Msgf("started")

	if err := processor.Run(ctx); err != nil {
		c.log.Err(err).Dict("details", zerolog.Dict().Str("module", "processor")).Msg("processor exited")
	}

	return nil
}
func (c *Consumer) Stop(ctx context.Context) error {
	var err error
	c.cancel()

	if c.consumerClient != nil {
		err = c.consumerClient.Close(ctx)
		c.log.Info().Dict("details", zerolog.Dict()).Msgf("stopped")
	}

	return err
}

func (c *Consumer) dispatchPartitionClients(processor *azeventhubs.Processor) {
	for {
		pc := processor.NextPartitionClient(*c.ctx)
		if pc == nil {
			// Processor has stopped
			c.log.Info().Dict("details", zerolog.Dict().Str("module", "processor")).Msgf("stopped")
			break
		}

		go func() {
			if err := c.processEventsForPartition(pc); err != nil {
				// TODO: work on recovery strategy
				c.log.Err(err).Dict("details", zerolog.Dict().
					Str("partition", pc.PartitionID()).
					Str("module", "processor"),
				).Msg("error consuming from partition")
				c.cancel()

				return
			}
		}()
	}
}

func (c *Consumer) processEventsForPartition(partitionClient *azeventhubs.ProcessorPartitionClient) error {
	// 1. [BEGIN] Initialize any partition specific resources for your application.
	// 2. [CONTINUOUS] Loop, calling ReceiveEvents() and UpdateCheckpoint().
	// 3. [END] Cleanup any resources.
	defer func() {
		// 3/3 [END] Do cleanup here, like shutting down database clients
		// or other resources used for processing this partition.
		c.shutdownPartitionResources(partitionClient)
	}()

	// 1/3 [BEGIN] Initialize any partition specific resources for your application.
	if err := c.initializePartitionResources(); err != nil {
		return ErrGenericErrorWrap(fmt.Sprintf("initializing partition for partition id %s", partitionClient.PartitionID()), err)
	}

	// 2/3 [CONTINUOUS] Receive events, checkpointing as needed using UpdateCheckpoint.
	for {
		// Wait up to a 10 Seconds for MAX_EVENTS events, otherwise returns whatever we collected during that time.
		receiveCtx, cancelReceive := context.WithTimeout(context.TODO(), time.Second*10)
		events, err := partitionClient.ReceiveEvents(receiveCtx, MAX_EVENTS, nil)

		cancelReceive()

		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			var eventHubError *azeventhubs.Error

			if errors.As(err, &eventHubError) && eventHubError.Code == azeventhubs.ErrorCodeOwnershipLost {
				return nil
			}

			return ErrGenericErrorWrap("receiving events from partition", err)
		}

		if len(events) == 0 {
			continue
		}

		var msgs pipeline.Messages

		for _, event := range events {
			msgs = append(msgs, event.Body)
		}

		// Metrics
		c.met.CounterAddN(METRICS_EVENTHUB_RECEIVED_EVENTS, uint64(len(events)))

		if c.pipe != nil {
			// Pipeline no longer resports an error , instead uses the health channel to communicate with monitor to take
			// preventive actions
			c.pipe.Next(&msgs)

			// Updates the checkpoint with the latest event received. If processing needs to restart
			// it will restart from this point, automatically.
			if err := partitionClient.UpdateCheckpoint(context.TODO(), events[len(events)-1], nil); err != nil {
				return ErrGenericErrorWrap("updating checkpoing", err)
			}
		}
	}
}

func (c *Consumer) initializePartitionResources() error {
	// initialize things that might be partition specific, like a
	// database connection.
	return nil
}

func (c *Consumer) shutdownPartitionResources(partitionClient *azeventhubs.ProcessorPartitionClient) {
	// Each PartitionClient holds onto an external resource and should be closed if you're
	// not processing them anymore.

	partitionClient.Close(context.TODO())
}

func (c *Consumer) Name() string {
	return c.name
}
