package terraform_module_test_helper

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	terratest "github.com/gruntwork-io/terratest/modules/testing"
)

type TerraformOutput = map[string]interface{}

var ch1 = make(chan string)
var ch2 = make(chan string)

func init() {
	println("=> init")
	go record()
}

func record() {
	for {
		// 收到一个地址，打开临时文件，读取，关掉文件
		tempFileDir := <-ch1
		fmt.Printf("======== FileDir: [%s] ========\n", tempFileDir)

		//contents, err := os.ReadFile(tempFileDir)
		//if err != nil {
		//	log.Panicf("Failed reading file: %s", err)
		//}
		//fmt.Printf("\n%s", string(contents))

		ch2 <- "ok"
	}
}

var _ logger.TestLogger = &FileTestLogger{}

type FileTestLogger struct {
	// tempFilePath string
	stream STREAM
	ch chan
}

func (f *FileTestLogger) Logf(t terratest.TestingT, format string, args ...interface{}) {
	tempFile, err := os.OpenFile(f.tempFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	logger.DoLog(t, 3, tempFile, fmt.Sprintf(format, args...))
}

func NewFileTestLogger() *FileTestLogger {

}

func (f *FileTestLogger) OpenFile() error {
	// file stream
	exampleName := strings.Split(exampleRelativePath, "/")[1]
	tempFileDir := fmt.Sprintf("%s/test%s%d", os.TempDir(), exampleName, rand.Intn(100))
	tempFile, err := os.OpenFile(tempFileDir, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	f.stream = xxx
}

func (f *FileTestLogger) OpenMemory() error {
	// io buffer?
	f.stream = xxx
}

func (f *FileTestLogger) Close() {
	f.stream.close()
}

func (f *FileTestLogger) Listen() {
	// global make chan, go 监听 chan，打印到 stdout
	// init() 里面创建一个
	f.chan = xxx
	io.copy to os.Stdout
	release
}


func PrepareFile(exampleRelativePath string) string {
	exampleName := strings.Split(exampleRelativePath, "/")[1]
	tempFileDir := fmt.Sprintf("%s/test%s%d", os.TempDir(), exampleName, rand.Intn(100))
	tempFile, err := os.OpenFile(tempFileDir, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer tempFile.Close()
	return tempFileDir
}

func RunE2ETest(t *testing.T, moduleRootPath, exampleRelativePath string, option terraform.Options, assertion func(*testing.T, TerraformOutput)) {
	t.Parallel()
	option = retryableOptions(t, option)
	tmpDir := test_structure.CopyTerraformFolderToTemp(t, moduleRootPath, exampleRelativePath)
	if err := rewriteHcl(tmpDir, ""); err != nil {
		t.Fatalf(err.Error())
	}
	option.TerraformDir = tmpDir

	// create or open a temp log file
	tempFileDir := PrepareFile(exampleRelativePath)

	//option.Logger = logger.Terratest
	option.Logger = logger.New(FileTestLogger{tempFileDir})
	// defer file.close()

	// option.NoColor = true

	defer destroy1(t, tempFileDir, option)
	terraform.InitAndApply(t, &option)
	if err := initAndPlanAndIdempotentAtEasyMode(t, option); err != nil {
		t.Fatalf(err.Error())
	}
	if assertion != nil {
		assertion(t, terraform.OutputAll(t, removeLogger(option)))
	}

	// t.Failed()
}

func destroy1(t *testing.T, tempFileDir string, option terraform.Options) {
	defer printLog(tempFileDir)

	option.MaxRetries = 10
	option.TimeBetweenRetries = time.Minute
	option.RetryableTerraformErrors = map[string]string{
		".*": "Retry destroy on any error",
	}
	_, err := terraform.RunTerraformCommandE(t, &option, terraform.FormatArgs(&option, "destroy", "-auto-approve", "-input=false", "-refresh=false")...)
	require.NoError(t, err)
}

func printLog(tempFileDir string) {
	// 把原来创建的临时文件关掉
	// 把临时文件的地址发给 goroutine record()
	ch1 <- tempFileDir
	_ = <-ch2
}

func destroy(t *testing.T, option terraform.Options) {
	option.MaxRetries = 10
	option.TimeBetweenRetries = time.Minute
	option.RetryableTerraformErrors = map[string]string{
		".*": "Retry destroy on any error",
	}
	_, err := terraform.RunTerraformCommandE(t, &option, terraform.FormatArgs(&option, "destroy", "-auto-approve", "-input=false", "-refresh=false")...)
	require.NoError(t, err)
}

func removeLogger(option terraform.Options) *terraform.Options {
	// default logger might leak sensitive data
	option.Logger = logger.Discard
	return &option
}

func retryableOptions(t *testing.T, options terraform.Options) terraform.Options {
	result := terraform.WithDefaultRetryableErrors(t, &options)
	result.RetryableTerraformErrors[".*Please try again.*"] = "Service side suggest retry."
	return *result
}
