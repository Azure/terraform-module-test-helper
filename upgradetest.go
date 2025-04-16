package terraform_module_test_helper

import (
	"context"
	"encoding/json"	
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ahmetb/go-linq/v3"
	"github.com/google/go-github/v42/github"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	terratest "github.com/gruntwork-io/terratest/modules/testing"
	"github.com/hashicorp/go-getter/v2"
	"github.com/hashicorp/terraform-json"
	"github.com/lonegunmanb/tfmodredirector"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

var CannotTestError = fmt.Errorf("no previous tag yet or previous tag's folder structure is different than the current version, skip upgrade test")
var SkipV0Error = fmt.Errorf("v0 is meant to be unstable, skip upgrade test")

type repositoryTag struct {
	*github.RepositoryTag
	version string
}

//goland:noinspection GoUnusedExportedFunction
func ModuleUpgradeTest(t *testing.T, owner, repo, moduleFolderRelativeToRoot, currentModulePath string, opts terraform.Options, currentMajorVer int, allowV0Testing bool, streamOutput bool) {
	wrappedT := newT(t)
	tryParallel(wrappedT)
	logger.Log(wrappedT, fmt.Sprintf("===> Starting test for %s/%s/examples/%s, ", owner, repo, moduleFolderRelativeToRoot))
	
	if streamOutput {
		opts.Logger = logger.Default
	} else {
		logger.Log(wrappedT, "Since we're running tests in parallel, the test log will be buffered and output to stdout after the tests are finished.")
		memLogger := NewMemoryLogger()
		// Important: Use logger.New to wrap our StreamLogger to avoid type incompatibility
		opts.Logger = logger.New(memLogger)
		// Ensure we flush the logs at the end
		defer func() { 
			if err := memLogger.Close(); err != nil {
				t.Logf("Error closing memory logger: %v", err)
			}
		}()
	}

	opts = setupRetryLogic(opts)

	err := moduleUpgrade(wrappedT, owner, repo, moduleFolderRelativeToRoot, currentModulePath, retryableOptions(t, opts), currentMajorVer, allowV0Testing)
	if err == CannotTestError || err == SkipV0Error {
		t.Skip(err.Error())
	}
	require.NoError(wrappedT, err)
}

func tryParallel(t testingT) {
	defer func() {
		if recover() != nil {
			t.T().Log("cannot run test in parallel, skip parallel")
		}
	}()
	t.T().Parallel()
}

func setupRetryLogic(opts terraform.Options) terraform.Options {
	if len(opts.RetryableTerraformErrors) == 0 {
		return opts
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 10
	}
	if opts.TimeBetweenRetries == time.Duration(0) {
		opts.TimeBetweenRetries = time.Minute
	}
	return opts
}

//goland:noinspection GoUnusedExportedFunction
func GetCurrentModuleRootPath() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(current, "..", "..")), nil
}

func GetCurrentMajorVersionFromEnv() (int, error) {
	currentMajorVer := os.Getenv("PREVIOUS_MAJOR_VERSION")
	if currentMajorVer == "" {
		return 0, nil
	}
	previousMajorVer, err := strconv.Atoi(strings.TrimPrefix(currentMajorVer, "v"))
	if err != nil {
		return 0, err
	}
	return previousMajorVer + 1, nil
}

func wrap(t interface{}) interface{} {
	tag := t.(*github.RepositoryTag)
	return repositoryTag{
		RepositoryTag: tag,
		version:       sterilize(tag.GetName()),
	}
}

func unwrap(t interface{}) interface{} {
	return t.(repositoryTag).RepositoryTag
}

