package main

import (
	"os"

	helper "github.com/Azure/terraform-module-test-helper"
)

func main() {
	owner := os.Args[1]
	repo := os.Args[2]
	currentRepoPath := os.Args[3]

	output, err := helper.BreakingChangesDetect(owner, repo, currentRepoPath)
	if err != nil {
		panic(err.Error())
	}
	print(output)
}
