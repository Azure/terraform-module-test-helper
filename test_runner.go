package terraform_module_test_helper

import (
	"fmt"
	terratest "github.com/gruntwork-io/terratest/modules/testing"
	"github.com/stretchr/testify/require"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type testingT interface {
	terratest.TestingT
	T() *testing.T
	Failed() bool
	ErrorMessage() string
}

var _ testingT = &T{}

type T struct {
	failed        bool
	t             *testing.T
	errorMessages []string
}

func (t *T) T() *testing.T {
	return t.t
}

func newT(t *testing.T) *T {
	return &T{t: t}
}

func (t *T) error(message string) {
	t.errorMessages = append(t.errorMessages, message)
}

func (t *T) Failed() bool {
	return t.failed
}

func (t *T) Fail() {
	t.failed = true
	t.error("Fail")
	t.t.Fail()
}

func (t *T) FailNow() {
	t.failed = true
	t.error("FailNow")
	t.t.FailNow()
}

func (t *T) Fatal(args ...interface{}) {
	t.failed = true
	t.error("Fatal:" + fmt.Sprintln(args...))
	t.t.Fatal(args...)
}

func (t *T) Fatalf(format string, args ...interface{}) {
	t.failed = true
	t.error("Fatal:" + fmt.Sprintf(format, args...))
	t.t.Fatalf(format, args...)
}

func (t *T) Error(args ...interface{}) {
	t.error("Error:" + fmt.Sprintln(args...))
	t.t.Error(args...)
}

func (t *T) Errorf(format string, args ...interface{}) {
	t.error("Error:" + fmt.Sprintf(format, args...))
	t.t.Errorf(format, args...)
}

func (t *T) Name() string {
	return t.t.Name()
}

func (t *T) ErrorMessage() string {
	sb := strings.Builder{}
	for i := 0; i < len(t.errorMessages); i++ {
		sb.WriteString(t.errorMessages[i])
		if i < len(t.errorMessages)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

var _ testingT = &mockT{}

type mockT struct {
	t *T
}

func (m *mockT) Name() string {
	return m.t.Name()
}

func (m *mockT) T() *testing.T {
	return m.t.T()
}

func (m *mockT) Failed() bool {
	return m.t.Failed()
}

func (m *mockT) ErrorMessage() string {
	return m.t.ErrorMessage()
}

func (m *mockT) Fail() {
	m.t.failed = true
	m.t.error("Fail")
}

func (m *mockT) FailNow() {
	m.t.failed = true
	m.t.error("Fail")
	runtime.Goexit()
}

func (m *mockT) Fatal(args ...interface{}) {
	m.t.failed = true
	m.t.error("Fatal:" + fmt.Sprintln(args...))
	runtime.Goexit()
}

func (m *mockT) Fatalf(format string, args ...interface{}) {
	m.t.failed = true
	m.t.error("Fatal:" + fmt.Sprintf(format, args...))
	runtime.Goexit()
}

func (m *mockT) Error(args ...interface{}) {
	m.t.error("Error:" + fmt.Sprintln(args...))
}

func (m *mockT) Errorf(format string, args ...interface{}) {
	m.t.error("Error:" + fmt.Sprintf(format, args...))
}

func expectFailure(t *testing.T, f func(tt testingT)) *mockT {
	m := &mockT{t: newT(t)}
	var wg sync.WaitGroup
	// setup the barrier
	wg.Add(1)
	// start a co-routine to execute the test function f
	// and release the barrier at its end
	go func() {
		defer wg.Done()
		f(m)
	}()

	// wait for the barrier.
	wg.Wait()
	// verify fail now is invoked
	require.True(t, m.t.failed)
	return m
}
