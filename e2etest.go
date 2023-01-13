package terraform_module_test_helper

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

type TerraformOutput = map[string]interface{}

func RunE2ETest(t *testing.T, moduleRootPath, exampleRelativePath string, option terraform.Options, assertion func(*testing.T, TerraformOutput)) {
	option = retryableOptions(t, option)
	tmpDir := test_structure.CopyTerraformFolderToTemp(t, moduleRootPath, exampleRelativePath)
	if err := rewriteHcl(tmpDir, ""); err != nil {
		t.Fatalf(err.Error())
	}
	option.TerraformDir = tmpDir
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
	option.MaxRetries = 5
	option.TimeBetweenRetries = time.Minute
	option.RetryableTerraformErrors = map[string]string{
		".*": "Retry destroy on any error",
	}
	_, err := terraform.DestroyE(t, &option)
	if err != nil {
		_, err = terraform.RunTerraformCommandE(t, &option, terraform.FormatArgs(&option, "destroy", "-auto-approve", "-input=false", "-refresh=false")...)
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
	return *result
}
