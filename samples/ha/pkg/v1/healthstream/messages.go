/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package healthstream

type MessageType int

const (
	Failover MessageType = iota
	TakeoverOn
	TakeoverOff
	Success
)

type Message struct {
	Type   MessageType
	Msg    string
	Origin string
}
