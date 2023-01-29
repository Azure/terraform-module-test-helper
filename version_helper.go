package terraform_module_test_helper

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/logger"
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

func NewVersionSnapshot(t *testing.T, rootFolder, terraformModuleFolder string, success bool) TestVersionSnapshot {
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
		Success: success && err == nil,
		Output:  output,
	}
}

func RecordVersionSnapshot(t *testing.T, rootFolder, terraformModuleFolder string, success bool) error {
	tmpFilePath, err := createTempRecordFile(t, rootFolder, terraformModuleFolder, success)
	if err != nil {
		return err
	}
	_, dir := filepath.Split(filepath.Join(rootFolder, terraformModuleFolder))
	return copyFile(tmpFilePath, filepath.Join(rootFolder, "TestRecord", dir, "TestRecord.md.tmp"))
}

func createTempRecordFile(t *testing.T, rootFolder string, terraformModuleFolder string, success bool) (string, error) {
	path := filepath.Clean(filepath.Join(rootFolder, terraformModuleFolder, "TestRecord.md.tmp"))
	if files.FileExists(path) {
		err := os.Remove(path)
		if err != nil {
			return "", err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	s := generateVersionSnapshot(t, rootFolder, terraformModuleFolder, success)
	_, err = f.WriteString(s.ToString())
	return path, err
}

func copyFile(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", src)
	}

	dst = filepath.Clean(dst)
	dstDir := filepath.Dir(dst)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) && os.MkdirAll(dstDir, os.ModePerm) != nil {
		return fmt.Errorf("failed to create destination folder: %s", dstDir)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) && os.Remove(dst) != nil {
		return fmt.Errorf("failed to delete destination file: %s", dst)
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %s", src)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %s", dst)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %s", err)
	}
	return nil
}

var generateVersionSnapshot = NewVersionSnapshot
