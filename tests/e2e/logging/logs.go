package logging

import (
	"log"
	"time"

	"k8s.io/klog/v2"
)

const (
	// LogFlushFreqDefault is the default for the corresponding command line
	// parameter.
	LogFlushFreqDefault = 5 * time.Second
)

// KlogWriter serves as a bridge between the standard log package and the glog package.
type KlogWriter struct{}

// Write implements the io.Writer interface.
func (writer KlogWriter) Write(data []byte) (n int, err error) {
	klog.InfoDepth(1, string(data))
	return len(data), nil
}

// InitLogs disables support for contextual logging in klog while
// that Kubernetes feature is not considered stable yet. Commands
// which want to support contextual logging can:
//   - call klog.EnableContextualLogging after calling InitLogs,
//     with a fixed `true` or depending on some command line flag or
//     a feature gate check
//   - set up a FeatureGate instance, the advanced logging configuration
//     with Options and call Options.ValidateAndApply with the FeatureGate;
//     k8s.io/component-base/logs/example/cmd demonstrates how to do that
func InitLogs() {
	log.SetOutput(KlogWriter{})
	log.SetFlags(0)

	// Start flushing now. If LoggingConfiguration.ApplyAndValidate is
	// used, it will restart the daemon with the log flush interval defined
	// there.
	klog.StartFlushDaemon(LogFlushFreqDefault)

	// This is the default in Kubernetes. Options.ValidateAndApply
	// will override this with the result of a feature gate check.
	klog.EnableContextualLogging(false)
}

// FlushLogs flushes logs immediately. This should be called at the end of
// the main function via defer to ensure that all pending log messages
// are printed before exiting the program.
func FlushLogs() {
	klog.Flush()
}
