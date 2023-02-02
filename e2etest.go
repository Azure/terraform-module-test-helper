package terraform_module_test_helper

import (
	"fmt"
	terratest "github.com/gruntwork-io/terratest/modules/testing"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
)

var initLock = new(sync.Mutex)

type TerraformOutput = map[string]interface{}

func RunE2ETest(t *testing.T, moduleRootPath, exampleRelativePath string, option terraform.Options, assertion func(*testing.T, TerraformOutput)) {
	t.Parallel()
	defer func() {
		tearDown(t, moduleRootPath, exampleRelativePath)
	}()
	testDir := filepath.Join(moduleRootPath, exampleRelativePath)
	logger.Log(t, fmt.Sprintf("===> Starting test for %s, since we're running tests in parallel, the test log will be buffered and output to stdout after the test was finished.", testDir))

	tmpDir := test_structure.CopyTerraformFolderToTemp(t, moduleRootPath, exampleRelativePath)
	option.TerraformDir = tmpDir

	l := NewMemoryLogger()
	defer func() { _ = l.Close() }()
	option.Logger = logger.New(l)
	defer destroy(t, option)

	initAndApply(t, &option)
	if err := initAndPlanAndIdempotentAtEasyMode(t, option); err != nil {
		t.Fatalf(err.Error())
	}
	if assertion != nil {
		assertion(t, terraform.OutputAll(t, removeLogger(option)))
	}
}

func tearDown(t *testing.T, rootDir string, modulePath string) {
	s := SuccessTestVersionSnapshot(rootDir, modulePath)
	if t.Failed() {
		s = FailedTestVersionSnapshot(rootDir, modulePath, "")
	}
	require.NoError(t, s.Save(t))
}

func initAndApply(t terratest.TestingT, options *terraform.Options) string {
	tfInit(t, options)
	return terraform.Apply(t, options)
}

func tfInit(t terratest.TestingT, options *terraform.Options) {
	initLock.Lock()
	defer initLock.Unlock()
	terraform.Init(t, options)
}

func destroy(t *testing.T, option terraform.Options) {
	path := option.TerraformDir
	if !files.IsExistingDir(path) || !files.FileExists(filepath.Join(path, "terraform.tfstate")) {
		return
	}

	option.MaxRetries = 5
	option.TimeBetweenRetries = time.Minute
	option.RetryableTerraformErrors = map[string]string{
		".*": "Retry destroy on any error",
	}
	_, err := terraform.RunTerraformCommandE(t, &option, terraform.FormatArgs(&option, "destroy", "-auto-approve", "-input=false", "-refresh=false")...)
	if err != nil {
		_, err = terraform.DestroyE(t, &option)
	}
	require.NoError(t, err)
}

func removeLogger(option terraform.Options) *terraform.Options {
	// default logger might leak sensitive data
	option.Logger = logger.Discard
	return &option
}

func retryableOptions(t *testing.T, options terraform.Options) terraform.Options {
	result := terraform.WithDefaultRetryableErrors(t, &options)
	result.RetryableTerraformErrors[".*Please try again.*"] = "Service side suggest retry."
	return *result
}
