package terraform_module_test_helper

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
)

type TerraformOutput = map[string]interface{}

func RunE2ETest(t *testing.T, moduleRootPath, exampleRelativePath string, option terraform.Options, assertion func(*testing.T, TerraformOutput)) {
	t.Parallel()

	tmpDir := test_structure.CopyTerraformFolderToTemp(t, moduleRootPath, exampleRelativePath)
	if err := rewriteHcl(tmpDir, ""); err != nil {
		t.Fatalf(err.Error())
	}
	option.TerraformDir = tmpDir

	l := NewMemoryLogger()
	defer func() { _ = l.Close() }()
	option.Logger = logger.New(l)
	defer destroy(t, option)

	terraform.InitAndApply(t, &option)
	if err := initAndPlanAndIdempotentAtEasyMode(t, option); err != nil {
		t.Fatalf(err.Error())
	}
	if assertion != nil {
		assertion(t, terraform.OutputAll(t, removeLogger(option)))
	}
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
