package common

import (
	logger "smdp-gateway/logger"
	"smdp-gateway/utils"
	"time"

	"github.com/kataras/iris/v12"
)

type commonResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ReturnOK 返回正常
func ReturnOK(ctx iris.Context, data any) {
	ErrCode := ErrCodeOK
	commonRsp := &commonResponse{
		Code:    ErrCode,
		Message: CodeMessage[ErrCode],
		Data:    data,
	}
	ctx.StatusCode(iris.StatusOK)
	ctx.JSON(commonRsp)
}

// ReturnInternalError 返回内部错误
func ReturnInternalError(ctx iris.Context) {
	ErrCode := ErrCodeInternalError
	commonRsp := &commonResponse{
		Code:    ErrCode,
		Message: CodeMessage[ErrCode],
	}
	ctx.StatusCode(iris.StatusInternalServerError)
	ctx.JSON(commonRsp)
}

// ReturnParseBodyError 返回解析body失败的错误
func ReturnParseBodyError(ctx iris.Context) {
	ErrCode := ErrCodeParseBodyError
	commonRsp := &commonResponse{
		Code:    ErrCode,
		Message: CodeMessage[ErrCode],
	}
	ctx.StatusCode(iris.StatusBadRequest)
	ctx.JSON(commonRsp)
}

// ReturnBadRequest 返回请求错误的错误
func ReturnBadRequest(ctx iris.Context, ErrCode int) {
	commonRsp := &commonResponse{
		Code:    ErrCode,
		Message: CodeMessage[ErrCode],
	}
	ctx.StatusCode(iris.StatusBadRequest)
	ctx.JSON(commonRsp)
}

// ReturnBadRequestWithMessage 返回请求错误的错误，携带错误信息
func ReturnBadRequestWithMessage(ctx iris.Context, ErrCode int, errMsg string) {
	commonRsp := &commonResponse{
		Code:    ErrCode,
		Message: CodeMessage[ErrCode],
		Data:    errMsg,
	}
	ctx.StatusCode(iris.StatusBadRequest)
	ctx.JSON(commonRsp)
}

// Preprocessing 预处理，设置traceid和本地时间
func Preprocessing(ctx iris.Context) {
	traceID := utils.GenerateUniqueID()
	ctx.Values().Set(logger.TraceIDKey, traceID)
	ctx.Header(logger.TraceIDKey, traceID)
	/*
		HTTP 协议规范要求 Date 头使用 RFC 1123 格式和 UTC 时区
		修改标准头可能会影响某些客户端的兼容性
		最佳实践是保持 Date 头为 UTC，在业务数据中使用正确时区
		所以，自定义一个头部，标记本地时间
	*/
	loc, _ := time.LoadLocation("Asia/Shanghai")
	ctx.Header("X-Local-Date", time.Now().In(loc).Format(time.RFC1123))
	ctx.Next()
}

// AdminAuth 管理员权限校验
func AdminAuth(ctx iris.Context) {
	// if !config.Global().Server.SkipAuth {
	// 	token := ctx.GetHeader("Authorization")

	// 	tokenLock.RLock()
	// 	defer tokenLock.RUnlock()

	// 	info, exist := tokenMap[token]
	// 	if !exist || info.ClientIP != ctx.RemoteAddr() || info.Expired.Before(time.Now()) {
	// 		errCode := ErrCodeInvalidToken
	// 		ReturnBadRequest(ctx, errCode)
	// 		logger.Default(ctx).Error(CodeMessage[errCode])
	// 		return
	// 	}
	// }
	ctx.Next()
}
