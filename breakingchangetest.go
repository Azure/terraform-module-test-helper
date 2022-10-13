package terraform_module_test_helper

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/r3labs/diff/v3"
)

type ChangeCategory = string

const (
	variable ChangeCategory = "Variables"
	output   ChangeCategory = "Outputs"
)

type Change struct {
	diff.Change
	Category  ChangeCategory `json:"category"`
	Name      *string        `json:"name"`
	Attribute *string        `json:"attribute"`
}

type Module struct {
	*tfconfig.Module
	OutputExts map[string]Output
	parser     FileParser
}

type Output struct {
	Name        string
	Description *string
	Sensitive   *string
	Value       string
	Range       hcl.Range
}

func (m *Module) LoadOutputExts() error {
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
				Value: *attributeValueString(attributes["value"], f),
			}
			if desc, ok := attributes["description"]; ok {
				o.Description = attributeValueString(desc, f)
			}
			if sensitive, ok := attributes["sensitive"]; ok {
				o.Sensitive = attributeValueString(sensitive, f)
			}
			o.Range = hcl.Range{}
			m.OutputExts[b.Labels[0]] = o
		}
	}
	return nil
}

func BreakingChanges(m1 *Module, m2 *Module) ([]Change, error) {
	err := m1.LoadOutputExts()
	if err != nil {
		return nil, err
	}
	err = m2.LoadOutputExts()
	if err != nil {
		return nil, err
	}
	sanitizeModule(m1)
	sanitizeModule(m2)

	changelog, err := diff.Diff(m1.Module, m2.Module)
	if err != nil {
		return nil, err
	}
	outputChangeLogs, err := diff.Diff(m1.OutputExts, m2.OutputExts)
	if err != nil {
		return nil, err
	}
	linq.From(outputChangeLogs).Select(func(i interface{}) interface{} {
		l := i.(diff.Change)
		l.Path = append([]string{output}, l.Path...)
		return l
	}).ToSlice(&outputChangeLogs)
	changelog = append(changelog, outputChangeLogs...)
	return filterBreakingChanges(convert(changelog)), nil
}

func sanitizeModule(m *Module) {
	m.Path = ""
	for _, v := range m.Variables {
		v.Pos = *new(tfconfig.SourcePos)
	}
	for _, r := range m.ManagedResources {
		r.Pos = *new(tfconfig.SourcePos)
	}
	for _, r := range m.DataResources {
		r.Pos = *new(tfconfig.SourcePos)
	}
	m.Outputs = nil
}

func convert(cl diff.Changelog) (r []Change) {
	linq.From(cl).Select(func(i interface{}) interface{} {
		c := i.(diff.Change)
		var name, attribute *string
		if len(c.Path) > 1 {
			name = &c.Path[1]
		}
		if len(c.Path) > 2 {
			attribute = &c.Path[2]
		}
		return Change{
			Change: diff.Change{
				Type: c.Type,
				Path: c.Path,
				From: c.From,
				To:   c.To,
			},
			Category:  c.Path[0],
			Name:      name,
			Attribute: attribute,
		}
	}).ToSlice(&r)
	return
}

func filterBreakingChanges(cl []Change) []Change {
	variables := linq.From(cl).Where(func(i interface{}) bool {
		return i.(Change).Category == variable
	})
	outputs := linq.From(cl).Where(func(i interface{}) bool {
		return i.(Change).Category == output
	})
	variableChanges := breakingVariables(variables)
	outputChanges := breakingOutputs(outputs)
	return append(variableChanges, outputChanges...)
}

func breakingOutputs(outputs linq.Query) []Change {
	var r []Change
	deletedOutputs := outputs.Where(func(i interface{}) bool {
		c := i.(Change)
		return c.Type == "delete" && c.Attribute != nil && *c.Attribute == "Name"
	})
	valueChangedOutputs := outputs.Where(func(i interface{}) bool {
		c := i.(Change)
		return c.Type == "update" && c.Attribute != nil && (*c.Attribute == "Value")
	})
	sensitiveChangedOutputs := outputs.Where(func(i interface{}) bool {
		c := i.(Change)
		isSensitive := c.Type == "update" && c.Attribute != nil && *c.Attribute == "Sensitive"
		if !isSensitive {
			return false
		}
		switch c.To.(type) {
		// When add `sensitive` attribute
		case *string:
			return *(c.To.(*string)) == "true"
		// When change `senstitive`'s value
		case string:
			return c.To.(string) == "true"
		default:
			return false
		}
	})
	deletedOutputs.
		Concat(valueChangedOutputs).
		Concat(sensitiveChangedOutputs).ToSlice(&r)
	return r
}

func breakingVariables(variables linq.Query) []Change {
	var r []Change
	newVariables := variables.Where(isNewVariable)
	requiredNewVariables := groupByName(newVariables).Where(noDefaultValue)
	deletedVariables := variables.Where(func(i interface{}) bool {
		c := i.(Change)
		return c.Type == "delete" && c.Attribute != nil && *c.Attribute == "Name"
	})
	typeChangedVariables := variables.Where(func(i interface{}) bool {
		c := i.(Change)
		return c.Type == "update" && c.Attribute != nil && (*c.Attribute == "Type" && !isStringNilOrEmpty(c.To))
	})
	defaultValueBreakingChangeVariables := variables.Where(func(i interface{}) bool {
		c := i.(Change)
		return c.Type == "update" && c.Attribute != nil && (*c.Attribute == "Default" && c.From != nil)
	})
	requiredNewVariables.Select(recordForName).
		Concat(deletedVariables).
		Concat(typeChangedVariables).
		Concat(defaultValueBreakingChangeVariables).ToSlice(&r)
	return r
}

func recordForName(g interface{}) interface{} {
	return linq.From(g.(linq.Group).Group).FirstWith(func(i interface{}) bool {
		return i.(Change).Attribute != nil && *i.(Change).Attribute == "Name"
	})
}

func groupByName(newVariables linq.Query) linq.Query {
	return newVariables.GroupBy(func(i interface{}) interface{} {
		return *i.(Change).Name
	}, func(i interface{}) interface{} {
		return i
	})
}

func noDefaultValue(g interface{}) bool {
	return linq.From(g.(linq.Group).Group).All(func(i interface{}) bool {
		return i.(Change).Attribute == nil || *i.(Change).Attribute != "Default"
	})
}

func isNewVariable(i interface{}) bool {
	return i.(Change).Type == "create"
}

func isStringNilOrEmpty(i interface{}) bool {
	if i == nil {
		return true
	}
	s, ok := i.(string)
	return !ok || s == ""
}

func attributeValueString(a *hcl.Attribute, f *hcl.File) *string {
	s := string(a.Expr.Range().SliceBytes(f.Bytes))
	return &s
}
