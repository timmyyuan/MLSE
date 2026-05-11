package diffcase

import (
	"fmt"
	"net/http"
)

type HttpRequest struct {
	Url     string
	Headers map[string]string
	Method  string
	Body    string
}

const CodebaseAPIHOST = "https://codebase.example/"

func F(newRepo, authToken, jwt, body string) string {
	authHeader := fmt.Sprintf("Codebase-User-JWT %s", authToken)
	httpRequest := &HttpRequest{
		Url: CodebaseAPIHOST + newRepo,
		Headers: map[string]string{
			"authorization": authHeader,
			"content-type":  "application/json",
			"X-Jwt-Token":   jwt,
		},
		Method: http.MethodPost,
		Body:   body,
	}
	_ = httpRequest
	return authHeader
}

func foo2(newRepo, authToken, jwt, body string) string {
	authHeader := "Codebase-User-JWT " + authToken
	httpRequest := &HttpRequest{
		Url: CodebaseAPIHOST + newRepo,
		Headers: map[string]string{
			"authorization": authHeader,
			"content-type":  "application/json",
			"X-Jwt-Token":   jwt,
		},
		Method: http.MethodPost,
		Body:   body,
	}
	_ = httpRequest
	return authHeader
}
