package diffcase

import "fmt"

// logf models a side-effecting sink with a format string.
func logf(_ int, _ string, _ ...any) {}

// sink models a side-effecting external call.
func sink(_ string) {}

func foo1(ctx int, token, body string) {
	if body == "" {
		logf(ctx, "body empty: %s", body)
		return
	}
	auth := fmt.Sprintf("Codebase-User-JWT %s", token)
	sink(auth)
}

func F(ctx int, token, body string) {
	if body == "" {
		// Different constant format string should not drive inequivalence.
		logf(ctx, "empty body: %+v", body)
		return
	}
	auth := "Codebase-User-JWT " + token
	sink(auth)
}
