/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package tasks

import (
	"context"
	"testing"
)

type MockTask struct{}

func (m MockTask) Name() string {
	return "MockTask"
}

func (m MockTask) Start(ctx context.Context) error {
	return nil
}

func (m MockTask) Stop(ctx context.Context) error {
	return nil
}

func TestTask(t *testing.T) {
	task := MockTask{}

	if task.Name() != "MockTask" {
		t.Errorf("Expected task name to be 'MockTask', got '%s'", task.Name())
	}

	if err := task.Start(context.Background()); err != nil {
		t.Errorf("Expected no error when starting task, got '%s'", err)
	}

	if err := task.Stop(context.Background()); err != nil {
		t.Errorf("Expected no error when stopping task, got '%s'", err)
	}
}
