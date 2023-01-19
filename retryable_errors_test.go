package terraform_module_test_helper

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadRetryableErrors(t *testing.T) {
	cfg, err := os.ReadFile("retryable_errors_sample.hcl.json")
	if err != nil {
		t.Fatalf(err.Error())
	}
	retryableErrors := ReadRetryableErrors(cfg, t)
	assert.Equal(t, 1, len(retryableErrors))
	desc, ok := retryableErrors["GatewayTimeout"]
	assert.True(t, ok)
	assert.Equal(t, "retryable errors set by test", desc)
}
