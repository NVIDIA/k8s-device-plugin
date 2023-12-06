/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package framework

import (
	"errors"
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/format"

	e2elog "github.com/NVIDIA/k8s-device-plugin/tests/e2e/framework/logs"
)

// FailureError is an error where the error string is meant to be passed to
// ginkgo.Fail directly, i.e. adding some prefix like "unexpected error" is not
// necessary. It is also not necessary to dump the error struct.
type FailureError struct {
	msg            string
	fullStackTrace string
}

func (f FailureError) Error() string {
	return f.msg
}

func (f FailureError) Backtrace() string {
	return f.fullStackTrace
}

// ErrFailure is an empty error that can be wrapped to indicate that an error
// is a FailureError. It can also be used to test for a FailureError:.
//
//	return fmt.Errorf("some problem%w", ErrFailure)
//	...
//	err := someOperation()
//	if errors.Is(err, ErrFailure) {
//	    ...
//	}
var ErrFailure error = FailureError{}

// ExpectNoError checks if "err" is set, and if so, fails assertion while logs the error.
func ExpectNoError(err error, explain ...interface{}) {
	ExpectNoErrorWithOffset(1, err, explain...)
}

// ExpectNoErrorWithOffset checks if "err" is set, and if so, fails assertion while logs the error at "offset" levels above its caller
// (for example, for call chain f -> g -> ExpectNoErrorWithOffset(1, ...) error would be logged for "f").
func ExpectNoErrorWithOffset(offset int, err error, explain ...interface{}) {
	if err == nil {
		return
	}

	// Errors usually contain unexported fields. We have to use
	// a formatter here which can print those.
	prefix := ""
	if len(explain) > 0 {
		if str, ok := explain[0].(string); ok {
			prefix = fmt.Sprintf(str, explain[1:]...) + ": "
		} else {
			prefix = fmt.Sprintf("unexpected explain arguments, need format string: %v", explain)
		}
	}

	// This intentionally doesn't use gomega.Expect. Instead we take
	// full control over what information is presented where:
	// - The complete error object is logged because it may contain
	//   additional information that isn't included in its error
	//   string.
	// - It is not included in the failure message because
	//   it might make the failure message very large and/or
	//   cause error aggregation to work less well: two
	//   failures at the same code line might not be matched in
	//   https://go.k8s.io/triage because the error details are too
	//   different.
	//
	// Some errors include all relevant information in the Error
	// string. For those we can skip the redundant logs message.
	// For our own failures we only logs the additional stack backtrace
	// because it is not included in the failure message.
	var failure FailureError
	if errors.As(err, &failure) && failure.Backtrace() != "" {
		e2elog.Logf("Failed inside E2E framework:\n    %s", strings.ReplaceAll(failure.Backtrace(), "\n", "\n    "))
	} else if !errors.Is(err, ErrFailure) {
		e2elog.Logf("Unexpected error: %s\n%s", prefix, format.Object(err, 1))
	}
	ginkgo.Fail(prefix+err.Error(), 1+offset)
}
