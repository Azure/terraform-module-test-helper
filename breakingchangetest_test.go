package terraform_module_test_helper

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

const basicVariable = `
variable "address_space" {
  type        = list(string)
  description = "The address space that is used by the virtual network."
  default     = ["10.0.0.0/16"]
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

var tpl = strings.Join([]string{basicVariable, basicOutput, basicVariable}, "\n")

func Test_NewRequiredVariableShouldReturnError(t *testing.T) {
	oldCode := tpl
	newVariableName := "vnet_location"
	newRequiredVariable := fmt.Sprintf(`
variable "%s" {
  description = "The location of the vnet to create."
  type        = string
  nullable    = false
}
`, newVariableName)
	newCode := fmt.Sprintf("%s\n%s", oldCode, newRequiredVariable)
	oldModule := noError(t, func() (*tfconfig.Module, error) {
		return loadModuleByCode(oldCode)
	})
	newModule := noError(t, func() (*tfconfig.Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]BreakingChange, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "create", changes[0].Type)
	assert.Equal(t, newVariableName, *changes[0].Name)
	assert.Equal(t, "Name", *changes[0].Attribute)
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
