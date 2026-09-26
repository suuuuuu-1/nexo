package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/suuuuu/nexo/internal/model"
)

type orderRequest struct {
	ItemType      model.ItemType      `json:"item_type" binding:"required"`
	ItemID        string              `json:"item_id" binding:"required"`
	PaymentMethod model.PaymentMethod `json:"payment_method"`
}

type mockPaymentRequest struct {
	OrderNo       string `json:"order_no" binding:"required"`
	ProviderTxnID string `json:"provider_txn_id" binding:"required"`
}

// listPlans 返回可购买的会员计划。
func (r *Router) listPlans(c *gin.Context) {
	items, err := r.orders.ListPlans(c.Request.Context())
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// createOrder 创建单集、整部内容或会员计划订单。
func (r *Router) createOrder(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		key = uuid.NewString()
	}
	item, err := r.orders.CreateOrder(c.Request.Context(), userID, model.OrderCreateInput{ItemType: req.ItemType, ItemID: req.ItemID, PaymentMethod: req.PaymentMethod, IdempotencyKey: key})
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

// listOrders 查询当前用户自己的订单。
func (r *Router) listOrders(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	items, err := r.orders.ListOrders(c.Request.Context(), userID, queryInt(c, "limit", 20), queryInt(c, "offset", 0))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// getOrder 查询当前用户自己的单笔订单。
func (r *Router) getOrder(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	item, err := r.orders.GetOrder(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// payOrder 使用钱包余额支付订单。
func (r *Router) payOrder(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	item, err := r.orders.PayWithWallet(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// mockPaymentCallback 模拟支付平台异步回调，验证回调幂等处理。
func (r *Router) mockPaymentCallback(c *gin.Context) {
	var req mockPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := r.orders.MockCallback(c.Request.Context(), req.OrderNo, req.ProviderTxnID)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
