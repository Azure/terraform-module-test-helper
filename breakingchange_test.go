package terraform_module_test_helper

import (
	"fmt"
	"github.com/spf13/afero"
	"strconv"
	"strings"
	"testing"

	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/stretchr/testify/assert"
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
  nullable    = false
}
`

const unTypedVariable = `
variable "subnet_service_endpoints" {
  description = "A map of subnet name to service endpoints to add to the subnet."
  default     = {}
}
`

const variableWithValidation = `
variable "identity_type" {
  type        = string
  description = "this is a description."
  default     = "SystemAssigned"

  validation {
    condition     = var.identity_type == "SystemAssigned" || var.identity_type == "UserAssigned" || var.identity_type == "SystemAssigned, UserAssigned"
    error_message = "this is an error message."
  }
}`

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

const basicSensitiveVariable = `
variable "db_username" {
  description = "Database administrator username"
  type        = string
  sensitive   = true
}`

const jsonCode = `
{                
	"output": {                                                                                                                     
        "vnet_subnets_name_id": [                                                                                                   
            {                                                                                                                       
                "description": "Can be queried subnet-id by subnet name by using lookup(module.vnet.vnet_subnets_name_id, subnet1)",
                "value": "${local.azurerm_subnets}"                                                                                 
            }                                                                                                                       
        ]                                                                                                                           
    },   
    "variable": {                                           
        "vnet_name": [                                      
            {                                               
                "description": "Name of the vnet to create",
                "nullable": false,                          
                "type": "string"                         
            }
        ]
    }
}
`

var basicBlocks = []string{basicRequiredVariable, basicOptionalVariable, basicSensitiveVariable, variableWithValidation, unTypedVariable, basicOutput, basicResource}
var tpl = strings.Join(basicBlocks, "\n")

func TestBreakingChange_JsonFormat(t *testing.T) {
	newJsonCode := strings.ReplaceAll(jsonCode, "string", "number")
	newJsonCode = strings.ReplaceAll(newJsonCode, "local.azurerm_subnets", "local.azurerm_subnet_names")
	oldMoule := noError(t, func() (*Module, error) {
		return loadModuleByJsonCode(jsonCode)
	})
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByJsonCode(newJsonCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldMoule, newModule)
	})
	assert.Equal(t, 2, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return c.Type == "update" && c.Category == "Variables" && c.Attribute != nil && *c.Attribute == "Type"
	}))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return c.Type == "update" && c.Category == "Outputs" && c.Attribute != nil && *c.Attribute == "Value"
	}))
}

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
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	newModule := noError(t, func() (*Module, error) {
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
	cases := []struct {
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
			oldModule := noError(t, func() (*Module, error) {
				return loadModuleByCode(tpl)
			})
			newModule := noError(t, func() (*Module, error) {
				return loadModuleByCode(newCode)
			})
			changes := noError(t, func() ([]Change, error) {
				return BreakingChanges(oldModule, newModule)
			})
			assert.Empty(t, changes)
		})
	}
}

func TestBreakingChange_RemoveVariableShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	newCode := strings.Join(removeBlocks(basicBlocks, basicOptionalVariable, basicRequiredVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
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

func TestBreakingChange_ReorderVariablesShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	newCode := strings.Join([]string{unTypedVariable, basicOptionalVariable, basicRequiredVariable, variableWithValidation, basicOutput, basicResource, basicSensitiveVariable}, "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_RenameRequiredVariableShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	renamedVariable := `variable "renamed_name" {
  description = "Name of the vnet to create"
  type        = string
}`
	newCode := strings.Join(replaceString(basicBlocks, basicRequiredVariable, renamedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 2, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "vnet_name" && c.Type == "delete"
	}))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "renamed_name" && c.Type == "create"
	}))
}

func TestBreakingChange_RenameOptionalVariableShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	renamedVariable := `
variable "renamed_variable" {
  type        = list(string)
  description = "The address space that is used by the virtual network."
  default     = ["10.0.0.0/16"]
}`
	newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, renamedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "address_space" && c.Type == "delete"
	}))
}

func TestBreakingChange_RemoveVariableDefaultValueShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "address_space" {
  type        = list(string)
  description = "The address space that is used by the virtual network."
}`
	newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "address_space" && c.Type == "update" && *c.Attribute == "Default"
	}))
}

