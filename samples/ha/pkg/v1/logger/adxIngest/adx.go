/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package adxingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-kusto-go/kusto"
	kustoerrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/avast/retry-go"
	"github.com/rs/zerolog"

	"github.com/Azure/streamliner/samples/ha/pkg/config"
)

var ErrADX = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("adxIngest %w : %s", ErrADX, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("adxIngest %w : %s", err, text)
}

const (
	azureClientID           = "AZURE_CLIENT_ID"
	azureFederatedTokenFile = "AZURE_FEDERATED_TOKEN_FILE"
	azureTenantID           = "AZURE_TENANT_ID"
)

type AzureDataExplorer struct {
	log           *zerolog.Logger
	createTables  bool
	ingestionType string
	kustoClient   *kusto.Client
	ingestors     map[string]ingest.Ingestor
	config        *config.ADXLogging
	name          string
	table         kusto.Statement
	tableMapping  kusto.Statement
	tableName     *string
	dataSample    interface{}
	retryOptions  []retry.Option
}

type AdxOption func(o *AzureDataExplorer)

func Config(config *config.ADXLogging) AdxOption {
	return func(a *AzureDataExplorer) {
		a.config = config
	}
}

func Logger(log *zerolog.Logger) AdxOption {
	return func(a *AzureDataExplorer) { a.log = log }
}

func Name(name string) AdxOption {
	return func(a *AzureDataExplorer) { a.name = name }
}

func KustoClient(client *kusto.Client) AdxOption {
	return func(a *AzureDataExplorer) { a.kustoClient = client }
}

func CreateTables(flag bool) AdxOption {
	return func(a *AzureDataExplorer) { a.createTables = flag }
}

func TableName(name *string) AdxOption {
	return func(a *AzureDataExplorer) { a.tableName = name }
}

func DataSample(data interface{}) AdxOption {
	return func(a *AzureDataExplorer) { a.dataSample = data }
}

func IngestionType(_type string) AdxOption {
	return func(a *AzureDataExplorer) {
		a.ingestionType = _type
	}
}

const (
	// These control the amount of memory we use when ingesting blobs
	bufferSize = 1 << 20 // 1 MiB
	maxBuffers = 5
)

const ManagedIngestion = "managed"
const QueuedIngestion = "queued"

func New(opts ...AdxOption) (*AzureDataExplorer, error) {
	var log zerolog.Logger

	adx := &AzureDataExplorer{
		ingestors: make(map[string]ingest.Ingestor),
	}

	for _, o := range opts {
		o(adx)
	}

	if adx.name == "" {
		log = adx.log.With().Str("task", "task").Logger()
	} else {
		log = adx.log.With().Str("task", adx.name).Logger()
	}

	adx.log = &log

	if adx.config == nil {
		return nil, ErrGenericError("cannot start adx ingestor without a configuration")
	}

	if adx.ingestionType != ManagedIngestion && adx.ingestionType != QueuedIngestion {
		return nil, ErrGenericError("unknown ingestion type")
	}

	if adx.tableName == nil {
		adx.tableName = getTableName(adx.dataSample)
	}

	adx.table = CreateTableStatement(adx.dataSample, adx.tableName)
	adx.tableMapping = CreateTableMappings(adx.dataSample, adx.tableName)

	adx.retryOptions = []retry.Option{
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(adx.config.RetryOptions.Delay), // Initial Delay
		retry.MaxJitter(adx.config.RetryOptions.MaxJitter),
		retry.Attempts(adx.config.RetryOptions.Attempts),
		retry.OnRetry(func(n uint, err error) {
			adx.log.Err(err).Dict("details", zerolog.Dict().
				Str("module", "adx-log-metrics").
				Uint("retry", n)).
				Msg("retrying request")
		}),
	}

	return adx, nil
}

// Clean up and close the ingestor
func (adx *AzureDataExplorer) Close() error {
	var errs []error

	for _, v := range adx.ingestors {
		if err := v.Close(); err != nil {
			// accumulate errors while closing ingestors
			errs = append(errs, err)
		}
	}

	if err := adx.kustoClient.Close(); err != nil {
		errs = append(errs, err)
	}

	adx.kustoClient = nil
	adx.ingestors = nil

	if len(errs) == 0 {
		adx.log.Info().Msg("Closed ingestors and client")
		return nil
	}
	// Combine errors into a single object and return the combined error
	return kustoerrors.GetCombinedError(errs...)
}

