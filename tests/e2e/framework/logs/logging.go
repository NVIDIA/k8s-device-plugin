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

package logs

import (
	"fmt"
	"log"
	"time"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/klog/v2"
)

const (
	// LogFlushFreqDefault is the default for the corresponding command line
	// parameter.
	LogFlushFreqDefault = 5 * time.Second
)

// KlogWriter serves as a bridge between the standard logs package and the glog package.
type KlogWriter struct{}

// Write implements the io.Writer interface.
func (writer KlogWriter) Write(data []byte) (n int, err error) {
	klog.InfoDepth(1, string(data))
	return len(data), nil
}

// InitLogs disables support for contextual logs in klog while
// that Kubernetes feature is not considered stable yet. Commands
// which want to support contextual logs can:
//   - call klog.EnableContextualLogging after calling InitLogs,
//     with a fixed `true` or depending on some command line flag or
//     a feature gate check
//   - set up a FeatureGate instance, the advanced logs configuration
//     with Options and call Options.ValidateAndApply with the FeatureGate;
//     k8s.io/component-base/logs/example/cmd demonstrates how to do that
func InitLogs() {
	log.SetOutput(KlogWriter{})
	log.SetFlags(0)

	// Start flushing now. If LoggingConfiguration.ApplyAndValidate is
	// used, it will restart the daemon with the logs flush interval defined
	// there.
	klog.StartFlushDaemon(LogFlushFreqDefault)

	// This is the default in Kubernetes. Options.ValidateAndApply
	// will override this with the result of a feature gate check.
	klog.EnableContextualLogging(false)
}

// FlushLogs flushes logs immediately. This should be called at the end of
// the main function via defer to ensure that all pending logs messages
// are printed before exiting the program.
func FlushLogs() {
	klog.Flush()
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func logf(level string, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf logs the info.
func Logf(format string, args ...interface{}) {
	logf("INFO", format, args...)
}

// Failf logs the fail info, including a stack trace starts with its direct caller
// (for example, for call chain f -> g -> Failf("foo", ...) error would be logged for "g").
func Failf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	skip := 1
	ginkgo.Fail(msg, skip)
	panic("unreachable")
}

// Fail is an alias for ginkgo.Fail.
var Fail = ginkgo.Fail
