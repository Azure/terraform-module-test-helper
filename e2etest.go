package terraform_module_test_helper

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

type TerraformOutput = map[string]interface{}

func RunE2ETest(t *testing.T, moduleRootPath, exampleRelativePath string, option terraform.Options, assertion func(*testing.T, TerraformOutput)) {
	tmpDir := test_structure.CopyTerraformFolderToTemp(t, moduleRootPath, exampleRelativePath)
	option.TerraformDir = tmpDir
	defer terraform.Destroy(t, &option)
	terraform.InitAndApplyAndIdempotent(t, &option)
	if assertion != nil {
		assertion(t, terraform.OutputAll(t, &option))
	}
}
