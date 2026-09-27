package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Config 集中保存服务启动所需的基础配置，业务包不直接读取环境变量。
	HTTPAddr      string
	DatabaseURL   string
	JWTSecret     string
	JWTExpires    time.Duration
	AllowedOrigin string
	RedisURL      string
	RabbitMQURL   string
	R2Endpoint    string
	R2AccessKey   string
	R2SecretKey   string
	R2Bucket      string
}

// Load 从环境变量加载配置；普通本地设置提供默认值，R2 配置刻意不提供模拟默认值。
// 部署环境必须显式注入 R2 Endpoint 和密钥，避免服务意外以假资源启动。
func Load() Config {
	expiresHours := 2
	if value := os.Getenv("JWT_EXPIRES_HOURS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			expiresHours = parsed
		}
	}

	return Config{
		HTTPAddr:      getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://nexo:nexo@localhost:5432/nexo?sslmode=disable"),
		JWTSecret:     getEnv("JWT_SECRET", "change-this-secret-in-development"),
		JWTExpires:    time.Duration(expiresHours) * time.Hour,
		AllowedOrigin: getEnv("ALLOWED_ORIGIN", "http://localhost:5173"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379/0"),
		RabbitMQURL:   getEnv("RABBITMQ_URL", "amqp://nexo:nexo@localhost:5672/"),
		R2Endpoint:    getEnv("R2_ENDPOINT", ""),
		R2AccessKey:   getEnv("R2_ACCESS_KEY", ""),
		R2SecretKey:   getEnv("R2_SECRET_KEY", ""),
		R2Bucket:      getEnv("R2_BUCKET", "nexo-media"),
	}
}

// getEnv 统一处理环境变量读取，避免各配置项重复编写默认值逻辑。
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
