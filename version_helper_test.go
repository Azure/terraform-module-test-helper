package terraform_module_test_helper

import (
	"bufio"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/terraform"
	terratest "github.com/gruntwork-io/terratest/modules/testing"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/require"
)

func TestGetVersionSnapshot(t *testing.T) {
	s := SuccessTestVersionSnapshot("./", "example/basic")
	s.load(t)
	require.NotEmpty(t, s.Versions)
	require.Contains(t, s.Versions, "Terraform v")
	require.Contains(t, s.Versions, "registry.terraform.io/hashicorp/null")
}

func TestVersionSnapshotToString(t *testing.T) {
	snapshot := TestVersionSnapshot{
		Time:     time.Now(),
		Success:  true,
		Versions: "Content",
	}
	s := snapshot.ToString()
	scanner := bufio.NewScanner(strings.NewReader(s))
	require.True(t, scanner.Scan())
	title := scanner.Text()
	require.True(t, strings.HasPrefix(title, "## "))
	require.Equal(t, snapshot.Time.Format(time.RFC822), strings.TrimPrefix(title, "## "))
	require.Contains(t, s, "Success: true")
	require.Contains(t, s, snapshot.Versions)
	require.Contains(t, s, NoErrorMessage)
}

func TestOutputNewTestVersionSnapshot(t *testing.T) {
	defer func() {
		err := os.RemoveAll("TestRecord")
		if err != nil {
			println(err.Error())
		}
	}()
	localPath := filepath.Join("example", "basic", "TestRecord.md.tmp")
	defer func() { _ = os.Remove(localPath) }()
	uploadPath := filepath.Join("TestRecord", "basic", "TestRecord.md.tmp")
	defer func() { _ = os.Remove(uploadPath) }()
	content := "Content"
	stub := gostub.Stub(&initE, func(terratest.TestingT, *terraform.Options) (string, error) {
		return "", nil
	})
	defer stub.Reset()
	stub.Stub(&runTerraformCommandE, func(terratest.TestingT, *terraform.Options, ...string) (string, error) {
		return content, nil
	})
	s := SuccessTestVersionSnapshot(".", filepath.Join("example", "basic"))
	err := s.Save(t)
	require.Nil(t, err)
	file, err := os.ReadFile(filepath.Clean(localPath))
	s.Versions = content
	require.Nil(t, err)
	require.Equal(t, s.ToString(), string(file))
	require.True(t, files.FileExists(filepath.Clean(localPath)))
	require.True(t, files.FileExists(filepath.Clean(uploadPath)))
}

func TestVersionSnapshotRecord_initCommandErrorShouldReturnInitCommandError(t *testing.T) {
	expectedOutput := "init error"
	stub := gostub.Stub(&initE, func(terratest.TestingT, *terraform.Options) (string, error) {
		return expectedOutput, fmt.Errorf("init error")
	})
	defer stub.Reset()
	stub.Stub(&runTerraformCommandE, func(terratest.TestingT, *terraform.Options, ...string) (string, error) {
		return "not expected output", nil
	})
	s := SuccessTestVersionSnapshot(".", filepath.Join("example", "basic"))
	s.load(t)
	assert.Equal(t, expectedOutput, s.ErrorMsg)
}

func TestVersionSnapshotRecord_versionCommandErrorShouldReturnVersionCommandError(t *testing.T) {
	expectedOutput := "version command error"
	stub := gostub.Stub(&initE, func(terratest.TestingT, *terraform.Options) (string, error) {
		return "not expected output", nil
	})
	defer stub.Reset()
	stub.Stub(&runTerraformCommandE, func(terratest.TestingT, *terraform.Options, ...string) (string, error) {
		return expectedOutput, fmt.Errorf(expectedOutput)
	})
	s := SuccessTestVersionSnapshot(".", filepath.Join("example", "basic"))
	s.load(t)
	assert.Equal(t, expectedOutput, s.ErrorMsg)
}
