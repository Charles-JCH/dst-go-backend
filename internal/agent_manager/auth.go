package agent_manager

import (
	"errors"
)

type TokenParser interface {
	ParseAgentToken(token string) (uint64, error)
}

type Auth struct {
	tokenParser TokenParser
}

func NewAuth(tokenParser TokenParser) *Auth {
	return &Auth{tokenParser: tokenParser}
}

func (a *Auth) Authenticate(token string) (uint64, error) {
	if token == "" {
		return 0, errors.New("agent token 为空")
	}
	return a.tokenParser.ParseAgentToken(token)
}
