package terraform_module_test_helper

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/files"
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

var _ logger.TestLogger = &StreamTestLogger{}

type StreamTestLogger struct {
	exampleName string
	stream      io.ReadWriter
	ch          chan string
}

func (f *StreamTestLogger) Logf(t terratest.TestingT, format string, args ...interface{}) {
	tempFile, err := os.OpenFile(f.tempFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	logger.DoLog(t, 3, tempFile, fmt.Sprintf(format, args...))
}

func NewStreamTestLogger(exName string, rw io.ReadWriter) *StreamTestLogger {
	return &StreamTestLogger{
		exampleName: exName,
		stream:      rw,
	}
}

func (f *StreamTestLogger) OpenFile() error {
	// file stream
	tempFilePath := fmt.Sprintf("%s/test%s%d", os.TempDir(), f.exampleName, rand.Intn(100))
	tempFile, err := os.OpenFile(tempFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	f.stream = make([]byte)
}

func (f *StreamTestLogger) OpenMemory() error {
	// io buffer?
	f.stream = new(bytes.Buffer)
}

func (f *StreamTestLogger) Close() {
	closer, ok := f.stream.(io.Closer)
	if ok == true {
		f.stream.Close()
	}
}

func (f *StreamTestLogger) Listen() {
	// init() 里面 global make chan, 新建 goroutine 监听 chan，打印到 stdout，release
	f.ch = make(chan string)
	io.Copy(os.Stdout, f.stream.(io.Reader))
	// release
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
	//tempFilePath := PrepareFile(exampleRelativePath)
	exampleName := strings.Split(exampleRelativePath, "/")[1]

	//option.Logger = logger.Terratest
	option.Logger = logger.New(&StreamTestLogger{
		exampleName: exampleName,
	})
	// defer file.close()

	// option.NoColor = true

	defer destroy1(t, tempFilePath, option)

	terraform.InitAndApply(t, &option)
	if err := initAndPlanAndIdempotentAtEasyMode(t, option); err != nil {
		t.Fatalf(err.Error())
	}
	if assertion != nil {
		assertion(t, terraform.OutputAll(t, removeLogger(option)))
	}

	// t.Failed()
}

func destroy1(t *testing.T, tempFilePath string, option terraform.Options) {
	defer printLog(tempFilePath)

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

	// t.Failed()
}

func destroy(t *testing.T, option terraform.Options) {
	path := option.TerraformDir
	if !files.IsExistingDir(path) || !files.FileExists(filepath.Join(path, "terraform.tfstate")) {
		return
	}

	option.MaxRetries = 5
	option.TimeBetweenRetries = time.Minute
	option.RetryableTerraformErrors = map[string]string{
		".*": "Retry destroy on any error",
	}
	_, err := terraform.RunTerraformCommandE(t, &option, terraform.FormatArgs(&option, "destroy", "-auto-approve", "-input=false", "-refresh=false")...)
	if err != nil {
		_, err = terraform.DestroyE(t, &option)
	}
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
