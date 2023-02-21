package terraform_module_test_helper

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

func TestUnitExampleTest(t *testing.T) {
	RunUnitTest(t, "./", "example/basic", terraform.Options{
		Upgrade: true,
	}, func(t *testing.T, output TerraformOutput) {
		resId, ok := output["resource_id"].(string)
		assert.True(t, ok)
		assert.NotEqual(t, "", resId, "expected output `resource_id`")
	})
}
