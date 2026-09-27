export type Role = "USER" | "OPERATOR" | "ADMIN";
export type UserStatus = "ACTIVE" | "DISABLED";

// 前端类型与后端公开 JSON 模型保持一致，避免页面组件重复声明业务字段。
export type User = {
  id: string;
  email: string;
  nickname: string;
  avatar_url?: string | null;
  role: Role;
  status: UserStatus;
};

export type ContentStatus = "DRAFT" | "PUBLISHED" | "OFFLINE";
export type AccessType = "FREE" | "PURCHASE" | "MEMBERSHIP" | "PURCHASE_OR_MEMBERSHIP";

export type Episode = {
  id: string;
  content_id: string;
  content_title?: string;
  episode_no: number;
  title: string;
  summary: string;
  access_type: AccessType;
  price_cents: number;
  resource_type: string;
  object_key?: string | null;
  status: ContentStatus;
};

export type Content = {
  id: string;
  title: string;
  content_type: string;
  description: string;
  cover_object_key?: string | null;
  price_cents: number;
  status: ContentStatus;
  episodes?: Episode[];
};

export type MembershipPlan = {
  id: string;
  name: string;
  description: string;
  price_cents: number;
  duration_days: number;
  status: string;
};

export type OrderItem = {
  item_type: "EPISODE" | "CONTENT" | "MEMBERSHIP";
  item_id: string;
  item_name: string;
  unit_price: number;
  quantity: number;
};

export type Order = {
  id: string;
  order_no: string;
  status: string;
  payment_method: "WALLET" | "MOCK";
  total_amount: number;
  currency: string;
  paid_at?: string | null;
  item: OrderItem;
};

export type Wallet = {
  user_id: string;
  balance: number;
  version: number;
};

export type WatchProgress = {
  episode_id: string;
  position_seconds: number;
  duration_seconds: number;
  version: number;
};

type AuthResponse = {
  token: string;
  user: User;
};

// Token 暂存于浏览器本地，用于 v1 的前后端联调；生产环境可再评估 HttpOnly Cookie 方案。
const TOKEN_KEY = "nexo_access_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function saveToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
	// 所有请求统一注入 Token、解析 JSON 和转换错误，业务页面只关注成功结果。
	const token = getToken();
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(path, { ...init, headers });
  const body = (await response.json().catch(() => ({}))) as { error?: string } & T;
  if (!response.ok) {
    throw new Error(body.error || "请求失败");
  }
  return body as T;
}

export async function register(input: { email: string; password: string; nickname: string }) {
  const result = await request<AuthResponse>("/api/auth/register", {
    method: "POST",
    body: JSON.stringify(input),
  });
  saveToken(result.token);
  return result.user;
}

// 登录和注册成功后统一保存服务端签发的访问 Token。
export async function login(input: { email: string; password: string }) {
  const result = await request<AuthResponse>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify(input),
  });
  saveToken(result.token);
  return result.user;
}

export function getMe() {
  return request<User>("/api/me");
}

export function updateMe(input: { nickname: string; avatar_url?: string }) {
  return request<User>("/api/me", {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function listContents() {
  return request<{ items: Content[] }>("/api/contents");
}

export function getContent(id: string) {
  return request<Content>(`/api/contents/${id}`);
}

export function listPlans() {
  return request<{ items: MembershipPlan[] }>("/api/plans");
}

export function getWallet() {
  return request<Wallet>("/api/me/wallet");
}

export function rechargeWallet(amount: number, key: string) {
  return request<Wallet>("/api/me/wallet/recharge", {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify({ amount }),
  });
}

// 创建订单时由调用方传入幂等键，页面重试不会重复创建订单。
export function createOrder(input: { item_type: string; item_id: string; payment_method: "WALLET" | "MOCK" }, key: string) {
  return request<Order>("/api/orders", {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify(input),
  });
}

export function listOrders() {
  return request<{ items: Order[] }>("/api/orders");
}

export function payOrder(id: string) {
  return request<Order>(`/api/orders/${id}/pay`, { method: "POST" });
}

export function mockPayment(orderNo: string) {
  return request<Order>("/api/payments/mock/callback", {
    method: "POST",
    body: JSON.stringify({ order_no: orderNo, provider_txn_id: `mock-${Date.now()}` }),
  });
}

// 播放地址只在服务端完成权益校验后生成，前端不自行判断购买状态。
export function getPlayURL(episodeID: string) {
  return request<{ play_url: string; expires_at: string }>(`/api/episodes/${episodeID}/play-url`);
}

export function listProgress() {
  return request<{ items: WatchProgress[] }>("/api/me/progress");
}

export function updateProgress(episodeID: string, input: { position_seconds: number; duration_seconds: number; expected_version?: number }) {
  return request<WatchProgress>(`/api/episodes/${episodeID}/progress`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// 运营端接口只负责调用后端，不在浏览器端复制发布和权限规则。
export function listOperatorContents() {
  return request<{ items: Content[] }>("/api/operator/contents");
}

export function createOperatorContent(input: { title: string; description: string; price_cents: number }) {
  return request<Content>("/api/operator/contents", {
    method: "POST",
    body: JSON.stringify({ ...input, content_type: "VIDEO" }),
  });
}

export function publishOperatorContent(id: string) {
  return request<Content>(`/api/operator/contents/${id}/publish`, { method: "POST" });
}

export function createOperatorEpisode(contentID: string, input: { episode_no: number; title: string; summary: string; access_type: AccessType; price_cents: number; object_key: string }) {
  return request<Episode>(`/api/operator/contents/${contentID}/episodes`, {
    method: "POST",
    body: JSON.stringify({ ...input, resource_type: "VIDEO" }),
  });
}

type EpisodeUploadTicket = {
  upload_url: string;
  object_key: string;
  expires_at: string;
  content_type: string;
};

// 上传票据只包含短期地址；长期保存到业务数据中的仍然只有 object_key。
export function createEpisodeUploadTicket(contentID: string, file: File) {
  return request<EpisodeUploadTicket>(`/api/operator/contents/${contentID}/uploads/presign`, {
    method: "POST",
    body: JSON.stringify({ filename: file.name, content_type: file.type || "video/mp4", size_bytes: file.size }),
  });
}

// 视频内容直传对象存储，避免大文件经过 Go API；用 XHR 提供浏览器上传进度。
export function uploadEpisodeVideo(uploadURL: string, file: File, onProgress: (percent: number) => void) {
  return new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", uploadURL);
    xhr.setRequestHeader("Content-Type", file.type || "video/mp4");
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(`R2 上传失败（HTTP ${xhr.status}）`));
    };
    xhr.onerror = () => reject(new Error("无法连接 R2；请检查桶的 CORS 是否允许当前网站来源执行 PUT"));
    xhr.onabort = () => reject(new Error("上传已取消"));
    xhr.send(file);
  });
}

export function publishOperatorEpisode(id: string) {
  return request<Episode>(`/api/operator/episodes/${id}/publish`, { method: "POST" });
}
