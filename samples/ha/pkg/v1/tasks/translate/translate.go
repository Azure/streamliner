/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	config "github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"
	adx "github.com/Azure/streamliner/samples/ha/pkg/v1/logger/adxIngest"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/metrics"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/rs/zerolog"
	"github.com/valyala/fastjson"
)

var ErrTranslation = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("translate %w : %s", ErrTranslation, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("translate %s : %w", text, err)
}

const (
	idMappingPattern                                      = `(?m)^\"?\/providers\/microsoft\.idmapping\/aliases\/default\/namespaces\/microsoft\.compute\/types\/virtualmachines\/identifiers\/vmssidorvmid\/values\/([\w-]+)\/providers\/microsoft.maintenance\/scheduledevents\/([\w-]+)\"?$`
	METRICS_TRANSLATOR_EVENTHUB_RECEIVED_MESSAGES         = "translator_eventhub_received_messages"
	METRICS_TRANSLATOR_EVENTHUB_CURRENT_RECEIVED_MESSAGES = "translator_eventhub_current_received_messages"
	METRICS_TRANSLATOR_JSON_PARSED_MESSAGES               = "translator_json_parsed_messages"
	METRICS_TRANSLATOR_TRANSLATION_ERRORS                 = "translator_translation_errors"
	METRICS_TRANSLATOR_TOTAL_SCHEDULED_EVENTS_FOUND       = "translator_total_scheduled_events_found"
	METRICS_TRANSLATOR_TOTAL_SCHEDULED_EVENTS_TRANSLATED  = "translator_total_scheduled_events_translated"
	METRICS_TRANSLATOR_SCHEDULE_EVENT_EXPIRED             = "translator_total_schedule_events_expired"
	// TRANSACTION LOGS
)

var (
	transaction_logs_tablename = "transaction_logs_v2"
)

type Task struct {
	config         *config.Translate
	log            *zerolog.Logger
	pool           fastjson.ParserPool
	cache          cache.Cache
	pipe           pipeline.Pipeline
	name           string
	idMappingRegex *regexp.Regexp
	healthChannel  chan *healthstream.Message
	arenaPool      fastjson.ArenaPool
	loggingStopCh  chan bool
	loggingStarted bool
	loggingCh      chan ADXLog
	// for ADX logs
	HostName    string
	Environment string
	Role        string
	Region      string
	// Metrics
	met                *metrics.Metrics
	adxClient          *kusto.Client
	adxTransactionLogs *adx.AzureDataExplorer
}

type ADXLog struct {
	Original   string `json:"original"`
	Translated string `json:"translated"`
	Timestamp  string `json:"timestamp" kusto:"type:datetime"`
	Host       string `json:"host"`
	Env        string `json:"env" kusto:"name:env"`
	Role       string `json:"role"`
	Region     string `json:"region"`
}

type translateError struct {
	err        error
	vmUniqueID string
}

type TaskOption func(o *Task)

func Config(config *config.Bootstrap) TaskOption {
	return func(t *Task) {
		t.config = &config.Translate
		t.Environment = config.Environment
		t.HostName, _ = os.Hostname()
		t.Role = config.Monitor.Role
		t.Region = config.Monitor.Region
	}
}

func Logger(log *zerolog.Logger) TaskOption {
	return func(t *Task) { t.log = log }
}

func Cache(c cache.Cache) TaskOption {
	return func(t *Task) { t.cache = c }
}

func Name(name string) TaskOption {
	return func(t *Task) { t.name = name }
}

func Next(next pipeline.Pipeline) TaskOption {
	return func(t *Task) { t.pipe = next }
}

func Metrics(met *metrics.Metrics) TaskOption {
	return func(t *Task) { t.met = met }
}

func HealthChannel(healthCh chan *healthstream.Message) TaskOption {
	return func(t *Task) { t.healthChannel = healthCh }
}

func AdxClient(client *kusto.Client) TaskOption {
	return func(t *Task) { t.adxClient = client }
}

func New(opts ...TaskOption) (*Task, error) {
	var log zerolog.Logger

	task := &Task{
		loggingStopCh:  make(chan bool),
		loggingStarted: false,
		loggingCh:      make(chan ADXLog),
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
		return nil, ErrGenericError("cannot start translate without a configuration")
	}

	r := regexp.MustCompile(idMappingPattern)
	task.idMappingRegex = r

	// Add transaction logs
	adxTransactionLogs, err := adx.New(
		adx.Config(&task.config.Logging),
		adx.CreateTables(true),
		adx.IngestionType(adx.QueuedIngestion),
		adx.KustoClient(task.adxClient),
		adx.Name("transactionlog-Ingestor"),
		adx.Logger(task.log),
		adx.TableName(&transaction_logs_tablename),
		adx.DataSample(ADXLog{}),
	)
	if err != nil {
		return nil, ErrGenericErrorWrap("starting adx transaction logs", err)
	}

	task.adxTransactionLogs = adxTransactionLogs

	return task, nil
}

