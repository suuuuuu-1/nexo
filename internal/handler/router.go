package handler

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/suuuuu/nexo/internal/config"
	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/service"
	"github.com/suuuuu/nexo/internal/storage"
)

// Router holds the HTTP layer's business dependencies; handlers delegate use cases to services.
type Router struct {
	config      config.Config
	userService *service.UserService
	content     *service.ContentService
	orders      *service.OrderService
	wallet      *service.WalletService
	entitlement *service.EntitlementService
	progress    *service.ProgressService
	storage     storage.ObjectStorage
	readiness   ReadinessChecker
}

// NewRouter creates the Gin engine and registers public, authenticated, admin, and operator routes.
func NewRouter(
	cfg config.Config,
	userService *service.UserService,
	contentService *service.ContentService,
	orderService *service.OrderService,
	walletService *service.WalletService,
	entitlementService *service.EntitlementService,
	progressService *service.ProgressService,
	objectStorage storage.ObjectStorage,
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

	// Authentication endpoints are public.
	authRoutes := router.Group("/api/auth")
	authRoutes.POST("/register", api.register)
	authRoutes.POST("/login", api.login)

	// Public content and plans; the mock payment callback is a public v1 test endpoint.
	router.GET("/api/contents", api.listPublishedContents)
	router.GET("/api/contents/:id", api.getPublishedContent)
	router.GET("/api/episodes/:id", api.getPublishedEpisode)
	router.GET("/api/plans", api.listPlans)
	router.POST("/api/payments/mock/callback", api.mockPaymentCallback)

	// All following endpoints require a valid JWT.
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

	// ADMIN-only user management endpoints.
	admin := protected.Group("/admin")
	admin.Use(RequireRoles(model.RoleAdmin))
	admin.GET("/users", api.listUsers)
	admin.PATCH("/users/:id/role", api.updateUserRole)
	admin.PATCH("/users/:id/status", api.updateUserStatus)

	// OPERATOR and ADMIN share content management endpoints.
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
