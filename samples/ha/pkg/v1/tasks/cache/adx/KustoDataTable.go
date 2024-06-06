/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package cache

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Azure/azure-kusto-go/kusto/data/value"
)

type DataTable struct {
	TableID   string          `json:"TableId"`
	TableKind string          `json:"TableKind"`
	TableName string          `json:"TableName"`
	Columns   []string        `json:"Columns"`
	Rows      [][]interface{} `json:"Rows"`
	_         [8]byte         // Padding to align the struct to 64 bytes
}

type DataTableRow struct {
	Timestamp        time.Time `json:"Timestamp"`
	ClientRequestID  string    `json:"ClientRequestId"`
	ActivityID       string    `json:"ActivityId"`
	SubActivityID    string    `json:"SubActivityId"`
	ParentActivityID string    `json:"ParentActivityId"`
	LevelName        string    `json:"LevelName"`
	StatusCodeName   string    `json:"StatusCodeName"`
	EventTypeName    string    `json:"EventTypeName"`
	Payload          string    `json:"Payload"`
	Level            int       `json:"Level"`
	StatusCode       int       `json:"StatusCode"`
	EventType        int       `json:"EventType"`
}

type QueryCompletionInformation struct {
	ResourceUsage             interface{} `json:"resource_usage" kusto:"column:resource_usage;type:dynamic"`
	InputDatasetStatistics    interface{} `json:"input_dataset_statistics" kusto:"column:input_dataset_statistics;type:dynamic"`
	DatasetStatistics         interface{} `json:"dataset_statistics" kusto:"column:dataset_statistics;type:dynamic"`
	CrossClusterResourceUsage interface{} `json:"cross_cluster_resource_usage" kusto:"column:cross_cluster_resource_usage;type:dynamic"`
	Timestamp                 string      `json:"timestamp" kusto:"type:datetime"`
	Host                      string      `json:"host"`
	Env                       string      `json:"env" kusto:"name:env"`
	Role                      string      `json:"role"`
	Region                    string      `json:"region"`
}

func (q *QueryCompletionInformation) GetBytes() ([]byte, error) {
	var qcBytes []byte

	qcBytes, err := json.Marshal(q)
	if err != nil {
		return nil, ErrGenericErrorWrap("marshalling query completion information", err)
	}

	return qcBytes, nil
}

func (q *DataTableRow) Decode(v value.Values) error {
	var err error

	for z, col := range v {
		switch z {
		case 0:
			timestamp, ok := col.(value.DateTime)
			if !ok {
				return fmt.Errorf("unable to unmarshal column %d into a DateTime value: %s", z, col.String())
			}

			q.Timestamp = timestamp.Value
		case 1:
			q.ClientRequestID = col.String()
		case 2:
			q.ActivityID = col.String()
		case 3:
			q.SubActivityID = col.String()
		case 4:
			q.ParentActivityID = col.String()
		case 5:
			q.Level, err = strconv.Atoi(col.String())
			if err != nil {
				return fmt.Errorf("unable to convert column %d into an int value: %s", z, col.String())
			}
		case 6:
			q.LevelName = col.String()
		case 7:
			q.StatusCode, err = strconv.Atoi(col.String())
			if err != nil {
				return fmt.Errorf("unable to convert column %d into an int value: %s", z, col.String())
			}
		case 8:
			q.StatusCodeName = col.String()
		case 9:
			q.EventType, err = strconv.Atoi(col.String())
			if err != nil {
				return fmt.Errorf("unable to convert column %d into an int value: %s", z, col.String())
			}
		case 10:
			q.EventTypeName = col.String()
		case 11:
			q.Payload = col.String()
		default:
			return fmt.Errorf("unable to unmarshal column %d into an int value: %s", z, col.String())
		}
	}

	return nil
}