func (t *Task) Name() string {
	return t.name
}

func (t *Task) Start(ctx context.Context) error {
	t.log.Info().Dict("details", zerolog.Dict()).Msg("started")
	go t.startLogging(ctx)

	return nil
}

func (t *Task) Stop(ctx context.Context) error {
	t.log.Info().Dict("details", zerolog.Dict()).Msg("stopped")

	if t.loggingStarted {
		t.loggingStopCh <- true
	}

	return nil
}

func (t *Task) startLogging(_ context.Context) {
	t.loggingStarted = true
	defer func() {
		t.loggingStarted = false
	}()

	t.log.Info().Dict("details", zerolog.Dict().
		Str("module", "logging"),
	).Msg("started")

	for {
		select {
		case msg := <-t.loggingCh:
			msgStr, err := json.Marshal(msg)
			if err != nil {
				t.log.Err(err).Dict("details", zerolog.Dict().
					Str("module", "translate-logging"),
				).Msg("cannot unmarshal")

				continue
			}

			if _, err := t.adxTransactionLogs.Write(msgStr); err != nil {
				t.log.Err(err).Dict("details", zerolog.Dict().
					Str("module", "translate-logging"),
				).Msg("writing to adx")
			}

		case <-t.loggingStopCh:
			t.log.Info().Dict("details", zerolog.Dict().
				Str("module", "translate-logging"),
			).Msg("exit")

			return
		}
	}
}

func (t *Task) Next(msgs *pipeline.Messages) {
	parser := t.pool.Get()

	translatedMsgs := pipeline.Messages{}
	totalMsgs := 0

	// Metrics
	t.met.CounterAddN(METRICS_TRANSLATOR_EVENTHUB_RECEIVED_MESSAGES, uint64(len(*msgs)))
	t.met.GaugeSet(METRICS_TRANSLATOR_EVENTHUB_CURRENT_RECEIVED_MESSAGES, int64(len(*msgs)))

	for _, msg := range *msgs {
		jsonMsg, err := parser.Parse(msg.String())
		if err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict().
				Str("module", "pipeline").
				Str("message", msg.String()),
			).Msg("error parsing json")

			continue
		}

		jsonEvents, err := jsonMsg.Array()

		if err != nil {
			t.log.Err(err).Dict("details", zerolog.Dict().
				Str("module", "pipeline").
				Str("message", jsonMsg.String()),
			).Msg("error extracting array from json")

			continue
		}

		totalMsgs += len(jsonEvents)

		t.met.CounterAddN(METRICS_TRANSLATOR_JSON_PARSED_MESSAGES, uint64(len(jsonEvents)))

		arena := t.arenaPool.Get()
		aa := arena.NewArray()
		idx := 0
		errs := []translateError{}

		for _, scheduleEvent := range jsonEvents {
			// Save original message bytes
			originSEBuf := scheduleEvent.MarshalTo(nil)

			// Should skip
			if t.isScheduleEventExpired(scheduleEvent) != nil {
				id := scheduleEvent.Get("id").MarshalTo(nil)
				id = id[1 : len(id)-1]

				t.log.Err(err).Dict("details", zerolog.Dict().
					Str("module", "translate").
					Bytes("eventId", id).
					Int("index", idx).
					Int("length", len(jsonEvents)),
				).Msg("event expired")

				t.met.CounterAdd(METRICS_TRANSLATOR_SCHEDULE_EVENT_EXPIRED)

				continue
			}

			t.traverseAndTranslate(scheduleEvent, "root", &errs)

			if len(errs) != 0 {
				t.log.Err(err).Dict("details", zerolog.Dict().
					Str("module", "translate").
					Str("message", jsonMsg.String()),
				).Msg("cannot translate this message, skipping")

				t.met.CounterAdd(METRICS_TRANSLATOR_TRANSLATION_ERRORS)

				continue
			}
			// Remove fields
			if data := scheduleEvent.Get("data"); data != nil {
				data.Del("homeTenantId")
				data.Del("resourceHomeTeantId")
			}

			// Add element in array after translation
			aa.SetArrayItem(idx, scheduleEvent)
			idx++

			// Log messages to ADX
			if t.config.Logging.Enabled {
				translatedSEBuf := scheduleEvent.MarshalTo(nil)
				logEntry := ADXLog{
					Original:   string(originSEBuf),
					Translated: string(translatedSEBuf),
					Timestamp:  time.Now().UTC().Format(time.RFC3339),
					Host:       t.HostName,
					Env:        t.Environment,
					Role:       t.Role,
					Region:     t.Region,
				}
				t.loggingCh <- logEntry
			}
		}
		// Dont add empty schedule events, this happens when the translation fails
		if aa.String() != "[]" {
			translatedMsgBuf := aa.MarshalTo(nil)
			translatedMsgs = append(translatedMsgs, translatedMsgBuf)
		}

		t.arenaPool.Put(arena)
	}

	t.pool.Put(parser)
	t.met.CounterAddN(METRICS_TRANSLATOR_TOTAL_SCHEDULED_EVENTS_FOUND, uint64(totalMsgs))

	if len(translatedMsgs) == 0 {
		t.log.Err(ErrGenericError("no messages to process")).Dict("details", zerolog.Dict().
			Str("module", "translate").
			Int("len", len(translatedMsgs)),
		).Msg("translation map empty")
	} else if t.pipe != nil { // Send to next in pipeline
		// Send messages to next in the pipeline, errors are reported to the health channel for preventive actions of the entire pipeline.

		t.met.CounterAddN(METRICS_TRANSLATOR_TOTAL_SCHEDULED_EVENTS_TRANSLATED, uint64(len(translatedMsgs)))
		t.pipe.Next(&translatedMsgs)
	}
}

