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
	TraceID string `json:"traceID,omitempty"`
}

// jsonResponse 统一响应 JSON
func jsonResponse(c *gin.Context, httpStatus int, code int, msg string, data any) {
	traceID := logger.GetTraceID(c.Request.Context())
	c.JSON(httpStatus, Response{
		Code:    code,
		Msg:     msg,
		Data:    data,
		TraceID: traceID,
	})
}

// Success 成功
func Success(c *gin.Context, data any) {
	jsonResponse(c, http.StatusOK, http.StatusOK, "成功", data)
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
	FailWithMsg(c, 500, "服务器内部错误")
}

// FailWithMsg 自定义失败状态码和错误消息
func FailWithMsg(c *gin.Context, code int, msg string) {
	jsonResponse(c, code, code, msg, nil)
}
