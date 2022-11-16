package terraform_module_test_helper

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func ReadRetryableErrors(f *os.File, t *testing.T) map[string]string {
	cfg := struct {
		RetryableErrors []string `json:"retryable_errors"`
	}{}

	byteValue, _ := io.ReadAll(f)
	err := json.Unmarshal(byteValue, &cfg)
	if err != nil {
		t.Fatalf("cannot unmarshal retryable_errors.hcl.json")
	}
	retryableRegexes := cfg.RetryableErrors
	retryableErrors := make(map[string]string)
	for _, r := range retryableRegexes {
		retryableErrors[r] = fmt.Sprintf("retryable errors set in %s", f.Name())
	}
	return retryableErrors
}
