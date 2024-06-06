/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	adxconnection "github.com/Azure/streamliner/samples/ha/pkg/adxConnection"
	"github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/tasks/cache/store"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/metrics"

	adx "github.com/Azure/streamliner/samples/ha/pkg/v1/logger/adxIngest"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/kql"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/avast/retry-go"
	"github.com/rs/zerolog"
)

var ErrCache = errors.New("cache error")

func ErrGenericError(text string) error {
	return fmt.Errorf("%w : %s", ErrCache, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("%w : %s", err, text)
}

func ErrCombineErrorWrap(text string, err1, err2 error) error {
	return fmt.Errorf("%w : %w : %s", err1, err2, text)
}

// ErrCreatingKustoConnection represents an error when creating a Kusto connection.
var ErrCreatingKustoConnection = errors.New("creating kusto connection")

// ErrUpdatingCache represents an error when updating the cache.
var ErrUpdatingCache = errors.New("updating cache")

var (
	cacheLogsTablename = "cache_logs_v2"
)

const (
	MetricsCacheRecordsReceived   = "cache_records_received"
	MetricsCacheUpdateTime        = "cache_update_time"
	MetricsCacheUpdateError       = "cache_update_errors"
	MetricsCacheExtractValueError = "cache_update_extract_value_errors"
)

type Cache struct {
	store          store.Store
	ctx            context.Context
	met            *metrics.Metrics
	config         *config.Translate
	stop           chan bool
	adxLogClient   *kusto.Client
	healthChannel  chan *healthstream.Message
	log            *zerolog.Logger
	client         *kusto.Client
	adxCacheLogs   *adx.AzureDataExplorer
	region         string
	name           string
	hostname       string
	env            string
	role           string
	retryOptions   []retry.Option
	UpdateInterval time.Duration
}

type TaskOption func(t *Cache)

func Config(conf *config.Translate) TaskOption {
	return func(t *Cache) { t.config = conf }
}

func Logger(log *zerolog.Logger) TaskOption {
	return func(t *Cache) { t.log = log }
}

func Name(name string) TaskOption {
	return func(t *Cache) { t.name = name }
}

func WithStore(s store.Store) TaskOption {
	return func(t *Cache) { t.store = s }
}

func HealthChannel(healthCh chan *healthstream.Message) TaskOption {
	return func(t *Cache) { t.healthChannel = healthCh }
}

func Metrics(met *metrics.Metrics) TaskOption {
	return func(t *Cache) { t.met = met }
}

func AdxLogClient(client *kusto.Client) TaskOption {
	return func(t *Cache) { t.adxLogClient = client }
}

func Environment(name string) TaskOption {
	return func(t *Cache) { t.env = name }
}

func Region(name string) TaskOption {
	return func(t *Cache) { t.region = name }
}

func Role(name string) TaskOption {
	return func(t *Cache) { t.role = name }
}

func Context(c context.Context) TaskOption {
	return func(t *Cache) { t.ctx = c }
}

// NewCache creates a new Cache with the necessary fields initialized.
func NewCache(options ...TaskOption) *Cache {
	hostname, _ := os.Hostname()

	c := &Cache{
		stop:     make(chan bool, 1),
		hostname: hostname,
	}
	for _, option := range options {
		option(c)
	}

	return c
}

func New(opts ...TaskOption) (*Cache, error) {
	var log zerolog.Logger

	var err error

	srv := NewCache(opts...)

	if srv.name == "" {
		log = srv.log.With().Str("task", "task").Logger()
	} else {
		log = srv.log.With().Str("task", srv.name).Logger()
	}

	srv.log = &log
	srv.UpdateInterval = srv.config.ADX.UpdateFrequency

	err = srv.setupadxCacheLogs()
	if err != nil {
		return nil, ErrGenericErrorWrap("setting up adx cache exec logs", err)
	}

	err = srv.Initialize()
	if err != nil {
		return nil, ErrGenericErrorWrap("initializing cache", err)
	}

	err = srv.UpdateCacheFromAdx(srv.ctx)
	if err != nil {
		return nil, ErrGenericErrorWrap("updating cache from adx", err)
	}

	return srv, nil
}

func (t *Cache) Initialize() error {
	err := t.setupKustoClient()
	if err != nil {
		return err
	}

	t.setupRetryOptions()

	return nil
}

func (t *Cache) setupKustoClient() error {
	kustoConnectionString := t.buildKustoConnectionString()

	c, err := t.buildHTTPClient(kustoConnectionString)
	if err != nil {
		return ErrGenericErrorWrap("creating kusto client", err)
	}

	t.client = c

	return nil
}

func (t *Cache) buildHTTPClient(kcsb *kusto.ConnectionStringBuilder) (*kusto.Client, error) {
	c := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:    0,
			IdleConnTimeout: 0,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	client, err := kusto.New(kcsb, kusto.WithHttpClient(c))
	if err != nil {
		return nil, ErrGenericErrorWrap("creating kusto client", err)
	}

	return client, nil
}

func (t *Cache) buildKustoConnectionString() *kusto.ConnectionStringBuilder {
	kustoConnectionString := adxconnection.NewConnectionStringbuilder(t.config.ADX.Endpoint)

	// TEM Path to increate retry options when crossing regions
	kustoConnectionString.AttachPolicyClientOptions(&azcore.ClientOptions{
		Retry: policy.RetryOptions{
			MaxRetries: 5,
			RetryDelay: 5 * time.Second,
			StatusCodes: []int{
				http.StatusRequestTimeout,      // 408
				http.StatusTooManyRequests,     //     429
				http.StatusInternalServerError, // 500
				http.StatusBadGateway,          //          502
				http.StatusServiceUnavailable,  //  503
				http.StatusGatewayTimeout,      //      504
			},
		},
	})

	return kustoConnectionString
}

func (t *Cache) setupRetryOptions() {
	t.retryOptions = []retry.Option{
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(t.config.ADX.RetryOptions.Delay), // Initial Delay
		retry.MaxJitter(t.config.ADX.RetryOptions.MaxJitter),
		retry.Attempts(t.config.ADX.RetryOptions.Attempts),
		retry.OnRetry(func(n uint, err error) {
			t.log.Err(err).Dict("details", zerolog.Dict().
				Str("module", "update").
				Uint("retry", n)).
				Msg("retrying request")
		}),
	}
}

func (t *Cache) setupadxCacheLogs() error {
	adxCacheLogs, err := adx.New(
		adx.Config(&t.config.Logging),
		adx.CreateTables(true),
		adx.IngestionType(adx.QueuedIngestion),
		adx.KustoClient(t.adxLogClient),
		adx.Name("cachelog-Ingestor"),
		adx.Logger(t.log),
		adx.TableName(&cacheLogsTablename),
		adx.DataSample(QueryCompletionInformation{}),
	)
	if err != nil {
		return ErrGenericErrorWrap("starting adx cache logs", err)
	}

	t.adxCacheLogs = adxCacheLogs

	return nil
}

func (t *Cache) UpdateCacheFromAdx(ctx context.Context) error {
	defer t.timer()()

	query := kql.New("WMTWarmup")

	// retry logic
	err := retry.Do(func() error {
		callCtx, cancel := context.WithTimeout(ctx, t.config.ADX.UpdateFrequency)
		defer cancel()

		rows, err := t.queryAdx(callCtx, query)
		if err != nil {
			return err
		}
		defer rows.Stop()

		err = t.processRows(rows)
		if err != nil {
			return err
		}

		err = t.swapTemporaryStore()
		if err != nil {
			return err
		}

		return nil
	}, t.retryOptions...)

	return err
}

func (t *Cache) queryAdx(ctx context.Context, query *kql.Builder) (*kusto.RowIterator, error) {
	rows, err := t.client.Query(ctx, t.config.ADX.Database, query,
		kusto.DeferPartialQueryFailures(),
		kusto.NoTruncation(),
		kusto.ServerTimeout(t.config.ADX.UpdateFrequency),
		kusto.Application(t.hostname),
		kusto.TruncationMaxRecords(2000000),
		kusto.TruncationMaxSize(2000000000),
	)
	if err != nil {
		return nil, ErrGenericErrorWrap("getting rows from adx", err)
	}

	return rows, nil
}

func (t *Cache) processRows(rows *kusto.RowIterator) error {
	for {
		row, inlineErr, e := rows.NextRowOrError()
		if e != nil {
			if e == io.EOF {
				break
			}

			return t.handleRowError(e)
		}

		if inlineErr != nil {
			return t.handleRowError(inlineErr)
		}

		if err := t.processRow(row); err != nil {
			return err
		}
	}

	if err := t.LogQueryCompletionInfo(rows); err != nil {
		return t.handleRowError(err)
	}

	return nil
}

func (t *Cache) processRow(row *table.Row) error {
	rec, extractErr := t.ExtractRecord(row)
	if extractErr != nil {
		return t.handleRowError(extractErr)
	}

	if addErr := t.store.AddRecordToTemporaryStore(rec); addErr != nil {
		return t.handleRowError(addErr)
	}

	return nil
}

func (t *Cache) handleRowError(err error) error {
	t.met.GaugeSet(MetricsCacheRecordsReceived, 0)

	t.store.DiscardTemporaryStore()

	return ErrGenericErrorWrap("processing row", err)
}

func (t *Cache) swapTemporaryStore() error {
	err := t.store.SwapWithTemporaryStore()
	if err != nil {
		t.store.DiscardTemporaryStore()
		return ErrGenericErrorWrap("swapping records", err)
	}

	t.met.GaugeSet(MetricsCacheRecordsReceived, int64(t.store.GetStoreSize()))

	return nil
}

func (t *Cache) LogQueryCompletionInfo(rows *kusto.RowIterator) error {
	info, err := rows.GetQueryCompletionInformation()
	if err != nil {
		return ErrGenericErrorWrap("getting query completion information", err)
	}

	if info.TableKind == "QueryCompletionInformation" {
		rows := info.KustoRows
		for _, row := range rows {
			var dtRow DataTableRow

			err := dtRow.Decode(row)
			if err != nil {
				return ErrGenericErrorWrap("decoding row", err)
			}

			if dtRow.EventTypeName == "QueryResourceConsumption" {
				err := t.LogResourceConsumptionInfo(&dtRow, t.adxCacheLogs)
				if err != nil {
					return ErrGenericErrorWrap("logging query resource consumption", err)
				}
			}
		}
	}

	return nil
}

func (t *Cache) DecodeQCInfo(payload string) (*QueryCompletionInformation, error) {
	qc := &QueryCompletionInformation{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Host:      t.hostname,
		Env:       t.env,
		Role:      t.role,
		Region:    t.region,
	}

	err := json.Unmarshal([]byte(payload), qc)
	if err != nil {
		fmt.Println(payload)
		return nil, err
	}

	return qc, nil
}

func (t *Cache) LogResourceConsumptionInfo(dtRow *DataTableRow, cluster *adx.AzureDataExplorer) error {
	qc, err := t.DecodeQCInfo(dtRow.Payload)
	if err != nil {
		return ErrGenericErrorWrap("decoding query completion information", err)
	}

	qcBytes, err := qc.GetBytes()
	if err != nil {
		return ErrGenericErrorWrap("getting query completion information bytes", err)
	}

	_, err = cluster.Write(qcBytes)
	if err != nil {
		return ErrGenericErrorWrap("writing query completion information", err)
	}

	return nil
}

func (t *Cache) Start(ctx context.Context) error {
	t.log.Info().Dict("details", zerolog.Dict()).Msg("started")
	ticker := time.NewTicker(t.UpdateInterval)

	for {
		select {
		case <-ticker.C:
			ticker.Stop()

			if err := t.UpdateCacheFromAdx(ctx); err != nil {
				t.log.Err(err).Dict("details", zerolog.Dict()).Msg("updating cache")
			}

			ticker = time.NewTicker(t.UpdateInterval)
		case <-t.stop:
			ticker.Stop()
			return nil
		}
	}
}

func (t *Cache) Stop(ctx context.Context) error {
	t.stop <- true

	t.log.Info().Dict("details", zerolog.Dict()).Msg("stopped")

	return nil
}

func (t *Cache) ExtractRecord(row *table.Row) (cache.Data, error) {
	rec, err := t.ExtractValuesToCacheData(row)
	if err != nil {
		return cache.Data{}, ErrGenericErrorWrap("extracting values to cache data", err)
	}

	return rec, nil
}

func (t *Cache) ExtractValuesToCacheData(row *table.Row) (cache.Data, error) {
	rec := cache.Data{}

	if err := row.ExtractValues(&rec.PreciseTimeStamp, &rec.VMId, &rec.VMScaleSetResourceID, &rec.Vmtype); err != nil {
		t.met.CounterAdd(MetricsCacheExtractValueError)

		return cache.Data{}, ErrGenericErrorWrap("cannot extract values", err)
	}

	return rec, nil
}

func (t *Cache) timer() func() {
	start := time.Now()

	return func() {
		t.met.GaugeSet(MetricsCacheUpdateTime, time.Since(start).Microseconds())
	}
}

func (t *Cache) Name() string {
	return t.name
}

func (t *Cache) Get(key string) cache.Data {
	return t.store.Get(key)
}
