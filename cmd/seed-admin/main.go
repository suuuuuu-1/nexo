package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/suuuuu/nexo/internal/config"
	"github.com/suuuuu/nexo/internal/database"
	"github.com/suuuuu/nexo/internal/repository"
	"github.com/suuuuu/nexo/internal/service"
)

func main() {
	// 管理员账号通过环境变量传入，避免把初始密码写进代码或提交到仓库。
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	adminNickname := os.Getenv("ADMIN_NICKNAME")
	if adminNickname == "" {
		adminNickname = "Nexo Admin"
	}
	if adminEmail == "" || adminPassword == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD are required")
	}

	cfg := config.Load()
	// 初始化命令也复用服务端的配置、数据库连接和迁移逻辑，避免两套数据库行为。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	// BootstrapAdmin 会以 ADMIN 角色创建账号，并复用用户服务的密码校验规则。
	created, err := service.NewUserService(repository.NewUserRepository(db), cfg.JWTSecret, cfg.JWTExpires).BootstrapAdmin(ctx, adminEmail, adminPassword, adminNickname)
	if err != nil {
		log.Fatalf("create admin: %v", err)
	}
	fmt.Printf("created admin %s (%s)\n", created.Email, created.ID)
}
