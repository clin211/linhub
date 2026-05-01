package jwt

import (
	"encoding/json"
)

// tokenInfo 包含 token 的信息。
type tokenInfo struct {
	// Token 字符串。
	Token string `json:"token"`

	// Token 类型。
	Type string `json:"type"`

	// Token 过期时间
	ExpiresAt int64 `json:"expiresAt"`
}

func (t *tokenInfo) GetToken() string {
	return t.Token
}

func (t *tokenInfo) GetTokenType() string {
	return t.Type
}

func (t *tokenInfo) GetExpiresAt() int64 {
	return t.ExpiresAt
}

func (t *tokenInfo) EncodeToJSON() ([]byte, error) {
	return json.Marshal(t)
}
