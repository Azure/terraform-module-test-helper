package terraform_module_test_helper

import (
	"fmt"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/testing"
	"io"
	"os"
	"sync"
)

var _ logger.TestLogger = new(StreamLogger)

// SerializedLogger Global Logger for Std output
var SerializedLogger = NewStreamLogger(os.Stdout)

type StreamLogger struct {
	stream io.ReadWriter
	mu     *sync.Mutex
}

func NewStreamLogger(stream io.ReadWriter) *StreamLogger {
	return &StreamLogger{
		stream: stream,
		mu:     new(sync.Mutex),
	}
}

func (s *StreamLogger) Logf(t testing.TestingT, format string, args ...interface{}) {
	logger.DoLog(t, 3, s.stream, fmt.Sprintf(format, args...))
}

// PipeTo s.stream -> destLogger.stream
func (s *StreamLogger) PipeTo(destLogger *StreamLogger) error {
	_, err := io.Copy(destLogger.stream, s.stream)
	return err
}

// PipeFrom s.stream <- src.stream
func (s *StreamLogger) PipeFrom(srcLogger *StreamLogger) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := io.Copy(s.stream, srcLogger.stream)
	return err
}

func (s *StreamLogger) Close() error {
	return SerializedLogger.PipeFrom(s)
}
