-- 用户基础表：保存登录凭证、角色和账号状态。
-- password_hash 只保存 bcrypt 哈希，不保存明文密码。
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    nickname VARCHAR(64) NOT NULL,
    avatar_url TEXT,
    role VARCHAR(16) NOT NULL DEFAULT 'USER'
        CHECK (role IN ('USER', 'OPERATOR', 'ADMIN')),
    status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'DISABLED')),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
