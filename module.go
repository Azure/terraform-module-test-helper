package terraform_module_test_helper

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/spf13/afero"
)

type Module struct {
	*tfconfig.Module
	OutputExts   map[string]Output
	VariableExts map[string]Variable
	dir          string
	fs           afero.Afero
}

type Output struct {
	Name        string
	Description string
	Sensitive   string
	Value       string
	Range       hcl.Range
}

type Variable struct {
	Name        string
	Type        string
	Description string
	Default     string
	Sensitive   string
	Nullable    string
	Range       hcl.Range
}

func NewModule(dir string, fs afero.Afero) (*Module, error) {
	m, diag := tfconfig.LoadModule(dir)
	if diag.HasErrors() {
		return nil, diag
	}
	return &Module{
		Module:       m,
		OutputExts:   make(map[string]Output),
		VariableExts: make(map[string]Variable),
		fs:           fs,
	}, nil
}

func (m *Module) Load() error {
	if err := m.LoadVariable(); err != nil {
		return err
	}
	if err := m.LoadOutput(); err != nil {
		return err
	}
	return nil
}

func (m *Module) LoadOutput() error {
	var fileNames []string
	linq.From(m.Outputs).Select(func(i interface{}) interface{} {
		return i.(linq.KeyValue).Value.(*tfconfig.Output).Pos.Filename
	}).Distinct().ToSlice(&fileNames)
	m.OutputExts = make(map[string]Output)
	parser := hclparse.NewParser()
	for _, n := range fileNames {
		content, err := m.fs.ReadFile(n)
		if err != nil {
			return err
		}
		f, diag := parser.ParseHCL(content, n)
		if diag.HasErrors() {
			return diag
		}
		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, b := range body.Blocks {
			if b.Type != "output" {
				continue
			}
			attributes := b.Body.Attributes
			o := Output{
				Name:  b.Labels[0],
				Range: b.Range(),
				Value: attributeValueString(attributes["value"], f),
			}
			if desc, ok := attributes["description"]; ok {
				o.Description = attributeValueString(desc, f)
			}
			if sensitive, ok := attributes["sensitive"]; ok {
				o.Sensitive = attributeValueString(sensitive, f)
			}
			// We don't compare position's change
			o.Range = hcl.Range{}
			m.OutputExts[b.Labels[0]] = o
		}
	}
	return nil
}

func (m *Module) LoadVariable() error {
	var fileNames []string
	linq.From(m.Variables).Select(func(i interface{}) interface{} {
		return i.(linq.KeyValue).Value.(*tfconfig.Variable).Pos.Filename
	}).Distinct().ToSlice(&fileNames)
	m.VariableExts = make(map[string]Variable)
	parser := hclparse.NewParser()
	for _, n := range fileNames {
		content, err := m.fs.ReadFile(n)
		if err != nil {
			return err
		}
		f, diag := parser.ParseHCL(content, n)
		if diag.HasErrors() {
			return diag
		}
		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, b := range body.Blocks {
			if b.Type != "variable" {
				continue
			}
			attributes := b.Body.Attributes
			v := Variable{
				Name:  b.Labels[0],
				Range: b.Range(),
			}
			if desc, ok := attributes["description"]; ok {
				v.Description = attributeValueString(desc, f)
			}
			if sensitive, ok := attributes["sensitive"]; ok {
				v.Sensitive = attributeValueString(sensitive, f)
			}
			if defaultValue, ok := attributes["default"]; ok {
				v.Default = attributeValueString(defaultValue, f)
			}
			if nullable, ok := attributes["nullable"]; ok {
				v.Nullable = attributeValueString(nullable, f)
			}
			if t, ok := attributes["type"]; ok {
				v.Type = attributeValueString(t, f)
			}
			// We don't compare position's change
			v.Range = hcl.Range{}
			m.VariableExts[b.Labels[0]] = v
		}
	}
	return nil
}