func TestBreakingChange_ChangeVariableDefaultValueShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "address_space" {
  type        = list(string)
  description = "The address space that is used by the virtual network."
  default     = ["192.168.0.0/16"]
}`
	newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "address_space" && c.Type == "update" && *c.Attribute == "Default"
	}))
}

func TestBreakingChange_AddVariableSensitiveShouldNotBeBreakingChange(t *testing.T) {
	sensitiveValues := []bool{true, false}
	for _, v := range sensitiveValues {
		t.Run(strconv.FormatBool(v), func(t *testing.T) {
			oldModule := noError(t, func() (*Module, error) {
				return loadModuleByCode(tpl)
			})
			changedVariable := fmt.Sprintf(`
variable "address_space" {
  type        = list(string)
  description = "The address space that is used by the virtual network."
  default     = ["10.0.0.0/16"]
  sensitive   = %t
}
`, v)
			newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, changedVariable), "\n")
			newModule := noError(t, func() (*Module, error) {
				return loadModuleByCode(newCode)
			})
			changes := noError(t, func() ([]Change, error) {
				return BreakingChanges(oldModule, newModule)
			})
			assert.Equal(t, 0, len(changes))
		})
	}
}

func TestBreakingChange_ChangeVariableSensitiveFromTrueToFalseShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "db_username" {
  description = "Database administrator username"
  type        = string
  sensitive   = false
}`
	newCode := strings.Join(replaceString(basicBlocks, basicSensitiveVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "db_username" && c.Type == "update" && *c.Attribute == "Sensitive"
	}))
}

func TestBreakingChange_RemoveVariableSensitiveShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "db_username" {
  description = "Database administrator username"
  type        = string
}`
	newCode := strings.Join(replaceString(basicBlocks, basicSensitiveVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "db_username" && c.Type == "update" && *c.Attribute == "Sensitive"
	}))
}

func TestBreakingChange_ChangeVariableNullableShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "vnet_name" {
  description = "Name of the vnet to create"
  type        = string
  nullable    = true
}`
	newCode := strings.Join(replaceString(basicBlocks, basicRequiredVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "vnet_name" && c.Type == "update" && *c.Attribute == "Nullable"
	}))
}

func TestBreakingChange_AddVariableTypeShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "subnet_service_endpoints" {
  description = "A map of subnet name to service endpoints to add to the subnet."
  type        = map(string)
  default     = {}
}
`
	newCode := strings.Join(replaceString(basicBlocks, unTypedVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "subnet_service_endpoints", *changes[0].Name)
	assert.Equal(t, "update", changes[0].Type)
	assert.Equal(t, "Type", *changes[0].Attribute)
}

func TestBreakingChange_RemoveVariableTypeShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "address_space" {
  description = "The address space that is used by the virtual network."
  default     = ["10.0.0.0/16"]
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_ChangeVariableTypeShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "address_space" {
  type        = set(string)
  description = "The address space that is used by the virtual network."
  default     = ["10.0.0.0/16"]
}`
	newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.True(t, linq.From(changes).AnyWith(func(i interface{}) bool {
		c := i.(Change)
		return *c.Name == "address_space" && c.Type == "update" && *c.Attribute == "Type"
	}))
}

