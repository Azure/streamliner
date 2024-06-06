/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package mock

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Azure/streamliner/samples/ha/pkg/config"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/healthstream"
	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"

	"github.com/rs/zerolog"
	"github.com/valyala/fastjson"
)

type Dummy struct {
	Data     *pipeline.Messages
	Log      *zerolog.Logger
	pool     fastjson.ParserPool
	Config   config.EventGrid
	HealthCh chan *healthstream.Message
}

const (
	armIDPattern = `(?m)^\"?\/subscriptions/[\w-]+\/resourceGroups\/[\w-]+\/providers/Microsoft.Compute/virtualMachineScaleSets\/[\w-]+`
)

func (d *Dummy) Start(ctx context.Context) error {
	return nil
}
func (d *Dummy) Stop(ctx context.Context) error {
	return nil
}

func (d *Dummy) Next(msgs *pipeline.Messages) {
	parser := d.pool.Get()

	d.Data = msgs
	for _, msg := range *msgs {
		jsonMsg, err := parser.Parse(msg.String())
		if err != nil {
			continue
		}

		jsonEvents, err := jsonMsg.Array()

		if err != nil {
			d.Log.Err(err).Msg("Error!")
			continue
		}

		for _, scheduleEvent := range jsonEvents {
			scheduleEvent.Set("topic", fastjson.MustParse(fmt.Sprintf("%q", d.Config.Primary.Topic)))
			found := d.traverseAndFind(scheduleEvent, "root")

			if found {
				d.Log.Debug().Msg("found ARM Id in json blob")
			}
		}
		d.Log.Debug().Msg(jsonMsg.String())
	}

	d.pool.Put(parser)
}

func (d *Dummy) Name() string {
	return "dummy-publisher"
}

// traverseAndFind traverses the json blob and on match of an ArmId will exit
func (d *Dummy) traverseAndFind(value *fastjson.Value, prefix string) bool {
	re := regexp.MustCompile(armIDPattern)

	switch value.Type() {
	case fastjson.TypeObject:
		var found bool

		object := value.GetObject()

		object.Visit(func(key []byte, v *fastjson.Value) {
			newPrefix := fmt.Sprintf("%s.%s", prefix, key)

			if v.Type() == fastjson.TypeString {
				match := re.MatchString(v.String())
				if match {
					d.Log.Debug().Str(string(key), v.String()).Msg("found")
					found = true
				}
				return
			}
			d.traverseAndFind(v, newPrefix)
		})

		return found

	case fastjson.TypeArray:
		array := value.GetArray()
		for i, v := range array {
			newPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			d.traverseAndFind(v, newPrefix)
		}
	case fastjson.TypeNull:
	case fastjson.TypeFalse:
	case fastjson.TypeString:
	case fastjson.TypeNumber:
	case fastjson.TypeTrue:
	default:
	}

	return false
}
