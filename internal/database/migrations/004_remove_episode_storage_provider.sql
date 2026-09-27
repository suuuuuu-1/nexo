-- v1 只使用 Cloudflare R2；存储提供方由部署配置决定，不在每个 Episode 上重复保存。
ALTER TABLE episodes DROP COLUMN IF EXISTS storage_provider;