func TestBreakingChange_RemoveVariableDescriptionShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "address_space" {
  type        = list(string)
  default     = ["10.0.0.0/16"]
}`
	newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_ChangeVariableDescriptionShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "address_space" {
  type        = list(string)
  description = "Changed description"
  default     = ["10.0.0.0/16"]
}`
	newCode := strings.Join(replaceString(basicBlocks, basicOptionalVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_AddVariableDefaultValueShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedVariable := `
variable "vnet_name" {
  description = "Name of the vnet to create"
  type        = string
  nullable    = false
  default     = "vnet"
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicRequiredVariable, changedVariable), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_NewOutputShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	newOutput := `
output "vnet_subnets_name_id" {
  description = "Can be queried subnet-id by subnet name by using lookup(module.vnet.vnet_subnets_name_id, subnet1)"
  value       = local.azurerm_subnets
}`
	newCode := fmt.Sprintf("%s\n%s", tpl, newOutput)
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_RemoveOutputBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	newCode := strings.Join(removeBlocks(basicBlocks, basicOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "delete", changes[0].Type)
	assert.Equal(t, "vnet_subnets_name_id", *changes[0].Name)
	assert.Equal(t, "Name", *changes[0].Attribute)
}

func TestBreakingChange_RenameOutputShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	renamedOutput := `
output "renamed_output" {
  description = "Can be queried subnet-id by subnet name by using lookup(module.vnet.vnet_subnets_name_id, subnet1)"
  value       = local.azurerm_subnets
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicOutput, renamedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "delete", changes[0].Type)
	assert.Equal(t, "vnet_subnets_name_id", *changes[0].Name)
	assert.Equal(t, "Name", *changes[0].Attribute)
}

func TestBreakingChange_ChangeOutputDescriptionShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	renamedOutput := `
output "vnet_subnets_name_id" {
  description = "changed description"
  value       = local.azurerm_subnets
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicOutput, renamedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_RemoveOutputDescriptionShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedOutput := `
output "vnet_subnets_name_id" {
  value       = local.azurerm_subnets
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicOutput, changedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_ChangeOutputValueShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedOutput := `
output "vnet_subnets_name_id" {
  description = "Can be queried subnet-id by subnet name by using lookup(module.vnet.vnet_subnets_name_id, subnet1)"
  value       = azurerm_subnet.main.id
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicOutput, changedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "update", changes[0].Type)
	assert.Equal(t, "vnet_subnets_name_id", *changes[0].Name)
	assert.Equal(t, "Value", *changes[0].Attribute)
}

func TestBreakingChange_AddOutputSensitiveTrueShouldBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedOutput := `
output "vnet_subnets_name_id" {
  description = "Can be queried subnet-id by subnet name by using lookup(module.vnet.vnet_subnets_name_id, subnet1)"
  value       = local.azurerm_subnets
  sensitive   = true
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicOutput, changedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "update", changes[0].Type)
	assert.Equal(t, "vnet_subnets_name_id", *changes[0].Name)
	assert.Equal(t, "Sensitive", *changes[0].Attribute)
}

func TestBreakingChange_AddOutputSensitiveFalseShouldNotBeBreakingChange(t *testing.T) {
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(tpl)
	})
	changedOutput := `
output "vnet_subnets_name_id" {
  description = "Can be queried subnet-id by subnet name by using lookup(module.vnet.vnet_subnets_name_id, subnet1)"
  value       = local.azurerm_subnets
  sensitive   = false
}
`
	newCode := strings.Join(replaceString(basicBlocks, basicOutput, changedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_ChangeOutputSensitiveToFalseShouldNotBeBreakingChange(t *testing.T) {
	sensitiveBlock := `
output "kube_admin_config_raw" {
  description = "A sensitive output"
  sensitive   = true
  value       = azurerm_kubernetes_cluster.main.kube_admin_config_raw
}
`
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(strings.Join(append(basicBlocks, sensitiveBlock), "\n"))
	})
	changedOutput := `
output "kube_admin_config_raw" {
  description = "A sensitive output"
  sensitive   = false
  value       = azurerm_kubernetes_cluster.main.kube_admin_config_raw
}
`
	newCode := strings.Join(append(basicBlocks, changedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Empty(t, changes)
}

func TestBreakingChange_ChangeOutputSensitiveToTrueShouldBeBreakingChange(t *testing.T) {
	sensitiveBlock := `
output "kube_admin_config_raw" {
  description = "A sensitive output"
  sensitive   = false
  value       = azurerm_kubernetes_cluster.main.kube_admin_config_raw
}
`
	oldModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(strings.Join(append(basicBlocks, sensitiveBlock), "\n"))
	})
	changedOutput := `
output "kube_admin_config_raw" {
  description = "A sensitive output"
  sensitive   = true
  value       = azurerm_kubernetes_cluster.main.kube_admin_config_raw
}
`
	newCode := strings.Join(append(basicBlocks, changedOutput), "\n")
	newModule := noError(t, func() (*Module, error) {
		return loadModuleByCode(newCode)
	})
	changes := noError(t, func() ([]Change, error) {
		return BreakingChanges(oldModule, newModule)
	})
	assert.Equal(t, 1, len(changes))
	assert.Equal(t, "kube_admin_config_raw", *changes[0].Name)
	assert.Equal(t, "update", changes[0].Type)
	assert.Equal(t, "Sensitive", *changes[0].Attribute)
}

