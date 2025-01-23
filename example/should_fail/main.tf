terraform {
  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "3.6.3"
    }
  }
}
resource "random_string" "test" {
  length = 10
  lifecycle {
    postcondition {
      condition       = length(self.result) != 10
      error_message   = "must fail"
    }
  }
}