package terraform_module_test_helper

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/r3labs/diff/v3"
)

type ChangeCategory = string

const (
	Variable ChangeCategory = "Variables"
	Output   ChangeCategory = "Outputs"
)

type Change struct {
	diff.Change
	Category  ChangeCategory `json:"category"`
	Name      *string        `json:"name"`
	Attribute *string        `json:"attribute"`
}

func BreakingChanges(m1 *tfconfig.Module, m2 *tfconfig.Module) ([]Change, error) {
	processModule(m1)
	processModule(m2)
	changelog, err := diff.Diff(m1, m2)
	if err != nil {
		return nil, err
	}
	return filterBreakingChanges(convert(changelog)), nil
}

func processModule(m *tfconfig.Module) {
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
	for _, o := range m.Outputs {
		o.Pos = *new(tfconfig.SourcePos)
	}
}

func convert(cl diff.Changelog) (r []Change) {
	expected := linq.From([]string{"Variables", "Outputs"})
	linq.From(cl).Where(func(i interface{}) bool {
		c := i.(diff.Change)
		return expected.Contains(c.Path[0])
	}).Select(func(i interface{}) interface{} {
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
	var r []Change
	variables := linq.From(cl).Where(func(i interface{}) bool {
		return i.(Change).Category == Variable
	})
	newVariables := variables.Where(isNewVariable)
	requiredNewVariables := groupByName(newVariables).Where(noDefaultValue)
	requiredNewVariables.Select(recordForName).ToSlice(&r)
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
		return i.(Change).Attribute == nil || *i.(Change).Attribute != "default"
	})
}

func isNewVariable(i interface{}) bool {
	return i.(Change).Type == "create"
}