func TestBreakingChange_RemoveOutputSensitiveShouldNotBeBreakingChange(t *testing.T) {
	values := []bool{true, false}
	for _, v := range values {
		t.Run(strconv.FormatBool(v), func(t *testing.T) {

			sensitiveBlock := fmt.Sprintf(`
output "kube_admin_config_raw" {
  description = "A sensitive output"
  sensitive   = %t
  value       = azurerm_kubernetes_cluster.main.kube_admin_config_raw
}
`, v)
			oldModule := noError(t, func() (*Module, error) {
				return loadModuleByCode(strings.Join(append(basicBlocks, sensitiveBlock), "\n"))
			})
			changedOutput := `
output "kube_admin_config_raw" {
  description = "A sensitive output"
  value       = azurerm_kubernetes_cluster.main.kube_admin_config_raw
}
`
			newCode := strings.Join(append(basicBlocks, changedOutput), "\n")
			newModule := noError(t, func() (*Module, error) {
				return loadModuleByCode(newCode)
			})
			changes := noError(t, func() ([]Change, error) {
				return BreakingChanges(oldModule, newModule)
			})
			assert.Empty(t, changes)
		})
	}
}

func loadModuleByCode(code string) (*Module, error) {
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
	mapFs := afero.NewMemMapFs()
	f, err := mapFs.Create("main.tf")
	if err != nil {
		return nil, err
	}
	_, err = f.WriteString(code)
	if err != nil {
		return nil, err
	}
	fs := afero.Afero{Fs: mapFs}
	return &Module{
		Module:       m,
		VariableExts: make(map[string]Variable),
		OutputExts:   make(map[string]Output),
		fs:           fs,
	}, nil
}

func loadModuleByJsonCode(json string) (*Module, error) {
	parser := hclparse.NewParser()
	file, diag := parser.ParseJSON([]byte(json), "main.tf.json")
	if diag.HasErrors() {
		return nil, diag
	}
	m := tfconfig.NewModule("")
	diag = tfconfig.LoadModuleFromFile(file, m)
	if diag.HasErrors() {
		return nil, diag
	}
	mapFs := afero.NewMemMapFs()
	f, err := mapFs.Create("main.tf.json")
	if err != nil {
		return nil, err
	}
	_, err = f.WriteString(json)
	if err != nil {
		return nil, err
	}
	fs := afero.Afero{Fs: mapFs}
	return &Module{
		Module:       m,
		VariableExts: make(map[string]Variable),
		OutputExts:   make(map[string]Output),
		fs:           fs,
	}, nil
}

func noError[T any](t *testing.T, f func() (T, error)) T {
	r, err := f()
	assert.Nil(t, err)
	return r
}

func removeBlocks(slice []string, itemsToRemove ...string) []string {
	var r []string
	linq.From(slice).Where(func(i interface{}) bool {
		for _, item := range itemsToRemove {
			if i.(string) == item {
				return false
			}
		}
		return true
	}).ToSlice(&r)
	return r
}

func replaceString(slice []string, old, new string) []string {
	return append(removeBlocks(slice, old), new)
}
