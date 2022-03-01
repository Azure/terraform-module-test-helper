package terraform_module_test_helper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/ahmetb/go-linq/v3"
	"github.com/google/go-github/v42/github"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/hashicorp/go-getter"
	"golang.org/x/mod/semver"
)

func GetCurrentModuleRootPath() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(current, "..", "..")), nil
}

//goland:noinspection GoUnusedExportedFunction
func ModuleUpgradeTest(t *testing.T, owner, repo, moduleFolderRelativeToRoot, currentModulePath string, opts terraform.Options, currentMajorVer int) {
	err := moduleUpgrade(t, owner, repo, moduleFolderRelativeToRoot, currentModulePath, opts, currentMajorVer)
	if err == SkipError {
		t.Skipf(err.Error())
	}
	if err != nil {
		t.Errorf(err.Error())
	}
}

func moduleUpgrade(t *testing.T, owner string, repo string, moduleFolderRelativeToRoot string, currentModulePath string, opts terraform.Options, currentMajorVer int) error {
	latestTag, err := getLatestTag(owner, repo, currentMajorVer)
	if err != nil {
		return err
	}
	tmpDirForTag, err := getTagCode(owner, repo, latestTag)
	if err != nil {
		return err
	}

	tmpDir := test_structure.CopyTerraformFolderToTemp(t, tmpDirForTag, moduleFolderRelativeToRoot)
	opts.TerraformDir = tmpDir
	defer terraform.Destroy(t, &opts)

	terraform.InitAndApplyAndIdempotent(t, &opts)
	overrideModuleSourceToCurrentPath(t, tmpDir, currentModulePath)
	code := terraform.InitAndPlanWithExitCode(t, &opts)
	if code != 0 {
		return fmt.Errorf("terraform configuration not idempotent:%s", terraform.Plan(t, &opts))
	}
	return nil
}

func overrideModuleSourceToCurrentPath(t *testing.T, moduleDir string, currentModulePath string) {
	//goland:noinspection GoUnhandledErrorResult
	os.Setenv("MODULE_SOURCE", currentModulePath)
	tplt := template.Must(template.New("override.tf.tplt").Funcs(sprig.TxtFuncMap()).ParseFiles(filepath.Join(moduleDir, "override.tf.tplt")))
	dstOverrideTf := filepath.Join(moduleDir, "override.tf")
	if _, err := os.Stat(dstOverrideTf); err == nil {
		err = os.Remove(dstOverrideTf)
		if err != nil {
			t.Errorf("cannot delete dst override file:%s", err.Error())
		}
	}
	dstFs, err := os.Create(dstOverrideTf)
	if err != nil {
		t.Errorf("cannot create dst override file:%s", err.Error())
	}
	//goland:noinspection GoUnhandledErrorResult
	defer dstFs.Close()
	err = tplt.Execute(dstFs, err)
	if err != nil {
		t.Errorf("cannot write override.tf for %s:%s", dstOverrideTf, err.Error())
	}
}

var getTagCode = func(owner string, repo string, latestTag string) (string, error) {
	tmpDirForTag := filepath.Join(os.TempDir(), latestTag)
	err := getter.Get(tmpDirForTag, fmt.Sprintf("github.com/%s/%s?ref=%s", owner, repo, latestTag))
	if err != nil {
		return "", fmt.Errorf("cannot get module with tag:%s", err.Error())
	}
	return tmpDirForTag, nil
}

var SkipError = fmt.Errorf("no previous tag yet, skip upgrade test")

var getLatestTag = func(owner string, repo string, currentMajorVer int) (string, error) {
	client := github.NewClient(nil)
	tags, _, err := client.Repositories.ListTags(context.TODO(), owner, repo, nil)
	if err != nil {
		return "", err
	}
	if tags == nil {
		return "", fmt.Errorf("cannot find tags")
	}
	first := linq.From(tags).Where(valid).Sort(semanticSort).Where(sameMajorVersion(currentMajorVer)).First()
	if first == nil {
		return "", SkipError
	}
	latestTag := first.(*github.RepositoryTag).GetName()
	return latestTag, nil
}

func valid(t interface{}) bool {
	if t == nil {
		return false
	}
	tag := t.(*github.RepositoryTag)
	v := tag.GetName()
	return semver.IsValid(v) && !strings.Contains(v, "rc")
}

func semanticSort(i, j interface{}) bool {
	it := i.(*github.RepositoryTag)
	jt := j.(*github.RepositoryTag)
	return semver.Compare(it.GetName(), jt.GetName()) > 0
}

func sameMajorVersion(majorVersion int) func(i interface{}) bool {
	return func(i interface{}) bool {
		major := semver.Major(i.(*github.RepositoryTag).GetName())
		currentMajor := fmt.Sprintf("v%d", majorVersion)
		return major == currentMajor
	}
}
