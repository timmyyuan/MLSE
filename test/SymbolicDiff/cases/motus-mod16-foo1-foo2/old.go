package diffcase

import "fmt"

func LocalRepoFolder(gitRepo string) string {
	return "/repo/" + gitRepo
}

func F(gitRepo string) string {
	cmd := fmt.Sprintf("rm -rf %s", LocalRepoFolder(gitRepo))
	return cmd
}

func foo2(gitRepo string) string {
	cmd := "rm -rf " + LocalRepoFolder(gitRepo)
	return cmd
}
