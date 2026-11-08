package response

type GetCaptchaResp struct {
	CaptchaID  string `json:"captchaID"`
	CaptchaImg string `json:"captchaImg"`
}

type LoginResp struct {
	AccessToken string `json:"accessToken"`
	UserID      uint64 `json:"userID"`
	Username    string `json:"username"`
	Role        string `json:"role"`
}

type RefreshTokenResp struct {
	AccessToken string `json:"accessToken"`
}

type ResetTokenResp struct {
	ResetToken string `json:"resetToken"`
}

type WSTicketResp struct {
	Ticket string `json:"ticket"`
}
