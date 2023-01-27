package terraform_module_test_helper

import (
	"bytes"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

/*
 * 需求：传入一个 buffer，StreamLogger 应该把需要 log 的内容打到 buffer 里面
 */
func TestStreamLoggerShouldLogSth(t *testing.T) {
	buff := new(bytes.Buffer)
	l := NewStreamLogger(buff)
	log := "hello"
	l.Logf(t, log)
	assert.Contains(t, buff.String(), log)
}

/**
1. Logger can pipe stream to another logger
2. Logger can be closed, when closed, pipe stream to another logger
3. Develop a global logger.
	- 别的 Logger 可以把 buffer 内的东西传给 GL,
	- GL 可以把这些 buffer 的内容打印到（PipeTo）STDOUT
*/

/*
 * 写一个 PipeTo(destination), 使得 srcLogger 把 srcBuff -> destLogger 的 destBuff
 */
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

/*
 * 写一个 Close(), 使得每次 Logger.Close() 的时候，GL 都会自动读 logger.stream 里的值
 */
func TestStreamLoggerClose(t *testing.T) {
	log := "hello"
	srcBuff := bytes.NewBufferString(log)
	srcLogger := NewStreamLogger(srcBuff)

	// 由于 SerializedLogger 的输出在 STDOUT，很难做测试
	// 打桩，在 line56 临时把 SerializedLogger 的值换成 dummyLogger
	// line59 defer stub.Reset() 把 SerializedLogger 再换回来
	destBuff := new(bytes.Buffer)
	dummyLogger := NewStreamLogger(destBuff)
	stub := gostub.Stub(&SerializedLogger, dummyLogger)
	defer stub.Reset()

	err := srcLogger.Close()
	require.Nil(t, err)
	assert.Contains(t, destBuff.String(), log)
}

func TestStreamLoggerPipeIsSerialize(t *testing.T) {
	log1 := "hello "
	srcBuff1 := bytes.NewBufferString(log1)
	srcLogger1 := NewStreamLogger(srcBuff1)

	destBuff := new(bytes.Buffer)
	dummyLogger := NewStreamLogger(destBuff)
	stub := gostub.Stub(&SerializedLogger, dummyLogger)
	defer stub.Reset()

	err := srcLogger1.Close()
	require.Nil(t, err)
	time.Sleep(500 * time.Millisecond)

	log2 := "world"
	srcBuff2 := bytes.NewBufferString(log2)
	srcLogger2 := NewStreamLogger(srcBuff2)
	err = srcLogger2.Close()
	require.Nil(t, err)

	assert.Contains(t, destBuff.String(), log1+log2)
}
