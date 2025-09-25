package terraform_module_test_helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	//terratest "github.com/gruntwork-io/terratest/modules/testing"
	"github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

type moduleUpgradeOptions struct {
	allowV0Testing bool
}

//goland:noinspection GoUnusedExportedFunction
func ModuleUpgradeDestructiveTest(t *testing.T, owner, repo, moduleFolderRelativeToRoot, currentModulePath string, opts terraform.Options, currentMajorVer int) {
	wrappedT := newT(t)
	tryParallel(wrappedT)
	logger.Log(wrappedT, fmt.Sprintf("===> Starting test for destructive changes in %s/%s/examples/%s, ", owner, repo, moduleFolderRelativeToRoot))

	streamEnv := os.Getenv("DEBUG_ENABLE_LOG_STREAMING")
	stream, _ := strconv.ParseBool(streamEnv) // Defaults to false if parsing fails or var is not set

	if stream {
		opts.Logger = logger.Default
	} else {
		memLogger := NewMemoryLogger()
		opts.Logger = logger.New(memLogger)
		defer func() {
			if err := memLogger.Close(); err != nil {
				t.Logf("Error closing memory logger: %v", err)
			}
		}()
	}

	opts = setupRetryLogic(opts)

	upgradeOpts := &moduleUpgradeOptions{
		allowV0Testing: true,
	}

	err := moduleCheckIfDestructive(wrappedT, owner, repo, moduleFolderRelativeToRoot, currentModulePath, retryableOptions(t, opts), currentMajorVer, upgradeOpts)
	if err == CannotTestError {
		t.Skip(err.Error())
	}
	require.NoError(wrappedT, err)
}

func moduleCheckIfDestructive(t *T, owner string, repo string, moduleFolderRelativeToRoot string, newModulePath string, opts terraform.Options, currentMajorVer int, upgradeOpts *moduleUpgradeOptions) error {
	if !upgradeOpts.allowV0Testing && currentMajorVer == 0 {
		return SkipV0Error
	}
	latestTag, err := getLatestTag(owner, repo, currentMajorVer)
	if err != nil {
		return err
	}
	if !upgradeOpts.allowV0Testing && semver.Major(latestTag) == "v0" {
		return SkipV0Error
	}
	tmpDirForTag, err := cloneGithubRepo(owner, repo, &latestTag)
	if err != nil {
		return err
	}

	fullTerraformModuleFolder := filepath.Join(tmpDirForTag, moduleFolderRelativeToRoot)

	exists := files.FileExists(fullTerraformModuleFolder)
	if !exists {
		return CannotTestError
	}
	tmpTestDir := test_structure.CopyTerraformFolderToTemp(t, tmpDirForTag, moduleFolderRelativeToRoot)
	defer func() {
		_ = os.RemoveAll(filepath.Clean(tmpTestDir))
	}()
	return diffForDestructiveChanges(t, opts, tmpTestDir, newModulePath)
}

func diffForDestructiveChanges(t *T, opts terraform.Options, originTerraformDir string, newModulePath string) error {
	opts.TerraformDir = originTerraformDir
	defer destroy(t, opts)
	initAndApply(t, &opts)
	overrideModuleSourceToCurrentPath(t, originTerraformDir, newModulePath)
	return initAndPlanAndNotDestructiveChanges(t, opts)
}

var initAndPlanAndNotDestructiveChanges = func(t testingT, opts terraform.Options) error {
	opts.PlanFilePath = filepath.Join(opts.TerraformDir, "tf.plan")
	opts.Logger = logger.Discard
	exitCode := initAndPlanWithExitCode(t, &opts)
	plan := terraform.InitAndPlanAndShowWithStruct(t, &opts)
	changes := plan.ResourceChangesMap

	// opts.Logger = logger.Default
	// jsonBytes, _ := json.MarshalIndent(changes, "", "  ")
	// logger.Log(t, string(jsonBytes))

	if exitCode == 0 || ignoreOnlyAddOrUpdate(changes) {
		return nil
	}
	return fmt.Errorf("terraform configuration contains destructive operations (breaking changes):%s", terraform.Plan(t, &opts))
}

func ignoreOnlyAddOrUpdate(changes map[string]*tfjson.ResourceChange) bool {
	// Return false (fail) if we see any "delete" or "replace."
	for _, rc := range changes {
			if rc == nil || rc.Change == nil {
					continue
			}
			actions := rc.Change.Actions
			if actions == nil {
					continue
			}
			// If Terraform lists multiple actions (e.g., ["delete", "create"] for a replace),
			// then if any action is "delete" or "replace," we fail.
			for _, a := range actions {
					if a == "delete" || a == "replace" {
							return false
					}
			}
	}
	return true
}
