package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/service"
)

type registerRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type updateProfileRequest struct {
	Nickname  string  `json:"nickname" binding:"required"`
	AvatarURL *string `json:"avatar_url"`
}

type roleRequest struct {
	Role model.Role `json:"role" binding:"required"`
}

type statusRequest struct {
	Status model.UserStatus `json:"status" binding:"required"`
}

// register 处理公开注册请求，注册用户固定为 USER 角色。
func (r *Router) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	result, err := r.userService.Register(c.Request.Context(), service.RegisterInput{Email: req.Email, Password: req.Password, Nickname: req.Nickname})
	if err != nil {
		r.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

// login 处理用户登录并返回 JWT 与公开用户信息。
func (r *Router) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	result, err := r.userService.Login(c.Request.Context(), service.LoginInput{Email: req.Email, Password: req.Password})
	if err != nil {
		r.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// me 返回当前登录用户的最新资料，而不是直接信任 Token 中的旧信息。
func (r *Router) me(c *gin.Context) {
	id, ok := CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	current, err := r.userService.GetByID(c.Request.Context(), id)
	if err != nil {
		r.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, current.Public())
}

// updateMe 更新当前用户的昵称和头像。
func (r *Router) updateMe(c *gin.Context) {
	id, ok := CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	updated, err := r.userService.UpdateProfile(c.Request.Context(), id, service.UpdateProfileInput{Nickname: req.Nickname, AvatarURL: req.AvatarURL})
	if err != nil {
		r.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated.Public())
}

// listUsers 提供管理员分页查看用户的能力。
func (r *Router) listUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	users, err := r.userService.List(c.Request.Context(), limit, offset)
	if err != nil {
		r.writeServiceError(c, err)
		return
	}
	result := make([]model.PublicUser, 0, len(users))
	for index := range users {
		result = append(result, users[index].Public())
	}
	c.JSON(http.StatusOK, gin.H{"items": result, "limit": limit, "offset": offset})
}

// updateUserRole 修改用户角色。
func (r *Router) updateUserRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	updated, err := r.userService.UpdateRole(c.Request.Context(), c.Param("id"), req.Role)
	if err != nil {
		r.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated.Public())
}

// updateUserStatus 启用或禁用用户。
func (r *Router) updateUserStatus(c *gin.Context) {
	var req statusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	updated, err := r.userService.UpdateStatus(c.Request.Context(), c.Param("id"), req.Status)
	if err != nil {
		r.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated.Public())
}