func (adx *AzureDataExplorer) Write(data []byte) (int, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, adx.config.Timeout)

	defer cancel()

	format := ingest.FileFormat(ingest.JSON)

	err := retry.Do(func() error {
		err := adx.pushData(ctx, format, *adx.tableName, data)
		if err != nil {
			return ErrGenericErrorWrap("pushing data to adx", err)
		}

		return nil
	}, adx.retryOptions...)

	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (adx *AzureDataExplorer) pushData(ctx context.Context, format ingest.FileOption, tableName string, data []byte) error {
	var metricIngestor ingest.Ingestor

	var err error

	metricIngestor, err = adx.getDataIngestor(ctx, tableName)
	if err != nil {
		return ErrGenericErrorWrap("getting data ingestor for adx", err)
	}

	reader := bytes.NewReader(data)
	mapping := ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", tableName), ingest.JSON)

	if metricIngestor != nil {
		if _, err := metricIngestor.FromReader(ctx, reader, format, mapping); err != nil {
			return ErrGenericError(fmt.Sprintf("sending ingestion request to Azure Data Explorer for table %q failed: %v", tableName, err))
		}
	}

	return nil
}

func (adx *AzureDataExplorer) getDataIngestor(ctx context.Context, tableName string) (ingest.Ingestor, error) {
	ingestor := adx.ingestors[tableName]

	if ingestor == nil {
		if err := adx.createAzureDataExplorerTable(ctx); err != nil {
			return nil, ErrGenericErrorWrap(fmt.Sprintf("creating table for %s failed", tableName), err)
		}

		// create a new ingestor client for the table
		tempIngestor, err := createIngestorByTable(adx.kustoClient, adx.config.Database, tableName, adx.ingestionType)
		if err != nil {
			return nil, ErrGenericErrorWrap(fmt.Sprintf("creating ingestor for %s failed", tableName), err)
		}

		adx.ingestors[tableName] = tempIngestor
		ingestor = tempIngestor
	}

	return ingestor, nil
}

func (adx *AzureDataExplorer) createAzureDataExplorerTable(ctx context.Context) error {
	if !adx.createTables {
		adx.log.Info().Msg("skipped table creation")
		return nil
	}

	if _, err := adx.kustoClient.Mgmt(ctx, adx.config.Database, adx.table); err != nil {
		return ErrGenericErrorWrap(fmt.Sprintf("creating table %s", adx.table), err)
	}

	if _, err := adx.kustoClient.Mgmt(ctx, adx.config.Database, adx.tableMapping); err != nil {
		return ErrGenericErrorWrap(fmt.Sprintf("creating table mapping %s", adx.tableMapping), err)
	}

	return nil
}

// For each table create the ingestor
func createIngestorByTable(client *kusto.Client, database, tableName, ingestionType string) (ingest.Ingestor, error) {
	switch strings.ToLower(ingestionType) {
	case ManagedIngestion:
		mi, err := ingest.NewManaged(client, database, tableName)
		if err != nil {
			return nil, ErrGenericErrorWrap("creating stream ingestion client", err)
		}

		return mi, nil

	case QueuedIngestion:
		qi, err := ingest.New(client, database, tableName, ingest.WithStaticBuffer(bufferSize, maxBuffers))
		if err != nil {
			return nil, ErrGenericErrorWrap("creating queue ingestion client", err)
		}

		return qi, nil
	}

	return nil, ErrGenericError(fmt.Sprintf("ingestion_type has to be one of %q or %q", ManagedIngestion, QueuedIngestion))
}

func NewConnectionStringbuilder(endpoint string) *kusto.ConnectionStringBuilder {
	var clientID, file, tenantID string

	useDefault := false
	ok := false

	if endpoint == "" {
		return nil
	}

	kustoConnectionStringBuilder := kusto.NewConnectionStringBuilder(endpoint)

	if clientID, ok = os.LookupEnv(azureClientID); !ok {
		useDefault = true
	}

	if file, ok = os.LookupEnv(azureFederatedTokenFile); !ok {
		useDefault = true
	}

	if tenantID, ok = os.LookupEnv(azureTenantID); !ok {
		useDefault = true
	}

	if useDefault {
		return kustoConnectionStringBuilder.WithDefaultAzureCredential()
	}

	return kustoConnectionStringBuilder.WithKubernetesWorkloadIdentity(clientID, file, tenantID)
}
