package googleintegration

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Google errors can contain request data and identifiers. Expose only known
// machine-readable reasons and our own guidance, never the raw response body.
func googleAPIError(service string, status int, body io.Reader) error {
	var payload struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&payload)
	reasons := []string{}
	for _, detail := range payload.Error.Details {
		reasons = append(reasons, detail.Reason)
	}
	for _, item := range payload.Error.Errors {
		reasons = append(reasons, item.Reason)
	}
	reasons = append(reasons, payload.Error.Status)
	guidance := map[string]string{
		"SERVICE_DISABLED":                "해당 OAuth 클라이언트의 Google Cloud 프로젝트에서 이 서비스의 API를 사용 설정하세요. 활성화 직후라면 잠시 후 다시 시도하세요. Enable this service's API in the Google Cloud project owning the OAuth client.",
		"accessNotConfigured":             "해당 OAuth 클라이언트의 Google Cloud 프로젝트에서 이 서비스의 API를 사용 설정하세요. Enable this service's API in the Google Cloud project owning the OAuth client.",
		"ACCESS_TOKEN_SCOPE_INSUFFICIENT": "이 서비스의 읽기 권한이 부족합니다. 계정 관리에서 서비스를 체크하고 Google 연결 / 권한 업데이트를 다시 진행하세요. Reconnect Google and grant the selected service's read permission.",
		"insufficientPermissions":         "이 서비스의 읽기 권한이 부족합니다. 계정 관리에서 서비스를 체크하고 Google 연결 / 권한 업데이트를 다시 진행하세요. Reconnect Google and grant the selected service's read permission.",
		"domainPolicy":                    "Google Workspace 관리자 정책이 접근을 차단했습니다. 조직 관리자에게 앱 접근 허용을 요청하세요. Ask the Workspace administrator to allow this app.",
		"rateLimitExceeded":               "API 요청 빈도 제한에 도달했습니다. 잠시 후 다시 시도하세요. Rate limit reached; retry later.",
		"userRateLimitExceeded":           "사용자별 API 요청 빈도 제한에 도달했습니다. 잠시 후 다시 시도하세요. User rate limit reached; retry later.",
		"dailyLimitExceeded":              "프로젝트의 일일 API 할당량에 도달했습니다. Google Cloud에서 할당량을 확인하세요. Check the project's daily API quota.",
		"quotaExceeded":                   "API 할당량에 도달했습니다. Google Cloud에서 할당량을 확인하세요. Check the project's API quota.",
		"RESOURCE_EXHAUSTED":              "API 할당량 또는 요청 빈도 제한에 도달했습니다. Google Cloud에서 할당량을 확인하세요. Check API quotas and retry later.",
	}
	// Some responses omit ErrorInfo but retain the standard diagnostic message.
	lower := strings.ToLower(payload.Error.Message)
	if strings.Contains(lower, "insufficient authentication scopes") {
		reasons = append([]string{"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}, reasons...)
	}
	if strings.Contains(lower, "has not been used in project") || strings.Contains(lower, "access not configured") {
		reasons = append([]string{"SERVICE_DISABLED"}, reasons...)
	}
	for _, reason := range reasons {
		if hint, ok := guidance[reason]; ok {
			return fmt.Errorf("Google %s returned HTTP %d [%s]: %s", service, status, reason, hint)
		}
	}
	hint := "Google Cloud의 API 사용 설정, 승인한 권한과 할당량을 확인하세요. Check API enablement, granted permissions and quota."
	if status == 401 {
		hint = "Google 인증이 만료되었거나 철회되었습니다. 계정 관리에서 Google을 다시 연결하세요. Reconnect Google in account management."
	}
	return fmt.Errorf("Google %s returned HTTP %d: %s", service, status, hint)
}

// OAuth token errors use a different envelope from service API errors. Preserve
// only recognized error codes and our guidance; descriptions may contain secrets.
func googleTokenError(status int, body io.Reader) error {
	var payload struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&payload)
	guidance := map[string]string{
		"invalid_grant":       "Google 인증을 갱신할 수 없습니다. 계정 관리에서 Google 연결 / 권한 업데이트를 다시 진행하세요. Reconnect Google in account management.",
		"invalid_client":      "Google OAuth 클라이언트 인증이 거부되었습니다. 계정 설정의 Desktop OAuth Client ID와 Client Secret이 같은 클라이언트의 값인지 확인하세요. Check the OAuth client ID and secret.",
		"unauthorized_client": "이 OAuth 클라이언트에서 요청한 인증 방식이 허용되지 않습니다. Desktop OAuth 클라이언트 설정을 확인하세요. Check the Desktop OAuth client configuration.",
		"invalid_scope":       "Google 요청 권한이 올바르지 않습니다. 사용할 서비스를 선택하고 Google 연결 / 권한 업데이트를 다시 진행하세요. Reconnect with the selected service permissions.",
	}
	if hint, known := guidance[payload.Error]; known {
		return fmt.Errorf("Google authorization failed (HTTP %d) [%s]: %s", status, payload.Error, hint)
	}
	return fmt.Errorf("Google authorization failed (HTTP %d); reconnect or check the OAuth client configuration", status)
}
