package result

import (
	"errors"
	"game-panel/internal/apperr"
	"game-panel/internal/platform/logger"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    any    `json:"data"`
	TraceId string `json:"traceId,omitempty"`
}

// jsonResponse 统一响应 JSON
func jsonResponse(c *gin.Context, httpStatus int, code int, msg string, data any) {
	traceId := logger.GetTraceId(c.Request.Context())
	c.JSON(httpStatus, Response{
		Code:    code,
		Msg:     msg,
		Data:    data,
		TraceId: traceId,
	})
}

// Success 成功
func Success(c *gin.Context, data any) {
	jsonResponse(c, http.StatusOK, http.StatusOK, "Success", data)
}

// SuccessWithMsg 自定义成功信息
func SuccessWithMsg(c *gin.Context, msg string, data any) {
	jsonResponse(c, http.StatusOK, http.StatusOK, msg, data)
}

// Fail 失败
func Fail(c *gin.Context, err error) {
	var bizErr *apperr.BizError
	if errors.As(err, &bizErr) {
		FailWithMsg(c, bizErr.Code, bizErr.Message)
		return
	}
	FailWithMsg(c, 500, "系统繁忙，请稍后再试")
}

// FailWithMsg 自定义失败状态码和错误消息
func FailWithMsg(c *gin.Context, code int, msg string) {
	jsonResponse(c, code, code, msg, nil)
}