func (t *Task) traverseAndTranslate(value *fastjson.Value, prefix string, errs *[]translateError) {
	switch value.Type() {
	case fastjson.TypeObject:
		object := value.GetObject()

		object.Visit(func(key []byte, v *fastjson.Value) {
			newPrefix := fmt.Sprintf("%s.%s", prefix, key)

			if v.Type() == fastjson.TypeString {
				vmUniqueID, err := t.matchAndTranslateIDMapping(object, string(key))
				if err != nil {
					t.log.Err(err).Dict("details", zerolog.Dict().
						Str("module", "translate").
						Str("vmUniqueID", vmUniqueID),
					).Msg("json parser failed, cannot find vm with uniqueId")
					*errs = append(*errs, translateError{
						err:        err,
						vmUniqueID: vmUniqueID,
					})
					return
				}
			}
			t.traverseAndTranslate(v, newPrefix, errs)
		})

		return
	case fastjson.TypeArray:
		array := value.GetArray()
		for i, v := range array {
			newPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			t.traverseAndTranslate(v, newPrefix, errs)
		}
	case fastjson.TypeFalse:
	case fastjson.TypeNull:
	case fastjson.TypeNumber:
	case fastjson.TypeString:
	case fastjson.TypeTrue:
	default:
	}
}

// matchAndTranslateIDMapping uses regex to identify idmappings and extract the vmUniqueId
// then it queries the cache for the translation.
func (t *Task) matchAndTranslateIDMapping(object *fastjson.Object, key string) (string, error) {
	var vmUniqueID string

	var guid string

	value := object.Get(key)
	match := t.idMappingRegex.FindStringSubmatch(value.String())

	if len(match) == 3 {
		vmUniqueID = match[1]
		guid = match[2]

		rec := t.cache.Get(vmUniqueID)

		if rec.VMScaleSetResourceID == "" {
			return vmUniqueID, ErrGenericError(fmt.Sprintf("vmUniqueId %s not found", vmUniqueID))
		}

		replaceStr := fmt.Sprintf("\"%s/providers/microsoft.maintenance/scheduledevents/%s\"", rec.VMScaleSetResourceID, guid)
		object.Set(key, fastjson.MustParse(replaceStr))
	}

	return vmUniqueID, nil
}

func (t *Task) isScheduleEventExpired(se *fastjson.Value) error {
	eventTime := se.Get("eventTime").MarshalTo(nil)
	eTime, err := time.Parse(time.RFC3339, string(eventTime[1:len(eventTime)-1]))

	if err != nil {
		return ErrGenericError("cannot extract eventTime")
	}

	if eTime.Add(t.config.MessageExpiration).Before(time.Now()) {
		return ErrGenericError("event expired")
	}

	return nil
}
