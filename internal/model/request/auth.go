package request

type SendSMSCodeReq struct {
	Phone       string `json:"phone" binding:"required,cnphone"`
	CaptchaID   string `json:"captchaID" binding:"required"`
	CaptchaCode string `json:"captchaCode" binding:"required"`
}

type RegisterReq struct {
	Username string `json:"username" binding:"required,min=2,max=64,alphanumunicode"`
	Password string `json:"password" binding:"required,min=8,max=72,printascii"`
	Phone    string `json:"phone" binding:"required,cnphone"`
	SMSCode  string `json:"smsCode" binding:"required,len=6,number"`
}

type LoginReq struct {
	Phone       string `json:"phone" binding:"required,cnphone"`
	Password    string `json:"password" binding:"required,min=8,max=72,printascii"`
	CaptchaID   string `json:"captchaID" binding:"required"`
	CaptchaCode string `json:"captchaCode" binding:"required"`
}

type LoginBySMSReq struct {
	Phone   string `json:"phone" binding:"required,cnphone"`
	SMSCode string `json:"smsCode" binding:"required,len=6,number"`
}

type VerifyResetCodeReq struct {
	Phone   string `json:"phone" binding:"required,cnphone"`
	SMSCode string `json:"smsCode" binding:"required,len=6,number"`
}

type ResetPasswordReq struct {
	ResetToken  string `json:"resetToken" binding:"required,uuid4"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=72,printascii"`
}
