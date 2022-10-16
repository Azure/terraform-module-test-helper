package main

import (
	"os"

	helper "github.com/Azure/terraform-module-test-helper"
)

func main() {
	currentRepoPath := os.Args[1]
	owner := os.Args[2]
	repo := os.Args[3]
	var targetRef *string
	if len(os.Args) > 4 {
		targetRef = &os.Args[4]
	}

	output, err := helper.BreakingChangesDetect(currentRepoPath, owner, repo, targetRef)
	if err != nil {
		panic(err.Error())
	}
	print(output)
}
