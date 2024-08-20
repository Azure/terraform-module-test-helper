package terraform_module_test_helper

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func ReadRetryableErrors(retryableCfg []byte, t *testing.T) map[string]string {
	cfg := struct {
		RetryableErrors []string `json:"retryable_errors"`
	}{}

	require.NoError(t, json.Unmarshal(retryableCfg, &cfg))
	retryableRegexes := cfg.RetryableErrors
	retryableErrors := make(map[string]string)
	for _, r := range retryableRegexes {
		retryableErrors[r] = "retryable errors set by test"
	}
	return retryableErrors
}
