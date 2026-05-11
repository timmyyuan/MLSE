package diffcase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"example.com/smtcmpmod24/logs"
	"example.com/smtcmpmod24/metrics"
	"example.com/smtcmpmod24/util_http"
	"example.com/smtcmpmod24/utils"
)

const CodebaseAPIHOST = "https://codebase.example"

const newRepo = "/repo"

func F(ctx context.Context, gitRepo, jwt, departmentId string, grootNodeId uint64, owners []string) (int, error) {
	authToken, err := GetAuthToken(ctx, jwt)
	if err != nil {
		return -1, err
	}
	reqBody := &CreateRepoReq{
		RepoName:     gitRepo,
		Platform:     "gitlab",
		DepartmentId: departmentId,
		RepoLevel:    "normal",
		GrootNodeId:  grootNodeId,
		Description:  "auto create by lego",
		ValidateOnly: false,
		InitialPerms: []InitialPerm{
			{Role: "owner", User: InitialPermUser{Username: "yetong.tony"}, ExpireDays: 3},
		},
	}

	for _, user := range owners {
		reqBody.InitialPerms = append(reqBody.InitialPerms, InitialPerm{
			Role:       "owner",
			User:       InitialPermUser{Username: user},
			ExpireDays: 0,
		})
	}
	body, _ := json.Marshal(reqBody)
	httpRequest := &util_http.HttpRequest{
		Url: CodebaseAPIHOST + newRepo,
		Headers: map[string]string{
			"authorization": fmt.Sprintf("Codebase-User-JWT %s", authToken),
			"content-type":  "application/json",
			"X-Jwt-Token":   jwt,
		},
		Method: http.MethodPost,
		Body:   body,
	}
	metrics.EmitThirdPartyInvoke(utils.GetPluginName(ctx), "codebase", "auth_token", 0)

	httpResp, err := httpRequest.SendWrapper()
	if err != nil {
		logs.CtxError(ctx, "[CreateRepo] get repo info failed, err: %+v", err)
		return -1, err
	}
	repoInfo := &CreateRepoResp{}
	if err := json.Unmarshal(httpResp.Body, repoInfo); err != nil {
		logs.CtxError(ctx, "unmarshal repo info failed, body: %s, err: %+v", httpResp.Body, err)
		return -1, err
	}
	logs.CtxInfo(ctx, "[CreateRepo] result body=%s", httpResp.Body)
	if httpResp.HttpCode == http.StatusCreated {
		return repoInfo.TicketID, nil
	}
	if httpResp.HttpCode == http.StatusBadRequest {
		logs.CtxError(ctx, "[CreateRepo] 参数错误: %s", httpResp.Body)
		return -1, errors.New(repoInfo.Message)
	}
	if httpResp.HttpCode == http.StatusForbidden {
		logs.CtxError(ctx, "[CreateRepo] 无权限。user_id=%v department_id=%v", owners, departmentId)
		return -1, fmt.Errorf("无权限，请检查参数: %s", repoInfo.Message)
	}
	if httpResp.HttpCode == http.StatusConflict {
		logs.CtxError(ctx, "[CreateRepo] 仓库已经存在: %s", gitRepo)
		return -1, fmt.Errorf("%s 仓库已经存在,err=%v", gitRepo, repoInfo.Message)
	}
	return -1, errors.New(repoInfo.Message)
}

func CreateRepo2(ctx context.Context, gitRepo, jwt, departmentId string, grootNodeId uint64, owners []string) (int, error) {
	authToken, err := GetAuthToken(ctx, jwt)
	if err != nil {
		return -1, err
	}
	reqBody := &CreateRepoReq{
		RepoName:     gitRepo,
		Platform:     "gitlab",
		DepartmentId: departmentId,
		RepoLevel:    "normal",
		GrootNodeId:  grootNodeId,
		Description:  "auto create by lego",
		ValidateOnly: false,
		InitialPerms: []InitialPerm{
			{Role: "owner", User: InitialPermUser{Username: "yetong.tony"}, ExpireDays: 3},
		},
	}

	for _, user := range owners {
		reqBody.InitialPerms = append(reqBody.InitialPerms, InitialPerm{
			Role:       "owner",
			User:       InitialPermUser{Username: user},
			ExpireDays: 0,
		})
	}
	body, _ := json.Marshal(reqBody)
	httpRequest := &util_http.HttpRequest{
		Url: CodebaseAPIHOST + newRepo,
		Headers: map[string]string{
			"authorization": "Codebase-User-JWT " + authToken,
			"content-type":  "application/json",
			"X-Jwt-Token":   jwt,
		},
		Method: http.MethodPost,
		Body:   body,
	}
	metrics.EmitThirdPartyInvoke(utils.GetPluginName(ctx), "codebase", "auth_token", 0)

	httpResp, err := httpRequest.SendWrapper()
	if err != nil {
		logs.CtxError(ctx, "[CreateRepo] get repo info failed, err: %+v", err)
		return -1, err
	}
	repoInfo := &CreateRepoResp{}
	if err := json.Unmarshal(httpResp.Body, repoInfo); err != nil {
		logs.CtxError(ctx, "unmarshal repo info failed, body: %s, err: %+v", httpResp.Body, err)
		return -1, err
	}
	logs.CtxInfo(ctx, "[CreateRepo] result body=%s", httpResp.Body)
	if httpResp.HttpCode == http.StatusCreated {
		return repoInfo.TicketID, nil
	}
	if httpResp.HttpCode == http.StatusBadRequest {
		logs.CtxError(ctx, "[CreateRepo] 参数错误: %s", httpResp.Body)
		return -1, errors.New(repoInfo.Message)
	}
	if httpResp.HttpCode == http.StatusForbidden {
		logs.CtxError(ctx, "[CreateRepo] 无权限。user_id=%v department_id=%v", owners, departmentId)
		return -1, fmt.Errorf("无权限，请检查参数: %s", repoInfo.Message)
	}
	if httpResp.HttpCode == http.StatusConflict {
		logs.CtxError(ctx, "[CreateRepo] 仓库已经存在: %s", gitRepo)
		return -1, fmt.Errorf("%s 仓库已经存在,err=%v", gitRepo, repoInfo.Message)
	}
	return -1, errors.New(repoInfo.Message)
}

type CreateRepoReq struct {
	RepoName     string        `json:"repo_name"`
	Platform     string        `json:"platform"`
	DepartmentId string        `json:"department_id"`
	RepoLevel    string        `json:"repo_level"`
	GrootNodeId  uint64        `json:"groot_node_id"`
	Description  string        `json:"description"`
	ValidateOnly bool          `json:"validate_only"`
	InitialPerms []InitialPerm `json:"initial_perms"`
}

type InitialPerm struct {
	Role       string          `json:"role"`
	User       InitialPermUser `json:"user"`
	ExpireDays int             `json:"expire_days"`
}

type InitialPermUser struct {
	Username string `json:"username"`
}

type CreateRepoResp struct {
	TicketID int    `json:"ticket_id"`
	Message  string `json:"message"`
}

func GetAuthToken(ctx context.Context, jwt string) (string, error) {
	if jwt == "" {
		return "", errors.New("missing jwt")
	}
	return "token", nil
}
