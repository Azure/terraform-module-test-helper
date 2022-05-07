package terraform_module_test_helper

import (
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
		return "v0.0.1", nil
	})
	defer stub.Reset()
	stub.Stub(&getTagCode, func(owner string, repo string, latestTag string) (string, error) {
		return "./", nil
	})
	err := moduleUpgrade(t, "lonegunmanb", "terraform-module-test-helper", "example/upgrade", "./", terraform.Options{Upgrade: true}, 0)
	if err == nil {
		assert.FailNow(t, "expect test failure, but test success")
	}
	if !strings.HasPrefix(err.Error(), "terraform configuration not idempotent") {
		assert.Failf(t, "not expected error, actual error is:%s", err.Error())
	}
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
	assert.Equal(t, SkipError, err)
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

func TestTagWithAlphaSuffixt(t *testing.T) {
	alpha := "v0.1.0-alpha"
	current := "0.1.1"
	alphaTag := &github.RepositoryTag{
		Name: &alpha,
	}
	currentTag := &github.RepositoryTag{
		Name: &current,
	}
	sort := semanticSort(alphaTag, currentTag)
	assert.False(t, sort)
}
