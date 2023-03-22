package terraform_module_test_helper

import (
	"os"
	"runtime"
	"strconv"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/timandy/routine"
)

func TestE2EExampleTest(t *testing.T) {
	RunE2ETest(t, "./", "example/basic", terraform.Options{
		Upgrade: true,
	}, func(t *testing.T, output TerraformOutput) {
		resId, ok := output["resource_id"].(string)
		assert.True(t, ok)
		assert.NotEqual(t, "", resId, "expected output `resource_id`")
	})
}

func TestE2EExample_WithoutIdempotent(t *testing.T) {
	currentId := routine.Goid()
	originStub := initAndPlanAndIdempotentAtEasyMode
	stub := gostub.Stub(&initAndPlanAndIdempotentAtEasyMode, func(t *testing.T, opts terraform.Options) error {
		// Do not impact other tests.
		id := routine.Goid()
		if id != currentId {
			return originStub(t, opts)
		}
		assert.FailNow(t, "should not be called")
		return nil
	})
	defer stub.Reset()
	RunE2ETestWithOption(t, "./", "example/basic",
		TestOptions{
			TerraformOptions: terraform.Options{
				Upgrade: true,
			},
			Assertion: func(t *testing.T, output TerraformOutput) {
				resId, ok := output["resource_id"].(string)
				assert.True(t, ok)
				assert.NotEqual(t, "", resId, "expected output `resource_id`")
			},
			SkipIdempotentCheck: true,
		})
}

func TestE2EExampleTest_setEnvWontCausePanicOnParallel(t *testing.T) {
	for i := 0; i < runtime.NumCPU(); i++ {
		iterator := i
		t.Run(strconv.Itoa(iterator), func(t *testing.T) {
			t.Setenv("TEST_ENV", strconv.Itoa(iterator))
			RunE2ETest(t, "./", "example/basic", terraform.Options{
				Upgrade: true,
			}, func(t *testing.T, output TerraformOutput) {
				assert.Equal(t, strconv.Itoa(iterator), os.Getenv("TEST_ENV"))
			})
		})
	}
}
