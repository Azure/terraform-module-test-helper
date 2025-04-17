package terraform_module_test_helper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tfjson "github.com/hashicorp/terraform-json"
)

func TestIgnoreOnlyAddOrUpdate_AllowsCreateAndUpdate(t *testing.T) {
	changes := map[string]*tfjson.ResourceChange{
		"a": {Change: &tfjson.Change{Actions: tfjson.Actions{"create"}}},
		"b": {Change: &tfjson.Change{Actions: tfjson.Actions{"update"}}},
		"c": {Change: &tfjson.Change{Actions: tfjson.Actions{"no-op"}}},
	}
	require.True(t, ignoreOnlyAddOrUpdate(changes))
}

func TestIgnoreOnlyAddOrUpdate_FailsOnDeleteOrReplace(t *testing.T) {
	changes1 := map[string]*tfjson.ResourceChange{
		"a": {Change: &tfjson.Change{Actions: tfjson.Actions{"delete"}}},
	}
	require.False(t, ignoreOnlyAddOrUpdate(changes1))

	changes2 := map[string]*tfjson.ResourceChange{
		"a": {Change: &tfjson.Change{Actions: tfjson.Actions{"delete", "create"}}},
		"b": {Change: &tfjson.Change{Actions: tfjson.Actions{"replace"}}},
	}
	require.False(t, ignoreOnlyAddOrUpdate(changes2))
}

func TestModuleCheckIfDestructive_FailOnReplace(t *testing.T) {
	// Stub out GitHub tag lookup and clone so we stay in this repo
	stub := gostub.Stub(&getLatestTag, func(owner, repo string, currentMajorVer int) (string, error) {
			// Simulate finding a previous tag for the major version
			return "v1.0.0", nil
	})
	defer stub.Reset()
	stub.Stub(&cloneGithubRepo, func(owner, repo string, tag *string) (string, error) {
			// Simulate cloning the repo root for the old tag
			return ".", nil
	})

	wt := newT(t)
	cwd, err := os.Getwd()
	require.NoError(t, err)

	err = moduleCheckIfDestructive(
			wt,
			"Azure", // Mock owner
			"terraform-module-test-helper", // Mock repo
			// Path to the example harness using the 'before' module structure (relative to repo root)
			"example/upgrade_destructive/example/version_upgrade",
			// Absolute path to the 'after' module code
			filepath.Join(cwd, "example", "after_destructive_upgrade"),
			terraform.Options{Upgrade: true},
			1, // Mock current major version (must be > 0 to avoid SkipV0Error)
			&moduleUpgradeOptions{allowV0Testing: true}, // Options for the destructive check
	)

	require.Error(t, err, "Expected an error due to destructive changes, but got nil")
	assert.Contains(t, err.Error(), "destructive operations", "Error message should indicate destructive operations")
}


func TestModuleCheckIfDestructive_PassOnCreateOnly(t *testing.T) {
	// Stub out GitHub tag lookup and clone
	stub := gostub.Stub(&getLatestTag, func(owner, repo string, currentMajorVer int) (string, error) {
			return "v1.0.0", nil
	})
	defer stub.Reset()
	stub.Stub(&cloneGithubRepo, func(owner, repo string, tag *string) (string, error) {
			return ".", nil
	})

	wt := newT(t)
	cwd, err := os.Getwd()
	require.NoError(t, err)

	err = moduleCheckIfDestructive(
			wt,
			"Azure", // Mock owner
			"terraform-module-test-helper", // Mock repo
			// Path to the example harness using the 'before' module structure (relative to repo root)
			"example/upgrade/example/version_upgrade",
			// Absolute path to the 'after' module code
			filepath.Join(cwd, "example", "after_upgrade"),
			terraform.Options{Upgrade: true},
			1, // Mock current major version
			&moduleUpgradeOptions{allowV0Testing: true}, // Options for the destructive check
	)

	require.NoError(t, err, "Expected no error for non-destructive changes, but got: %v", err)
}