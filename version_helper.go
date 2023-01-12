package terraform_module_test_helper

import (
	"fmt"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/logger"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

type TestVersionSnapshot struct {
	Time    time.Time
	Success bool
	Output  string
}

func (s TestVersionSnapshot) ToString() string {
	return fmt.Sprintf(`## %s

Success: %t
%s

---

`, s.Time.Format(time.RFC822), s.Success, s.Output)
}

func GetVersion(t *testing.T, rootFolder, terraformModuleFolder string) TestVersionSnapshot {
	tmpPath := test_structure.CopyTerraformFolderToTemp(t, rootFolder, terraformModuleFolder)
	defer func() {
		_ = os.RemoveAll(tmpPath)
	}()
	options := terraform.Options{
		TerraformDir: tmpPath,
		NoColor:      true,
		Logger:       logger.Discard,
	}
	output, err := terraform.InitE(t, &options)
	if err != nil {
		return TestVersionSnapshot{
			Time:    time.Now(),
			Success: false,
			Output:  output,
		}
	}
	output, err = terraform.RunTerraformCommandE(t, &options, "version")
	return TestVersionSnapshot{
		Time:    time.Now(),
		Success: err == nil,
		Output:  output,
	}
}

func RecordVersionSnapshot(t *testing.T, rootFolder, terraformModuleFolder string) error {
	path := filepath.Join(rootFolder, terraformModuleFolder, "TestRecord.md.tmp")
	if files.FileExists(path) {
		err := os.Remove(path)
		if err != nil {
			return err
		}
	}
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	s := generateVersionSnapshot(t, rootFolder, terraformModuleFolder)
	_, err = f.WriteString(s.ToString())
	return err
}

var generateVersionSnapshot = GetVersion
