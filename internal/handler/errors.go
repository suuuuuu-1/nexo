package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/suuuuu/nexo/internal/service"
)

// writeServiceError 将用户 Service 错误转换为稳定的 HTTP 状态码和响应格式。
func (r *Router) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidUserInput), errors.Is(err, service.ErrInvalidRole), errors.Is(err, service.ErrInvalidUserStatus):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrUserEmailExists):
		c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
	case errors.Is(err, service.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrUserDisabled):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
	default:
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

// queryInt 读取并校验分页查询参数，非法值回退到默认值。
func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.DefaultQuery(key, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return value
}

// writeBusinessError 将内容、订单、钱包和进度错误统一映射为 HTTP 响应。
func (r *Router) writeBusinessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidContentInput), errors.Is(err, service.ErrInvalidOrderInput), errors.Is(err, service.ErrProgressVersionConflict):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrContentConflict), errors.Is(err, service.ErrOrderConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrContentNotFound), errors.Is(err, service.ErrOrderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
	case errors.Is(err, service.ErrInsufficientFunds):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidOrderState), errors.Is(err, service.ErrAlreadyOwned):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidWalletAmount):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
