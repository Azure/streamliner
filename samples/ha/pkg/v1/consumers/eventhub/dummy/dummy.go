/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package dummy

import (
	"context"
	"time"

	"github.com/Azure/streamliner/samples/ha/pkg/v1/pipeline"

	"github.com/rs/zerolog"
)

type Dummy struct {
	Pipeline pipeline.Pipeline
	Log      *zerolog.Logger
	stopCh   chan bool
}

func (d *Dummy) Name() string {
	return "dummy-consumer"
}
func (d *Dummy) Start(ctx context.Context) error {
	var msgs pipeline.Messages

	d.stopCh = make(chan bool)

	msgs = append(msgs, []byte(payload1))

	d.Log.Debug().Msg("Sending 1 message from file")

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ticker.C:
			d.Pipeline.Next(&msgs)
		case <-d.stopCh:
			d.Log.Debug().Msg("received sig to stop dummy consumer")
			ticker.Stop()

			return nil
		}
	}
}

func (d *Dummy) Stop(ctx context.Context) error {
	d.stopCh <- true
	return nil
}

var payload1 = `
[{
    "id": "3664cb2f-d24a-4db8-8e6c-0393b4253929",
    "topic": "/subscriptions/b698965a-1f53-45ec-8b81-58458d1b25f7/resourceGroups/github.com/Azure/streamliner/samples/ha/providers/Microsoft.EventGrid/topics/translated-schedule-event-01",
    "subject": "/providers/microsoft.idmapping/aliases/default/namespaces/microsoft.compute/types/virtualmachines/identifiers/vmssidorvmid/values/b182bb9d-3376-4f30-81df-a25fd77c97c8/providers/microsoft.maintenance/scheduledevents/a9e57b41-fca6-4668-a2d2-0a5a4d6b6996",
    "data": {
        "resourceLocation": "eastus2euap",
        "publisherInfo": "microsoft.maintenance",
        "homeTenantId": "f8cdef31-a31e-4b4a-93e4-5f571e91255a",
        "resources": [
            {
                "resourceSystemProperties": {
                    "modifiedTime": "2023-08-28T20:02:27.5869585+00:00",
                    "createdBy": "System",
                    "changedAction": "Update",
                    "modifiedBy": "System"
                },
                "correlationId": "0dc66c28-a5cb-42a2-b679-462677ee86e2",
                "resourceId": "/providers/microsoft.idmapping/aliases/default/namespaces/microsoft.compute/types/virtualmachines/identifiers/vmssidorvmid/values/b182bb9d-3376-4f30-81df-a25fd77c97c8/providers/microsoft.maintenance/scheduledevents/a9e57b41-fca6-4668-a2d2-0a5a4d6b6996",
                "armResource": {
                    "id": "/providers/microsoft.idmapping/aliases/default/namespaces/microsoft.compute/types/virtualmachines/identifiers/vmssidorvmid/values/b182bb9d-3376-4f30-81df-a25fd77c97c8/providers/microsoft.maintenance/scheduledevents/a9e57b41-fca6-4668-a2d2-0a5a4d6b6996",
                    "name": "a9e57b41-fca6-4668-a2d2-0a5a4d6b6996",
                    "type": "microsoft.maintenance/scheduledevents",
                    "location": "eastus2euap",
                    "tags": {},
                    "plan": "",
                    "properties": {
                        "DurationInSeconds": -1,
                        "Description": "Virtual machine is going to be restarted as requested by authorized user.",
                        "EventId": "a9e57b41-fca6-4668-a2d2-0a5a4d6b6996",
                        "EventSource": "User",
                        "EventStatus": "Started",
                        "EventType": "Reboot",
                        "NotBefore": "",
                        "Resources": [
                            "eastus2euap_1"
                        ],
                        "ResourceType": "VirtualMachine"
                    },
                    "kind": "",
                    "managedBy": "System",
                    "sku": "",
                    "identity": {},
                    "zones": [
                        "eastus2euap-az01"
                    ],
                    "displayName": "a9e57b41-fca6-4668-a2d2-0a5a4d6b6996",
                    "apiVersion": "2020-03-01"
                },
                "apiVersion": "2020-03-01",
                "resourceEventTime": "2023-08-28T20:02:27.5869585+00:00",
                "sourceResourceId": "",
                "statusCode": "OK",
                "homeTenantId": "f8cdef31-a31e-4b4a-93e4-5f571e91255a"
            }
        ],
        "additionalBatchProperties": {
            "sdkVersion": "2.0.53-experimental",
            "batchSize": 1,
            "batchCorrelationId": "6943bb6d-9f84-4770-9a24-4bb2982ed1ca"
        },
        "dataBoundary": "Global"
    },
    "eventType": "microsoft.maintenance/scheduledevents/Write",
    "dataVersion": "3.0",
    "metadataVersion": "1",
    "eventTime": "2023-08-28T20:02:29.6649537Z"
}]`
