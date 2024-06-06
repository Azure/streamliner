/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package app

import (
	"errors"
	"fmt"
)

var ErrApp = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("application %w : %s", ErrApp, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("application %w : %s", err, text)
}
