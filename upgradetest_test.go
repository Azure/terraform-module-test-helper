package terraform_module_test_helper

import (
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v42/github"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"golang.org/x/mod/semver"
)

func TestModuleUpgradeTest(t *testing.T) {
	stub := gostub.Stub(&getLatestTag, func(owner string, repo string, currentMajorVer int) (string, error) {
		return "v1.0.0", nil
	})
	defer stub.Reset()
	stub.Stub(&getTagCode, func(owner string, repo string, latestTag string) (string, error) {
		return "./", nil
	})
	err := moduleUpgrade(t, "lonegunmanb", "terraform-module-test-helper", "example/upgrade", "./", terraform.Options{Upgrade: true}, 1)
	if err == nil {
		assert.FailNow(t, "expect test failure, but test success")
	}
	if !strings.HasPrefix(err.Error(), "terraform configuration not idempotent") {
		assert.Failf(t, "not expected error, actual error is:%s", err.Error())
	}
}

func TestModuleUpgradeTestShouldSkipV0(t *testing.T) {
	stub := gostub.Stub(&getLatestTag, func(owner string, repo string, currentMajorVer int) (string, error) {
		return "v0.0.1", nil
	})
	defer stub.Reset()
	stub.Stub(&getTagCode, func(owner string, repo string, latestTag string) (string, error) {
		return "./", nil
	})
	err := moduleUpgrade(t, "lonegunmanb", "terraform-module-test-helper", "example/upgrade", "./", terraform.Options{Upgrade: true}, 0)
	assert.Equal(t, SkipV0Error, err)
}

func TestGetLatestTag(t *testing.T) {
	tag, err := getLatestTag("hashicorp", "terraform", 1)
	assert.Nil(t, err)
	assert.True(t, semver.IsValid(tag))
	assert.Equal(t, "v1", semver.Major(tag))
}

func TestSkipIfNoTagsWithinMajorVersion(t *testing.T) {
	v := os.TempDir()
	assert.NotEqual(t, "", v)
	_, err := getLatestTag("hashicorp", "terraform", 100)
	assert.Equal(t, CannotTestError, err)
}

func TestGetCurrentMajorVersionFromEnv_default(t *testing.T) {
	majorVersionFromEnv, err := GetCurrentMajorVersionFromEnv()
	assert.Nil(t, err)
	assert.Equal(t, 0, majorVersionFromEnv)
}

func TestGetCurrentMajorVersionFromEnv_basic(t *testing.T) {
	os.Setenv("PREVIOUS_MAJOR_VERSION", "v0")
	majorVersionFromEnv, err := GetCurrentMajorVersionFromEnv()
	assert.Nil(t, err)
	assert.Equal(t, 1, majorVersionFromEnv)
}

func TestTagWithAlphaSuffix(t *testing.T) {
	alpha := "v0.1.0-alpha"
	current := "0.1.1"
	alphaTag := &github.RepositoryTag{
		Name: &alpha,
	}
	currentTag := &github.RepositoryTag{
		Name: &current,
	}
	sort := semanticSort(wrap(alphaTag), wrap(currentTag))
	assert.False(t, sort)
}

func TestLatestTagWithAlphaSuffix(t *testing.T) {
	alphaVersion := "v0.1.0-alpha"
	latestVersion := "0.1.2"
	tags := []*github.RepositoryTag{
		{
			Name: &alphaVersion,
		},
		{
			Name: &latestVersion,
		},
	}
	first := latestTagWithinMajorVersion(tags, 0)
	assert.Equal(t, latestVersion, first.GetName())
}

func TestLatestTag(t *testing.T) {
	alphaVersion := "0.1.0"
	latestVersion := "0.1.2"
	tags := []*github.RepositoryTag{
		{
			Name: &alphaVersion,
		},
		{
			Name: &latestVersion,
		},
	}
	first := latestTagWithinMajorVersion(tags, 0)
	assert.Equal(t, latestVersion, first.GetName())
}

func TestNoValidVersion(t *testing.T) {
	v1 := "a.b.c"
	v2 := "e.f.g"
	tags := []*github.RepositoryTag{
		{
			Name: &v1,
		},
		{
			Name: &v2,
		},
	}
	first := latestTagWithinMajorVersion(tags, 0)
	assert.Nil(t, first)
}

func TestAddNewOutputShouldNotFailTheTest(t *testing.T) {
	tmpDir := test_structure.CopyTerraformFolderToTemp(t, "./example/output_upgrade", "test")
	err := diffTwoVersions(t, terraform.Options{
		Upgrade: true,
	}, tmpDir, "../after")
	assert.Nil(t, err)
}

func TestNoChange(t *testing.T) {
	tmpDir := test_structure.CopyTerraformFolderToTemp(t, "./example/output_upgrade", "test")
	err := diffTwoVersions(t, terraform.Options{
		Upgrade: true,
	}, tmpDir, "../before")
	assert.Nil(t, err)
}

func TestGetRepoCode(t *testing.T) {
	codePath, err := getTagCode("hashicorp", "go-getter", "v1.0.0")
	assert.Nil(t, err)
	_, err = os.Stat(codePath)
	assert.False(t, os.IsNotExist(err))
}
