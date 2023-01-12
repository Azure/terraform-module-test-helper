package terraform_module_test_helper

import (
	"bufio"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetVersionSnapshot(t *testing.T) {
	version := GetVersion(t, "./", "example/basic")
	require.NotEmpty(t, version.Output)
	require.Contains(t, version.Output, "terraform_version")
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
	require.Equal(t, snapshot.Time.Format(time.RFC822), strings.TrimLeft(title, "## "))
	require.Contains(t, s, "Success: true")
	require.Contains(t, s, snapshot.Output)
}

func TestOutputNewTestVersionSnapshot(t *testing.T) {
	destPath := filepath.Join("example", "basic", "TestRecord.md")
	tmpPath := filepath.Join("example", "basic", "TestRecord.md.tmp")
	defer func() {
		_ = os.Remove(destPath)
		_ = os.Remove(tmpPath)
	}()

	snapshot := TestVersionSnapshot{
		Time:    time.Now(),
		Success: true,
		Output:  "Content",
	}
	err := RecordVersionSnapshot(snapshot, ".", filepath.Join("example", "basic"))
	require.Nil(t, err)
	file, err := os.ReadFile(destPath)
	require.Nil(t, err)
	require.Equal(t, snapshot.ToString(), string(file))
	require.False(t, files.FileExists(tmpPath))

	snapshot2 := TestVersionSnapshot{
		Time:    time.Now(),
		Success: true,
		Output:  "Content2",
	}

	err = RecordVersionSnapshot(snapshot2, ".", filepath.Join("example", "basic"))
	require.Nil(t, err)
	file, err = os.ReadFile(destPath)
	require.Nil(t, err)
	require.Equal(t, fmt.Sprintf("%s%s", snapshot2.ToString(), snapshot.ToString()), string(file))
	require.False(t, files.FileExists(tmpPath))
}
