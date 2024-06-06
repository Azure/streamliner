/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package pipeline

type Message []byte

type Messages []Message

type Pipeline interface {
	Next(*Messages)
}

func (msg Message) String() string {
	return string(msg)
}

func (msgs Messages) String() string {
	var str string
	for _, msg := range msgs {
		str += msg.String() + "\n"
	}
	return str
}
