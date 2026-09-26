package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type rechargeRequest struct {
	Amount int64 `json:"amount" binding:"required"`
}

// getWallet 查询当前用户钱包余额。
func (r *Router) getWallet(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	item, err := r.wallet.Get(c.Request.Context(), userID)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// rechargeWallet 处理虚拟币充值，并透传 Idempotency-Key。
func (r *Router) rechargeWallet(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	var req rechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	key := c.GetHeader("Idempotency-Key")
	item, err := r.wallet.Recharge(c.Request.Context(), userID, req.Amount, key)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
