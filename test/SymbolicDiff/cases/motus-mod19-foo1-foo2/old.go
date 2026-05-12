package diffcase

import "fmt"

var customGoEnv = "CUSTOM_LEGO_BUILD_ENV"

func F(userEnv map[string]string, expriementArgs string) {
	userEnv[customGoEnv] = fmt.Sprintf("GOEXPERIMENT=%s", expriementArgs)
}

func foo2(userEnv map[string]string, expriementArgs string) {
	userEnv[customGoEnv] = "GOEXPERIMENT=" + expriementArgs
}
