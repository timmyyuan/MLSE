package diffcase

import "fmt"

func LocalRepoFolder(gitRepo string) string {
	return "/repo/" + gitRepo
}

func foo1(gitRepo string) string {
	cmd := fmt.Sprintf("rm -rf %s", LocalRepoFolder(gitRepo))
	return cmd
}

func F(gitRepo string) string {
	cmd := "rm -rf " + LocalRepoFolder(gitRepo)
	return cmd
}
