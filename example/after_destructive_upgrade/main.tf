terraform {
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = ">= 3.0"
    }
  }
}

resource "null_resource" "test" {
  triggers = {
    version = "2"
  }
}

output "result" {
  value = null_resource.test.id
}
