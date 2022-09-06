package terraform_module_test_helper

import (
	"context"
	"errors"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/logger"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/ahmetb/go-linq/v3"
	"github.com/google/go-github/v42/github"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/hashicorp/go-getter/v2"
	"github.com/hashicorp/terraform-json"
	"golang.org/x/mod/semver"
)

var CannotTestError = fmt.Errorf("no previous tag yet or previous tag's folder structure is different than the current version, skip upgrade test")
var SkipV0Error = fmt.Errorf("v0 is meant to be unstable, skip upgrade test")

type repositoryTag struct {
	*github.RepositoryTag
	version string
}

//goland:noinspection GoUnusedExportedFunction
func ModuleUpgradeTest(t *testing.T, owner, repo, moduleFolderRelativeToRoot, currentModulePath string, opts terraform.Options, currentMajorVer int) {
	err := moduleUpgrade(t, owner, repo, moduleFolderRelativeToRoot, currentModulePath, retryableOptions(t, opts), currentMajorVer)
	if err == CannotTestError || err == SkipV0Error {
		t.Skipf(err.Error())
	}
	if err != nil {
		t.Fatalf(err.Error())
	}
}

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

func moduleUpgrade(t *testing.T, owner string, repo string, moduleFolderRelativeToRoot string, newModulePath string, opts terraform.Options, currentMajorVer int) error {
	latestTag, err := getLatestTag(owner, repo, currentMajorVer)
	if err != nil {
		return err
	}
	if semver.Major(latestTag) == "v0" {
		return SkipV0Error
	}
	tmpDirForTag, err := getTagCode(owner, repo, latestTag)
	if err != nil {
		return err
	}

	fullTerraformModuleFolder := filepath.Join(tmpDirForTag, moduleFolderRelativeToRoot)

	exists := files.FileExists(fullTerraformModuleFolder)
	if !exists {
		return CannotTestError
	}
	tmpTestDir := test_structure.CopyTerraformFolderToTemp(t, tmpDirForTag, moduleFolderRelativeToRoot)
	return diffTwoVersions(t, opts, tmpTestDir, newModulePath)
}

func diffTwoVersions(t *testing.T, opts terraform.Options, originTerraformDir string, newModulePath string) error {
	opts.TerraformDir = originTerraformDir
	defer terraform.Destroy(t, &opts)
	terraform.InitAndApplyAndIdempotent(t, &opts)
	overrideModuleSourceToCurrentPath(t, originTerraformDir, newModulePath)
	return initAndPlanAndIdempotentAtEasyMode(t, opts)
}

func initAndPlanAndIdempotentAtEasyMode(t *testing.T, opts terraform.Options) error {
	opts.PlanFilePath = filepath.Join(opts.TerraformDir, "tf.plan")
	opts.Logger = logger.Discard
	exitCode := terraform.InitAndPlanWithExitCode(t, &opts)
	plan := terraform.InitAndPlanAndShowWithStruct(t, &opts)
	changes := plan.ResourceChangesMap
	if exitCode == 0 || noChange(changes) {
		return nil
	}
	return fmt.Errorf("terraform configuration not idempotent:%s", terraform.Plan(t, &opts))
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

func overrideModuleSourceToCurrentPath(t *testing.T, moduleDir string, currentModulePath string) {
	//goland:noinspection GoUnhandledErrorResult
	os.Setenv("MODULE_SOURCE", currentModulePath)
	err := renderOverrideFile(moduleDir)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func renderOverrideFile(moduleDir string) error {
	templatePath := filepath.Join(moduleDir, "override.tf.tplt")
	if _, err := os.Stat(templatePath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	tplt := template.Must(template.New("override.tf.tplt").Funcs(sprig.TxtFuncMap()).ParseFiles(templatePath))
	dstOverrideTf := filepath.Join(moduleDir, "override.tf")
	dstFs, err := createOrOverwrite(dstOverrideTf)
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer dstFs.Close()
	if err = tplt.Execute(dstFs, err); err != nil {
		return fmt.Errorf("cannot write override.tf for %s:%s", dstOverrideTf, err.Error())
	}
	return nil
}

func createOrOverwrite(dstOverrideTf string) (*os.File, error) {
	if err := removeIfExist(dstOverrideTf); err != nil {
		return nil, err
	}
	dstFs, err := os.Create(filepath.Clean(dstOverrideTf))
	if err != nil {
		return nil, fmt.Errorf("cannot create dst override file:%s", err.Error())
	}
	return dstFs, nil
}

func removeIfExist(dstOverrideTf string) error {
	if _, err := os.Stat(dstOverrideTf); err == nil {
		err = os.Remove(dstOverrideTf)
		if err != nil {
			return fmt.Errorf("cannot delete dst override file:%s", err.Error())
		}
	}
	return nil
}

var getTagCode = func(owner string, repo string, latestTag string) (string, error) {
	tmpDirForTag := filepath.Join(os.TempDir(), owner, repo, latestTag)
	_, err := getter.Get(context.TODO(), tmpDirForTag, fmt.Sprintf("github.com/%s/%s?ref=%s", owner, repo, latestTag))
	if err != nil {
		return "", fmt.Errorf("cannot get module with tag:%s", err.Error())
	}
	return tmpDirForTag, nil
}

var getLatestTag = func(owner string, repo string, currentMajorVer int) (string, error) {
	client := github.NewClient(nil)
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
