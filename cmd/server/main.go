package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/cache"
	"github.com/suuuuu/nexo/internal/config"
	"github.com/suuuuu/nexo/internal/database"
	"github.com/suuuuu/nexo/internal/handler"
	"github.com/suuuuu/nexo/internal/health"
	"github.com/suuuuu/nexo/internal/mq"
	"github.com/suuuuu/nexo/internal/repository"
	"github.com/suuuuu/nexo/internal/service"
	"github.com/suuuuu/nexo/internal/storage"
	"github.com/suuuuu/nexo/internal/worker"
)

func main() {
	// main 是应用程序的组合根（Composition Root）。
	// 这里不承载具体业务逻辑，只负责按照依赖顺序完成：
	// 配置加载 → 基础设施初始化 → 数据访问层组装 → 后台 Worker 启动 → HTTP 服务启动。
	cfg := config.Load()

	// 启动阶段的依赖连接和数据库迁移不能无限等待。
	// 这个 context 只用于启动过程，不与服务运行期间的请求 context 混用。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 数据库是核心依赖，Repository、迁移和大部分业务 Service 都建立在它之上。
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	// 先执行迁移，再创建 Repository，确保应用启动后使用的是最新表结构。
	if err := database.Migrate(ctx, db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	// Redis 用于缓存热点内容；当前缓存失效时业务仍可回源数据库。
	redisClient, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	// RabbitMQ 用于承载订单支付后的异步履约事件，避免支付请求同步完成所有权益发放工作。
	broker, err := mq.Connect(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("connect rabbitmq: %v", err)
	}
	defer broker.Close()

	// 通过接口屏蔽对象存储实现：开发环境使用 Mock Storage，配置 R2 后切换到 Cloudflare R2。
	// 数据库只保存对象 key，访问时再由 Storage 实现生成临时地址。
	objectStorage, err := storage.New(cfg.StorageProvider, cfg.R2Endpoint, cfg.R2AccessKey, cfg.R2SecretKey)
	if err != nil {
		log.Fatalf("configure object storage: %v", err)
	}

	// 集中创建 Repository 和 Service；底层连接仍由 main 初始化并传入。
	deps := buildDependencies(db, redisClient, cfg)

	// 监听进程退出信号，并把同一个运行 context 传给后台任务。
	// 收到 SIGINT/SIGTERM 后，Outbox Publisher 和权益 Worker 会停止继续领取新任务。
	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Outbox Publisher 负责把数据库中的待发送事件投递到 RabbitMQ。
	// Entitlement Worker 消费订单支付事件，并幂等地发放用户权益。
	worker.StartOutboxPublisher(runCtx, deps.outboxRepo, broker)
	if err := worker.StartEntitlementWorker(runCtx, broker, deps.entitlementService); err != nil {
		log.Fatalf("start entitlement worker: %v", err)
	}

	// Router 统一组装 HTTP Handler、Middleware 和各业务依赖。
	// Run 会阻塞当前 goroutine，直到 HTTP 服务退出或返回启动错误。
	readiness := health.NewChecker(map[string]health.Probe{
		"postgres": db.Ping,
		"redis":    redisClient.Ping,
		"rabbitmq": broker.Check,
	})
	router := handler.NewRouter(cfg, deps.userService, deps.contentService, deps.orderService, deps.walletService, deps.entitlementService, deps.progressService, objectStorage, readiness)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run http server: %v", err)
	}
}

func init() {
	// 生产环境关闭 Gin 的调试输出，避免无关日志干扰服务日志。
	gin.SetMode(gin.ReleaseMode)
}

// serverDependencies 收纳 HTTP Handler 和后台 Worker 需要的业务依赖。
// 它只用于启动时组装，不承担业务逻辑，也不创建数据库或 Redis 连接。
type serverDependencies struct {
	userService        *service.UserService
	contentService     *service.ContentService
	orderService       *service.OrderService
	walletService      *service.WalletService
	entitlementService *service.EntitlementService
	progressService    *service.ProgressService
	outboxRepo         *repository.OutboxRepository
}

// buildDependencies 使用已经建立的基础设施客户端创建 Repository 和 Service。
func buildDependencies(db *pgxpool.Pool, redisClient *cache.Client, cfg config.Config) serverDependencies {
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, cfg.JWTSecret, cfg.JWTExpires)

	contentRepo := repository.NewContentRepository(db)
	contentService := service.NewContentService(contentRepo, redisClient)

	transactionManager := repository.NewTransactionManager(db)
	orderRepo := repository.NewOrderRepository(db)
	orderService := service.NewOrderService(orderRepo, transactionManager)
	walletRepo := repository.NewWalletRepository(db)
	walletService := service.NewWalletService(walletRepo, transactionManager)
	entitlementRepo := repository.NewEntitlementRepository(db)
	entitlementService := service.NewEntitlementService(entitlementRepo, transactionManager)

	progressRepo := repository.NewProgressRepository(db)
	progressService := service.NewProgressService(progressRepo)
	outboxRepo := repository.NewOutboxRepository(db)

	return serverDependencies{
		userService:        userService,
		contentService:     contentService,
		orderService:       orderService,
		walletService:      walletService,
		entitlementService: entitlementService,
		progressService:    progressService,
		outboxRepo:         outboxRepo,
	}
}
