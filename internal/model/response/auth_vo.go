package response

type Identity struct {
	UserId   uint64
	Username string
	Role     string
}

type GetCaptchaResp struct {
	CaptchaId  string `json:"captchaId"`
	CaptchaImg string `json:"captchaImg"`
}

type LoginResp struct {
	AccessToken string `json:"accessToken"`
	UserId      uint64 `json:"userId"`
	Username    string `json:"username"`
	Role        string `json:"role"`
}
