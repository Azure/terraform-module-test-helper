package terraform_module_test_helper

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/files"
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
	}
	output, err := terraform.InitE(t, &options)
	if err != nil {
		return TestVersionSnapshot{
			Time:    time.Now(),
			Success: false,
			Output:  output,
		}
	}
	output, err = terraform.RunTerraformCommandE(t, &options, "version", "-json")
	return TestVersionSnapshot{
		Time:    time.Now(),
		Success: err == nil,
		Output:  output,
	}
}

func RecordVersionSnapshot(s TestVersionSnapshot, rootFolder, terraformModuleFolder string) error {
	path := filepath.Join(rootFolder, terraformModuleFolder, "TestRecord.md")
	if !files.FileExists(path) {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
	}
	of, err := os.Open(path)
	if err != nil {
		return err
	}
	tmpPath := fmt.Sprintf("%s.tmp", path)
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	_, err = f.WriteString(s.ToString())
	if err != nil {
		return err
	}
	_, err = io.Copy(f, of)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	err = of.Close()
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
