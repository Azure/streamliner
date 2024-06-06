/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package adxingest

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

const tagName = "kusto"

func CreateTableStatement(s interface{}, tableName *string) kusto.Statement {
	if tableName == nil {
		tableName = getTableName(s)
	}

	builder := kql.New(`.create-merge table ['`).AddTable(*tableName).AddLiteral(`'] `)

	t := reflect.TypeOf(s)

	columns := []string{}

	for i := 0; i < t.NumField(); i++ {
		var columnName, columnType string

		field := t.Field(i)
		tagField := field.Tag.Get(tagName)

		tags := parseTag(tagField)

		if cn, ok := tags["column"]; ok {
			columnName = cn
		} else {
			columnName = strings.ToLower(field.Name)
		}

		if typeCol, ok := tags["type"]; ok {
			columnType = typeCol
		} else {
			columnType = field.Type.Name()
		}

		columns = append(columns, fmt.Sprintf(`['%s']:%s`, columnName, columnType))
	}

	columnsStmt := strings.Join(columns, ", ")

	builder.AddLiteral(`(`).AddUnsafe(columnsStmt).AddLiteral(`);`)

	return builder
}

func CreateTableMappings(s interface{}, tableName *string) kusto.Statement {
	if tableName == nil {
		tableName = getTableName(s)
	}

	builder := kql.New(`.create-or-alter table ['`).AddTable(*tableName).AddLiteral(`'] `)
	builder.AddLiteral(`ingestion json mapping '`).AddTable(*tableName + "_mapping").AddLiteral(`' `)

	t := reflect.TypeOf(s)

	columns := []string{}

	for i := 0; i < t.NumField(); i++ {
		var columnName, transform string

		field := t.Field(i)
		tagField := field.Tag.Get(tagName)

		tags := parseTag(tagField)

		if cn, ok := tags["column"]; ok {
			columnName = cn
		} else {
			columnName = strings.ToLower(field.Name)
		}

		if tr, ok := tags["transform"]; ok {
			transform = tr
		}

		columns = append(columns, fmt.Sprintf(`{"column": "%s","Properties":{"Path":"$.%s", "Transform":"%s"}}`, columnName, columnName, transform))
	}

	columnsStmt := strings.Join(columns, ", ")

	builder.AddLiteral(`'[`).AddUnsafe(columnsStmt).AddLiteral(`]';`)

	return builder
}

func parseTag(str string) map[string]string {
	tags := map[string]string{}
	fields := strings.Split(str, ";")

	for _, v := range fields {
		kv := strings.Split(v, ":")
		if len(kv) != 2 {
			continue
		}

		tags[kv[0]] = kv[1]
	}

	return tags
}

func getTableName(s interface{}) *string {
	var str string

	if t := reflect.TypeOf(s); t.Kind() == reflect.Ptr {
		str = t.Elem().Name()
	} else {
		str = t.Name()
	}

	return &str
}
