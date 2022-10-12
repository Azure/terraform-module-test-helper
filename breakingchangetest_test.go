package terraform_module_test_helper

import (
	"fmt"
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

const basicOptionalVariable = `
variable "address_space" {
  type        = list(string)
  description = "The address space that is used by the virtual network."
  default     = ["10.0.0.0/16"]
}
`

const basicRequiredVariable = `
variable "vnet_name" {
  description = "Name of the vnet to create"
  type        = string
}
`

const basicOutput = `
output "vnet_subnets_name_id" {
  description = "Can be queried subnet-id by subnet name by using lookup(module.vnet.vnet_subnets_name_id, subnet1)"
  value       = local.azurerm_subnets
}
`

const basicResource = `
resource "azurerm_virtual_network" "vnet" {
  address_space       = var.address_space
  location            = var.vnet_location
  name                = var.vnet_name
  resource_group_name = var.resource_group_name
  dns_servers         = var.dns_servers
  tags                = var.tags
}
`

var tpl = strings.Join([]string{basicRequiredVariable, basicOptionalVariable, basicOutput, basicResource}, "\n")

func TestBreakingChange_NewRequiredVariableShouldBeBreakingChange(t *testing.T) {
	newVariableName := "vnet_location"
	newRequiredVariable := fmt.Sprintf(`
variable "%s" {
  description = "The location of the vnet to create."
  type        = string
  nullable    = false
}
`, newVariableName)
	newCode := fmt.Sprintf("%s\n%s", tpl, newRequiredVariable)
	oldModule := noError(t, func() (*tfconfig.Module, error) {
		return loadModuleByCode(tpl)
	})
	newModule := noError(t, func() (*tfconfig.Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "create", changes[0].Type)
	assert.Equal(t, newVariableName, *changes[0].Name)
	assert.Equal(t, "Name", *changes[0].Attribute)
}

func TestBreakingChange_NewOptionalVariableShouldNotBeBreakingChange(t *testing.T) {
	cases := []struct{
		code string
		name string
	}{{
		code: `
variable "vnet_location" {
  description = "The location of the vnet to create."
  type        = string
  nullable    = false
  default	  = "eastus"
}
`,
		name: "optionalVariableWithNullableArgument",
	}, {
		code: `
variable "vnet_location2" {
  description = "The location of the vnet to create."
  type        = string
  default	  = "eastus"
}
`,
		name: "optionalVariableWithoutNullableArgument",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newCode := fmt.Sprintf("%s\n%s", tpl, c.code)
			oldModule := noError(t, func() (*tfconfig.Module, error) {
				return loadModuleByCode(tpl)
			})
			newModule := noError(t, func() (*tfconfig.Module, error) {
				return loadModuleByCode(newCode)
			})
			changes := noError(t, func() ([]Change, error) {
				return BreakingChanges(oldModule, newModule)
			})
			assert.Equal(t, 0, len(changes))
		})
	}
}

func TestBreakingChange_RemoveVariableShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*tfconfig.Module, error) {
		return loadModuleByCode(tpl)
	})
	newCode :=strings.Join([]string{basicOutput, basicResource}, "\n")
	newModule := noError(t, func() (*tfconfig.Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 2, len(changes))
	assert.Equal(t, "delete", changes[0].Type)
	assert.Equal(t, "delete", changes[1].Type)
	assert.Equal(t, "Name", *changes[0].Attribute)
	assert.Equal(t, "Name", *changes[1].Attribute)
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		return *i.(Change).Name == "vnet_name"
	}))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		return *i.(Change).Name == "address_space"
	}))
}

func loadModuleByCode(code string) (*tfconfig.Module, error) {
	parser := hclparse.NewParser()
	file, diag := parser.ParseHCL([]byte(code), "main.tf")
	if diag.HasErrors() {
		return nil, diag
	}
	m := tfconfig.NewModule("")
	diag = tfconfig.LoadModuleFromFile(file, m)
	if diag.HasErrors() {
		return nil, diag
	}
	return m, nil
}

func noError[T any](t *testing.T, f func() (T, error)) T {
	r, err := f()
	assert.Nil(t, err)
	return r
}
