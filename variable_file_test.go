package terraform_module_test_helper

import (
	"encoding/json"
	"github.com/gruntwork-io/go-commons/files"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestVarsToFile(t *testing.T) {
	vars := make(map[string]any, 0)
	vars["number"] = 1.0
	vars["string"] = "hello"
	vars["object"] = map[string]any {
		"key": "value",
	}
	path := VarsToFile(t, vars)
	defer func() {
		_ = os.Remove(path)
	}()

	content, err := files.ReadFileAsString(path)
	assert.NoError(t, err)
	actual := make(map[string]any, 0)
	err = json.Unmarshal([]byte(content), &actual)
	assert.NoError(t, err)
	assert.Equal(t, vars, actual)
}