func moduleUpgrade(t *T, owner string, repo string, moduleFolderRelativeToRoot string, newModulePath string, opts terraform.Options, currentMajorVer int, allowV0Testing bool) error {
	if !allowV0Testing && currentMajorVer == 0 {
		return SkipV0Error
	}
	latestTag, err := getLatestTag(owner, repo, currentMajorVer)
	if err != nil {
		return err
	}
	if !allowV0Testing && semver.Major(latestTag) == "v0" {
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
	return diffTwoVersions(t, opts, tmpTestDir, newModulePath)
}

func diffTwoVersions(t *T, opts terraform.Options, originTerraformDir string, newModulePath string) error {
	opts.TerraformDir = originTerraformDir
	defer destroy(t, opts)
	initAndApply(t, &opts)
	overrideModuleSourceToCurrentPath(t, originTerraformDir, newModulePath)
	return initAndPlanAndIdempotentAtEasyMode(t, opts)
}

var initAndPlanAndIdempotentAtEasyMode = func(t testingT, opts terraform.Options) error {
	opts.PlanFilePath = filepath.Join(opts.TerraformDir, "tf.plan")
	opts.Logger = logger.Discard
	exitCode := initAndPlanWithExitCode(t, &opts)
	plan := terraform.InitAndPlanAndShowWithStruct(t, &opts)
	changes := plan.ResourceChangesMap

	opts.Logger = logger.Default
	jsonBytes, _ := json.MarshalIndent(changes, "", "  ")
	logger.Log(t, string(jsonBytes))

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

func noChange(changes map[string]*tfjson.ResourceChange) bool {
	if len(changes) == 0 {
		return true
	}
	return linq.From(changes).Select(func(i interface{}) interface{} {
		return i.(linq.KeyValue).Value
	}).All(func(i interface{}) bool {
		change := i.(*tfjson.ResourceChange).Change
		if change == nil {
			return true
		}
		if change.Actions == nil {
			return true
		}
		return change.Actions.NoOp()
	})
}

func overrideModuleSourceToCurrentPath(t *T, moduleDir string, currentModulePath string) {
	require.NoError(t, rewriteHcl(moduleDir, currentModulePath))
}

func rewriteHcl(moduleDir, newModuleSource string) error {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}
		filePath := filepath.Clean(filepath.Join(moduleDir, entry.Name()))
		f, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		tfCode := string(f)
		tfCode, err = tfmodredirector.RedirectModuleSource(tfCode, "../../", newModuleSource)
		if err != nil {
			return err
		}
		tfCode, err = tfmodredirector.RedirectModuleSource(tfCode, "../..", newModuleSource)
		if err != nil {
			return err
		}
		err = os.WriteFile(filePath, []byte(tfCode), 0600)
		if err != nil {
			return err
		}
	}
	return nil
}

var cloneGithubRepo = func(owner string, repo string, tag *string) (string, error) {
	repoUrl := fmt.Sprintf("github.com/%s/%s", owner, repo)
	dirPath := []string{os.TempDir(), owner, repo}
	if tag != nil {
		dirPath = append(dirPath, *tag)
		repoUrl = fmt.Sprintf("%s?ref=%s", repoUrl, *tag)
	}
	tmpDir := filepath.Join(dirPath...)
	_, err := getter.Get(context.TODO(), tmpDir, repoUrl)
	if err != nil {
		return "", fmt.Errorf("cannot clone repo:%s", err.Error())
	}
	return tmpDir, nil
}

var getLatestTag = func(owner string, repo string, currentMajorVer int) (string, error) {
	client := github.NewClient(githubClient())
	tags, _, err := client.Repositories.ListTags(context.TODO(), owner, repo, nil)
	if err != nil {
		return "", err
	}
	if tags == nil {
		return "", fmt.Errorf("cannot find tags")
	}
	first := latestTagWithinMajorVersion(tags, currentMajorVer)
	if first == nil {
		return "", CannotTestError
	}
	latestTag := first.GetName()
	return latestTag, nil
}

func githubClient() *http.Client {
	var httpClient *http.Client
	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: ghToken},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	}
	return httpClient
}

func latestTagWithinMajorVersion(tags []*github.RepositoryTag, currentMajorVer int) *github.RepositoryTag {
	t := linq.From(tags).Where(notNil).Select(wrap).Where(valid).Sort(bySemantic).Where(sameMajorVersion(currentMajorVer)).Select(unwrap).First()
	if t == nil {
		return nil
	}
	return t.(*github.RepositoryTag)
}

func notNil(i interface{}) bool {
	return i != nil
}

func valid(t interface{}) bool {
	if t == nil {
		return false
	}
	tag := t.(repositoryTag)
	v := tag.version
	return semver.IsValid(v) && !strings.Contains(v, "rc")
}

func bySemantic(i, j interface{}) bool {
	it := i.(repositoryTag)
	jt := j.(repositoryTag)
	return semver.Compare(it.version, jt.version) > 0
}

func sterilize(v string) string {
	if !strings.HasPrefix(v, "v") {
		return fmt.Sprintf("v%s", v)
	}
	return v
}

func sameMajorVersion(majorVersion int) func(i interface{}) bool {
	return func(i interface{}) bool {
		major := semver.Major(i.(repositoryTag).version)
		currentMajor := fmt.Sprintf("v%d", majorVersion)
		return major == currentMajor
	}
}

func initAndPlanWithExitCode(t terratest.TestingT, options *terraform.Options) int {
	tfInit(t, options)
	return terraform.PlanExitCode(t, options)
}
