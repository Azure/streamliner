/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package adxingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableMappingName(t *testing.T) {
	type DemoStruct struct {
	}

	output := getTableName(DemoStruct{})
	assert.Equal(t, *output, "DemoStruct")
}

func TestTableMappingStatementSimple(t *testing.T) {
	type DemoStruct struct {
		name string `kusto:"column:name;type:string"`
	}

	expectedOutput := `.create-or-alter table ['DemoStruct'] ingestion json mapping 'DemoStruct_mapping' '[{"column": "name","Properties":{"Path":"$.name", "Transform":""}}]';`
	stmt := CreateTableMappings(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableMappingNoTags(t *testing.T) {
	type DemoStruct struct {
		name string
	}

	expectedOutput := `.create-or-alter table ['DemoStruct'] ingestion json mapping 'DemoStruct_mapping' '[{"column": "name","Properties":{"Path":"$.name", "Transform":""}}]';`
	stmt := CreateTableMappings(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableMappingMissingDataType(t *testing.T) {
	type DemoStruct struct {
		name string `kusto:"column:name;"`
	}

	expectedOutput := `.create-or-alter table ['DemoStruct'] ingestion json mapping 'DemoStruct_mapping' '[{"column": "name","Properties":{"Path":"$.name", "Transform":""}}]';`
	stmt := CreateTableMappings(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableMappingMissingColumName(t *testing.T) {
	type DemoStruct struct {
		id string `kusto:"type:int64;"`
	}

	expectedOutput := `.create-or-alter table ['DemoStruct'] ingestion json mapping 'DemoStruct_mapping' '[{"column": "id","Properties":{"Path":"$.id", "Transform":""}}]';`
	stmt := CreateTableMappings(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableMappingBadFormatMissingComma(t *testing.T) {
	type DemoStruct struct {
		id string `kusto:"type:int64"`
	}

	expectedOutput := `.create-or-alter table ['DemoStruct'] ingestion json mapping 'DemoStruct_mapping' '[{"column": "id","Properties":{"Path":"$.id", "Transform":""}}]';`
	stmt := CreateTableMappings(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableMappingWithTransform(t *testing.T) {
	type DemoStruct struct {
		id int64 `kusto:"type:datetime;transform:DateTimeFromUnixSeconds"`
	}

	expectedOutput := `.create-or-alter table ['DemoStruct'] ingestion json mapping 'DemoStruct_mapping' '[{"column": "id","Properties":{"Path":"$.id", "Transform":"DateTimeFromUnixSeconds"}}]';`
	stmt := CreateTableMappings(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}
