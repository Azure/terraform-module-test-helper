package terraform_module_test_helper

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"os"
)

var _ FileParser = fileParser{}

type FileParser interface {
	Parse(path string) (*hcl.File, error)
}

type fileParser struct {

}

func (fileParser) Parse(path string) (*hcl.File, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, diag := hclsyntax.ParseConfig(bytes, path, hcl.Pos{})
	if diag.HasErrors() {
		return nil, diag
	}
	return f, nil
}


