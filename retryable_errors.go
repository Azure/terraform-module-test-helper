package terraform_module_test_helper

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func ReadRetryableErrors(path string, t *testing.T) map[string]string {
	jsonFile, err := os.Open(path)
	if err != nil {
		t.Fatalf("cannot find retryable_errors.hcl.json")
	}
	fmt.Println("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer func() {
		_ = jsonFile.Close()
	}()

	cfg := struct {
		RetryableErrors []string `json:"retryable_errors"`
	}{}

	byteValue, _ := io.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &cfg)
	if err != nil {
		t.Fatalf("cannot unmarshal retryable_errors.hcl.json")
	}
	retryableRegexes := cfg.RetryableErrors
	retryableErrors := make(map[string]string)
	for _, r := range retryableRegexes {
		retryableErrors[r] = fmt.Sprintf("retryable errors set in %s", path)
	}
	return retryableErrors
}
