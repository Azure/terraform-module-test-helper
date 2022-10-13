package terraform_module_test_helper

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

type Module struct {
	*tfconfig.Module
	OutputExts   map[string]Output
	VariableExts map[string]Variable
	parser       FileParser
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
	for _, n := range fileNames {
		f, err := m.parser.Parse(n)
		if err != nil {
			return err
		}
		content, _, diag := f.Body.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{
					Type:       "output",
					LabelNames: []string{"name"},
				},
			},
		})
		if diag.HasErrors() {
			return diag
		}
		for _, b := range content.Blocks {
			attributes, diag := b.Body.JustAttributes()
			if diag.HasErrors() {
				return diag
			}
			o := Output{
				Name:  b.Labels[0],
				Range: b.DefRange,
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
	for _, n := range fileNames {
		f, err := m.parser.Parse(n)
		if err != nil {
			return err
		}
		content, _, diag := f.Body.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{
					Type:       "variable",
					LabelNames: []string{"name"},
				},
			},
		})
		if diag.HasErrors() {
			return diag
		}
		for _, b := range content.Blocks {
			attributes, diag := b.Body.JustAttributes()
			if diag.HasErrors() {
				return diag
			}
			v := Variable{
				Name:  b.Labels[0],
				Range: b.DefRange,
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
