package terraform_module_test_helper

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadRetryableErrors(t *testing.T) {
	retryableErrors := ReadRetryableErrors("retryable_errors_sample.hcl.json", t)
	assert.Equal(t, 1, len(retryableErrors))
	desc, ok := retryableErrors["GatewayTimeout"]
	assert.True(t, ok)
	assert.Equal(t, "retryable errors set in retryable_errors_sample.hcl.json", desc)
}
