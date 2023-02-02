package terraform_module_test_helper

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/testing"
)

var _ logger.TestLogger = new(StreamLogger)

var serializedLogger = NewStreamLogger("", os.Stdout)

type StreamLogger struct {
	name   string
	stream io.ReadWriter
	mu     *sync.Mutex
	logCount int
}

func NewMemoryLogger(name string) *StreamLogger {
	buff := new(bytes.Buffer)
	return NewStreamLogger(name, buff)
}

func NewStreamLogger(name string, stream io.ReadWriter) *StreamLogger {
	return &StreamLogger{
		name:   name,
		stream: stream,
		mu:     new(sync.Mutex),
	}
}

func (s *StreamLogger) Logf(t testing.TestingT, format string, args ...interface{}) {
	logger.DoLog(t, 3, s.stream, fmt.Sprintf(format, args...))
	s.logCount++
	if s.name != "" && s.logCount % 50 == 0 {
		logger.Log(t, fmt.Sprintf("test %s is still running, current log count: %d", s.name, s.logCount))
	}
}

func (s *StreamLogger) PipeFrom(srcLogger *StreamLogger) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := io.Copy(s.stream, srcLogger.stream)
	return err
}

func (s *StreamLogger) Close() error {
	defer func() {
		c, ok := s.stream.(io.Closer)
		if ok {
			_ = c.Close()
		}
	}()
	return serializedLogger.PipeFrom(s)
}
