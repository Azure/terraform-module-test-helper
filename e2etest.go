package terraform_module_test_helper

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

type TerraformOutput = map[string]interface{}
type TerraformOutputAssertion = func(t *testing.T, output TerraformOutput)

type TestFolder struct {
	RootPath                    string
	ExampleFolderRelativeToRoot string
}

func NewTestFolder(rootPath, exampleFolderRelativeToRoot string) TestFolder {
	return TestFolder{
		RootPath:                    rootPath,
		ExampleFolderRelativeToRoot: exampleFolderRelativeToRoot,
	}
}

func RunE2ETest(t *testing.T, testFolder TestFolder, option terraform.Options) TerraformOutput {
	tmpDir := test_structure.CopyTerraformFolderToTemp(t, testFolder.RootPath, testFolder.ExampleFolderRelativeToRoot)
	option.TerraformDir = tmpDir
	defer terraform.Destroy(t, &option)
	terraform.InitAndApplyAndIdempotent(t, &option)
	return terraform.OutputAll(t, &option)
}
