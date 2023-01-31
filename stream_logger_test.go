package terraform_module_test_helper

import (
	"bytes"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStreamLoggerShouldLogSth(t *testing.T) {
	buff := new(bytes.Buffer)
	l := NewStreamLogger(buff)
	log := "hello"
	l.Logf(t, log)
	assert.Contains(t, buff.String(), log)
}

func TestStreamLoggerCanPipeLogsToAnotherStreamLogger(t *testing.T) {
	log := "hello"
	srcBuff := bytes.NewBufferString(log)
	srcLogger := NewStreamLogger(srcBuff)
	destBuff := new(bytes.Buffer)
	destLogger := NewStreamLogger(destBuff)
	err := destLogger.PipeFrom(srcLogger)
	require.Nil(t, err)
	assert.Contains(t, destBuff.String(), log)
}

func TestStreamLoggerClose(t *testing.T) {
	log := "hello"
	srcBuff := bytes.NewBufferString(log)
	srcLogger := NewStreamLogger(srcBuff)

	destBuff := new(bytes.Buffer)
	dummyLogger := NewStreamLogger(destBuff)
	stub := gostub.Stub(&serializedLogger, dummyLogger)
	defer stub.Reset()

	err := srcLogger.Close()
	require.Nil(t, err)
	assert.Contains(t, destBuff.String(), log)
}