/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package adxingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableName(t *testing.T) {
	type DemoStruct struct {
	}

	output := getTableName(DemoStruct{})
	assert.Equal(t, *output, "DemoStruct")
}

func TestTableStatementSimple(t *testing.T) {
	type DemoStruct struct {
		name string `kusto:"column:name;type:string"`
	}

	expectedOutput := `.create-merge table ['DemoStruct'] (['name']:string);`
	stmt := CreateTableStatement(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableNoTags(t *testing.T) {
	type DemoStruct struct {
		name string
	}

	expectedOutput := `.create-merge table ['DemoStruct'] (['name']:string);`
	stmt := CreateTableStatement(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableMissingDataType(t *testing.T) {
	type DemoStruct struct {
		name string `kusto:"column:name;"`
	}

	expectedOutput := `.create-merge table ['DemoStruct'] (['name']:string);`
	stmt := CreateTableStatement(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableMissingColumName(t *testing.T) {
	type DemoStruct struct {
		id string `kusto:"type:int64;"`
	}

	expectedOutput := `.create-merge table ['DemoStruct'] (['id']:int64);`
	stmt := CreateTableStatement(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}

func TestTableBadFormatMissingComma(t *testing.T) {
	type DemoStruct struct {
		id string `kusto:"type:int64"`
	}

	expectedOutput := `.create-merge table ['DemoStruct'] (['id']:int64);`
	stmt := CreateTableStatement(DemoStruct{}, nil)
	assert.Equal(t, expectedOutput, stmt.String())
}
