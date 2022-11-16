package terraform_module_test_helper

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestLoadRetryableErrors(t *testing.T) {
	f, err := os.Open("retryable_errors_sample.hcl.json")
	if err != nil {
		t.Fatalf("Cannot open test file")
	}
	retryableErrors := ReadRetryableErrors(f, t)
	assert.Equal(t, 1, len(retryableErrors))
	desc, ok := retryableErrors["GatewayTimeout"]
	assert.True(t, ok)
	assert.Equal(t, "retryable errors set in retryable_errors_sample.hcl.json", desc)
}
