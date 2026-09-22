package request

type SendSMSCodeReq struct {
	Phone       string `json:"phone" binding:"required"`
	CaptchaId   string `json:"captchaId" binding:"required"`
	CaptchaCode string `json:"captchaCode" binding:"required"`
}

type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,ascii,max=72"`
	Phone    string `json:"phone" binding:"required"`
	SMSCode  string `json:"smsCode" binding:"required"`
}

type LoginReq struct {
	Phone       string `json:"phone" binding:"required"`
	Password    string `json:"password" binding:"required,ascii,max=72"`
	CaptchaId   string `json:"captchaId" binding:"required"`
	CaptchaCode string `json:"captchaCode" binding:"required"`
}

type LoginBySMSReq struct {
	Phone   string `json:"phone" binding:"required"`
	SMSCode string `json:"smsCode" binding:"required"`
}

type VerifyResetCodeReq struct {
	Phone   string `json:"phone" binding:"required"`
	SMSCode string `json:"smsCode" binding:"required"`
}

type ResetPasswordReq struct {
	ResetToken  string `json:"resetToken" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,ascii,max=72"`
}
