package terraform_module_test_helper

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/require"
)

func TestGetVersionSnapshot(t *testing.T) {
	version := NewVersionSnapshot(t, "./", "example/basic", true)
	require.NotEmpty(t, version.Output)
	require.Contains(t, version.Output, "Terraform v")
	require.Contains(t, version.Output, "registry.terraform.io/hashicorp/null")
}

func TestVersionSnapshotToString(t *testing.T) {
	snapshot := TestVersionSnapshot{
		Time:    time.Now(),
		Success: true,
		Output:  "Content",
	}
	s := snapshot.ToString()
	scanner := bufio.NewScanner(strings.NewReader(s))
	require.True(t, scanner.Scan())
	title := scanner.Text()
	require.True(t, strings.HasPrefix(title, "## "))
	require.Equal(t, snapshot.Time.Format(time.RFC822), strings.TrimPrefix(title, "## "))
	require.Contains(t, s, "Success: true")
	require.Contains(t, s, snapshot.Output)
}

func TestOutputNewTestVersionSnapshot(t *testing.T) {
	tmpPath := filepath.Join("example", "basic", "TestRecord.md.tmp")
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	snapshot := TestVersionSnapshot{
		Time:    time.Now(),
		Success: true,
		Output:  "Content",
	}
	stub := gostub.Stub(&generateVersionSnapshot, func(t *testing.T, rootFolder, terraformModuleFolder string, success bool) TestVersionSnapshot {
		return snapshot
	})
	defer stub.Reset()
	err := RecordVersionSnapshot(t, ".", filepath.Join("example", "basic"), true)
	require.Nil(t, err)
	file, err := os.ReadFile(filepath.Clean(tmpPath))
	require.Nil(t, err)
	require.Equal(t, snapshot.ToString(), string(file))
	require.True(t, files.FileExists(filepath.Clean(tmpPath)))
}
