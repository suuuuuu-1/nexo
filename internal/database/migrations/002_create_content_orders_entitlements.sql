-- 内容域：Content 是完整内容，Episode 是可访问的具体章节/集。
CREATE TABLE IF NOT EXISTS contents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(200) NOT NULL,
    content_type VARCHAR(32) NOT NULL DEFAULT 'VIDEO'
        CHECK (content_type IN ('VIDEO', 'NOVEL')),
    description TEXT NOT NULL DEFAULT '',
    cover_object_key TEXT,
    price_cents BIGINT NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    status VARCHAR(16) NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'PUBLISHED', 'OFFLINE')),
    created_by UUID NOT NULL REFERENCES users(id),
    updated_by UUID NOT NULL REFERENCES users(id),
    published_by UUID REFERENCES users(id),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_contents_status_created_at ON contents(status, created_at DESC);

CREATE TABLE IF NOT EXISTS episodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    episode_no INT NOT NULL CHECK (episode_no > 0),
    title VARCHAR(200) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    access_type VARCHAR(32) NOT NULL DEFAULT 'FREE'
        CHECK (access_type IN ('FREE', 'PURCHASE', 'MEMBERSHIP', 'PURCHASE_OR_MEMBERSHIP')),
    price_cents BIGINT NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    resource_type VARCHAR(32) NOT NULL DEFAULT 'VIDEO'
        CHECK (resource_type IN ('VIDEO', 'TEXT')),
    storage_provider VARCHAR(32) NOT NULL DEFAULT 'R2',
    bucket_name VARCHAR(128),
    object_key TEXT,
    status VARCHAR(16) NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'PUBLISHED', 'OFFLINE')),
    created_by UUID NOT NULL REFERENCES users(id),
    updated_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (content_id, episode_no)
);

CREATE INDEX IF NOT EXISTS idx_episodes_content_status ON episodes(content_id, status, episode_no);

-- 会员域：计划定义价格和期限，订阅记录用户实际获得的有效期。
CREATE TABLE IF NOT EXISTS membership_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    price_cents BIGINT NOT NULL CHECK (price_cents > 0),
    duration_days INT NOT NULL CHECK (duration_days > 0),
    status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'INACTIVE')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO membership_plans (name, description, price_cents, duration_days)
VALUES ('基础月卡', '访问会员专区内容，有效期 30 天', 1990, 30)
ON CONFLICT (name) DO NOTHING;

-- 订单域：订单只记录购买事实和状态，具体访问权限由 Entitlement 表表达。
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_no VARCHAR(40) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(24) NOT NULL DEFAULT 'PENDING_PAYMENT'
        CHECK (status IN ('PENDING_PAYMENT', 'PAYMENT_FAILED', 'PAID', 'FULFILLING', 'COMPLETED', 'CANCELLED', 'EXPIRED')),
    payment_method VARCHAR(24) NOT NULL DEFAULT 'WALLET'
        CHECK (payment_method IN ('WALLET', 'MOCK')),
    provider_txn_id VARCHAR(128) UNIQUE,
    total_amount BIGINT NOT NULL CHECK (total_amount >= 0),
    currency VARCHAR(8) NOT NULL DEFAULT 'CREDITS',
    idempotency_key VARCHAR(128),
    expires_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_orders_user_created_at ON orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    item_type VARCHAR(24) NOT NULL
        CHECK (item_type IN ('EPISODE', 'CONTENT', 'MEMBERSHIP')),
    item_id UUID NOT NULL,
    item_name VARCHAR(200) NOT NULL,
    unit_price BIGINT NOT NULL CHECK (unit_price >= 0),
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity = 1),
    UNIQUE (order_id)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    plan_id UUID NOT NULL REFERENCES membership_plans(id),
    order_id UUID NOT NULL UNIQUE REFERENCES orders(id),
    status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'EXPIRED', 'CANCELLED')),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_active ON subscriptions(user_id, status, ends_at);

-- 权益域：统一表达单集、整部内容和会员访问权限。
CREATE TABLE IF NOT EXISTS entitlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    scope_type VARCHAR(24) NOT NULL
        CHECK (scope_type IN ('EPISODE', 'CONTENT', 'MEMBERSHIP')),
    scope_id UUID NOT NULL,
    source_type VARCHAR(24) NOT NULL
        CHECK (source_type IN ('ORDER', 'SUBSCRIPTION', 'ADMIN_GRANT')),
    source_id UUID NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'REVOKED')),
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, scope_type, scope_id, source_type, source_id)
);

CREATE INDEX IF NOT EXISTS idx_entitlements_access ON entitlements(user_id, scope_type, scope_id, status, ends_at);

-- 钱包域：balance 是当前余额，wallet_ledger 是可审计的变更流水。
CREATE TABLE IF NOT EXISTS wallets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    version BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallet_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    order_id UUID REFERENCES orders(id),
    transaction_type VARCHAR(24) NOT NULL
        CHECK (transaction_type IN ('RECHARGE', 'PURCHASE', 'REFUND')),
    amount BIGINT NOT NULL CHECK (amount <> 0),
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_ledger_order_purchase
    ON wallet_ledger(order_id)
    WHERE transaction_type = 'PURCHASE' AND order_id IS NOT NULL;

-- 观看进度域：每个用户对每个 Episode 保留一条最新进度，并通过 version 支持乐观锁。
CREATE TABLE IF NOT EXISTS watch_progress (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id UUID NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    position_seconds INT NOT NULL DEFAULT 0 CHECK (position_seconds >= 0),
    duration_seconds INT NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    version BIGINT NOT NULL DEFAULT 0,
    last_watched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, episode_id)
);

-- Outbox 域：支付状态和业务事件在同一事务写入，后台 Publisher 再异步投递到 MQ。
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(64) NOT NULL,
    aggregate_type VARCHAR(32) NOT NULL,
    aggregate_id UUID NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    attempts INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox_events(status, next_retry_at, created_at);
