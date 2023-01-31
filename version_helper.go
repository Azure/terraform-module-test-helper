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

var initE = terraform.InitE
var runTerraformCommandE = terraform.RunTerraformCommandE

type TestVersionSnapshot struct {
	ModuleRootFolder        string
	SubModuleRelativeFolder string
	Time                    time.Time
	Success                 bool
	Output                  string
}

func SuccessTestVersionSnapshot(rootFolder, exampleRelativePath string) TestVersionSnapshot {
	return TestVersionSnapshot{
		ModuleRootFolder:        rootFolder,
		SubModuleRelativeFolder: exampleRelativePath,
		Time:                    time.Now(),
		Success:                 true,
	}
}

func FailedTestVersionSnapshot(rootFolder, exampleRelativePath, errMsg string) TestVersionSnapshot {
	return TestVersionSnapshot{
		ModuleRootFolder:        rootFolder,
		SubModuleRelativeFolder: exampleRelativePath,
		Time:                    time.Now(),
		Success:                 false,
		Output:                  errMsg,
	}
}

func (s *TestVersionSnapshot) ToString() string {
	return fmt.Sprintf(`## %s

Success: %t

%s

---

`, s.Time.Format(time.RFC822), s.Success, s.Output)
}

func (s *TestVersionSnapshot) RecordVersionSnapshot(t *testing.T) error {
	tmpFilePath, err := s.createTempRecordFile(t)
	if err != nil {
		return err
	}
	_, dir := filepath.Split(filepath.Join(s.ModuleRootFolder, s.SubModuleRelativeFolder))
	return copyFile(tmpFilePath, filepath.Join(s.ModuleRootFolder, "TestRecord", dir, "TestRecord.md.tmp"))
}

func (s *TestVersionSnapshot) createTempRecordFile(t *testing.T) (string, error) {
	path := filepath.Clean(filepath.Join(s.ModuleRootFolder, s.SubModuleRelativeFolder, "TestRecord.md.tmp"))
	if files.FileExists(path) {
		if err := os.Remove(path); err != nil {
			return "", err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	s.runVersionSnapshot(t)
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

func (s *TestVersionSnapshot) runVersionSnapshot(t *testing.T) {
	tmpPath := test_structure.CopyTerraformFolderToTemp(t, s.ModuleRootFolder, s.SubModuleRelativeFolder)
	defer func() {
		_ = os.RemoveAll(tmpPath)
	}()
	options := terraform.Options{
		TerraformDir: tmpPath,
		NoColor:      true,
		Logger:       logger.Discard,
	}
	if output, err := initE(t, &options); err != nil {
		s.Success = false
		s.Output = output
		return
	}
	output, err := runTerraformCommandE(t, &options, "version")
	if err != nil {
		s.Success = false
		s.Output = output
		return
	}
	if s.Success {
		s.Output = output
	}
}
