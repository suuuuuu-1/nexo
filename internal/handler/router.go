package handler

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/suuuuu/nexo/internal/config"
	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/service"
	"github.com/suuuuu/nexo/internal/storage"
)

// Router 保存 HTTP 层的业务依赖；各处理函数将具体用例交给对应的 Service。
type Router struct {
	config      config.Config
	userService *service.UserService
	content     *service.ContentService
	orders      *service.OrderService
	wallet      *service.WalletService
	entitlement *service.EntitlementService
	progress    *service.ProgressService
	storage     *storage.R2Storage
	readiness   ReadinessChecker
}

// NewRouter 创建 Gin 引擎，并注册公开、认证、管理员和运营人员路由。
func NewRouter(
	cfg config.Config,
	userService *service.UserService,
	contentService *service.ContentService,
	orderService *service.OrderService,
	walletService *service.WalletService,
	entitlementService *service.EntitlementService,
	progressService *service.ProgressService,
	objectStorage *storage.R2Storage,
	readiness ReadinessChecker,
) *gin.Engine {
	api := &Router{
		config:      cfg,
		userService: userService,
		content:     contentService,
		orders:      orderService,
		wallet:      walletService,
		entitlement: entitlementService,
		progress:    progressService,
		storage:     objectStorage,
		readiness:   readiness,
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.AllowedOrigin},
		AllowMethods:     []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	router.GET("/health", api.health)
	router.GET("/ready", api.ready)

	// 注册和登录接口无需登录。
	authRoutes := router.Group("/api/auth")
	authRoutes.POST("/register", api.register)
	authRoutes.POST("/login", api.login)

	// 内容和会员计划浏览接口公开；模拟支付回调仅用于 v1 演示。
	router.GET("/api/contents", api.listPublishedContents)
	router.GET("/api/contents/:id", api.getPublishedContent)
	router.GET("/api/episodes/:id", api.getPublishedEpisode)
	router.GET("/api/plans", api.listPlans)
	router.POST("/api/payments/mock/callback", api.mockPaymentCallback)

	// 以下接口均要求请求携带有效 JWT。
	protected := router.Group("/api")
	protected.Use(AuthRequired(userService))
	protected.GET("/me", api.me)
	protected.PATCH("/me", api.updateMe)
	protected.GET("/me/wallet", api.getWallet)
	protected.POST("/me/wallet/recharge", api.rechargeWallet)
	protected.GET("/me/progress", api.listProgress)
	protected.GET("/episodes/:id/play-url", api.getPlayURL)
	protected.POST("/episodes/:id/progress", api.updateProgress)
	protected.POST("/orders", api.createOrder)
	protected.GET("/orders", api.listOrders)
	protected.GET("/orders/:id", api.getOrder)
	protected.POST("/orders/:id/pay", api.payOrder)

	// 仅 ADMIN 可访问用户管理接口。
	admin := protected.Group("/admin")
	admin.Use(RequireRoles(model.RoleAdmin))
	admin.GET("/users", api.listUsers)
	admin.PATCH("/users/:id/role", api.updateUserRole)
	admin.PATCH("/users/:id/status", api.updateUserStatus)

	// OPERATOR 和 ADMIN 共用内容运营接口。
	operator := protected.Group("/operator")
	operator.Use(RequireRoles(model.RoleOperator, model.RoleAdmin))
	operator.GET("/contents", api.listOperatorContents)
	operator.GET("/contents/:id", api.getOperatorContent)
	operator.POST("/contents", api.createContent)
	operator.PATCH("/contents/:id", api.updateContent)
	operator.POST("/contents/:id/publish", api.publishContent)
	operator.POST("/contents/:id/offline", api.offlineContent)
	operator.POST("/contents/:id/uploads/presign", api.presignEpisodeUpload)
	operator.POST("/contents/:id/episodes", api.createEpisode)
	operator.PATCH("/episodes/:id", api.updateEpisode)
	operator.POST("/episodes/:id/publish", api.publishEpisode)
	operator.POST("/episodes/:id/offline", api.offlineEpisode)

	return router
}
