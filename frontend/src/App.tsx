import { FormEvent, ReactNode, useEffect, useState } from "react";
import {
  AccessType, clearToken, Content, createOperatorContent, createOperatorEpisode, createOrder,
  createEpisodeUploadTicket, getContent, getMe, getPlayURL, getToken, getWallet, listContents,
  listOperatorContents, listOrders, listPlans, login, MembershipPlan, mockPayment, Order, payOrder,
  publishOperatorContent, publishOperatorEpisode, rechargeWallet, register, Role, updateMe, uploadEpisodeVideo, User, Wallet,
} from "./api";

type View = "home" | "plans" | "orders" | "wallet" | "profile" | "operator";
const money = (cents: number) => `¥${(cents / 100).toFixed(2)}`;

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [view, setView] = useState<View>("home");
  const [selectedContentID, setSelectedContentID] = useState<string | null>(null);
  const [loading, setLoading] = useState(Boolean(getToken()));
  useEffect(() => { if (!getToken()) return; getMe().then(setUser).catch(() => clearToken()).finally(() => setLoading(false)); }, []);
  if (loading) return <main className="center-page">正在加载用户信息…</main>;
  if (!user) return <AuthScreen onSuccess={setUser} />;
  function openContent(id: string) { setSelectedContentID(id); }
  function logout() { clearToken(); setUser(null); setView("home"); }
  return <div className="app-shell">
    <header className="topbar">
      <button className="brand brand-button" onClick={() => { setView("home"); setSelectedContentID(null); }}>Nexo</button>
      <nav className="main-nav">
        <button onClick={() => { setView("home"); setSelectedContentID(null); }}>内容</button><button onClick={() => setView("plans")}>会员</button><button onClick={() => setView("orders")}>订单</button><button onClick={() => setView("wallet")}>钱包</button>
        {(user.role === "OPERATOR" || user.role === "ADMIN") && <button onClick={() => setView("operator")}>运营台</button>}
      </nav>
      <div className="topbar-actions"><span>{user.nickname}</span><button className="link-button" onClick={() => setView("profile")}>账户</button><button className="link-button" onClick={logout}>退出</button></div>
    </header>
    <main className="page-container">{selectedContentID ? <ContentDetail id={selectedContentID} onBack={() => setSelectedContentID(null)} /> : view === "home" ? <Home onOpen={openContent} /> : view === "plans" ? <Plans /> : view === "orders" ? <Orders /> : view === "wallet" ? <WalletView /> : view === "operator" ? <Operator /> : <Profile user={user} onUpdated={setUser} />}</main>
  </div>;
}

function AuthScreen({ onSuccess }: { onSuccess: (user: User) => void }) { const [page, setPage] = useState<"login" | "register">("login"); return page === "login" ? <Login onSuccess={onSuccess} onRegister={() => setPage("register")} /> : <Register onSuccess={onSuccess} onLogin={() => setPage("login")} />; }

function Login({ onSuccess, onRegister }: { onSuccess: (user: User) => void; onRegister: () => void }) {
  const [email, setEmail] = useState(""); const [password, setPassword] = useState(""); const [error, setError] = useState(""); const [submitting, setSubmitting] = useState(false);
  async function submit(event: FormEvent) { event.preventDefault(); setError(""); setSubmitting(true); try { onSuccess(await login({ email, password })); } catch (err) { setError(messageOf(err)); } finally { setSubmitting(false); } }
  return <AuthCard title="登录 Nexo" subtitle="进入你的数字内容账户"><form onSubmit={submit} className="form-stack"><label>邮箱<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required /></label><label>密码<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required /></label>{error && <p className="error-text">{error}</p>}<button className="primary-button" disabled={submitting}>{submitting ? "登录中…" : "登录"}</button></form><p className="switch-text">还没有账号？<button className="link-button" onClick={onRegister}>注册</button></p></AuthCard>;
}

function Register({ onSuccess, onLogin }: { onSuccess: (user: User) => void; onLogin: () => void }) {
  const [email, setEmail] = useState(""); const [nickname, setNickname] = useState(""); const [password, setPassword] = useState(""); const [error, setError] = useState(""); const [submitting, setSubmitting] = useState(false);
  async function submit(event: FormEvent) { event.preventDefault(); setError(""); if (password.length < 8) { setError("密码至少需要 8 位"); return; } setSubmitting(true); try { onSuccess(await register({ email, nickname, password })); } catch (err) { setError(messageOf(err)); } finally { setSubmitting(false); } }
  return <AuthCard title="创建 Nexo 账户" subtitle="注册后即可体验内容平台"><form onSubmit={submit} className="form-stack"><label>邮箱<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required /></label><label>昵称<input value={nickname} onChange={(e) => setNickname(e.target.value)} placeholder="可选" /></label><label>密码<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} minLength={8} required /></label>{error && <p className="error-text">{error}</p>}<button className="primary-button" disabled={submitting}>{submitting ? "注册中…" : "注册"}</button></form><p className="switch-text">已有账号？<button className="link-button" onClick={onLogin}>返回登录</button></p></AuthCard>;
}

function Home({ onOpen }: { onOpen: (id: string) => void }) {
  const [items, setItems] = useState<Content[]>([]); const [error, setError] = useState("");
  useEffect(() => { listContents().then((result) => setItems(result.items)).catch((err) => setError(messageOf(err))); }, []);
  return <section><div className="section-heading"><div><p className="eyebrow">DISCOVER</p><h1>找到下一段内容</h1><p className="muted">短剧内容、会员权益和付费解锁都从这里开始。</p></div></div>{error && <p className="error-text">{error}</p>}<div className="content-grid">{items.map((item) => <button className="content-card" key={item.id} onClick={() => onOpen(item.id)}><div className="cover-placeholder">{item.title.slice(0, 1)}</div><div className="content-card-body"><span className="content-type">{item.content_type}</span><h3>{item.title}</h3><p>{item.description || "暂无简介"}</p><strong>{item.price_cents ? money(item.price_cents) : "免费浏览"}</strong></div></button>)}</div>{!items.length && !error && <EmptyState text="暂无已发布内容，先进入运营台创建内容。" />}</section>;
}

function ContentDetail({ id, onBack }: { id: string; onBack: () => void }) {
  const [item, setItem] = useState<Content | null>(null); const [error, setError] = useState(""); const [videoURL, setVideoURL] = useState(""); const [message, setMessage] = useState("");
  useEffect(() => { getContent(id).then(setItem).catch((err) => setError(messageOf(err))); }, [id]);
  if (error) return <section><button className="link-button" onClick={onBack}>← 返回</button><p className="error-text">{error}</p></section>;
  if (!item) return <p className="muted">正在加载内容…</p>;
  async function purchase(itemType: "EPISODE" | "CONTENT", itemID: string) { setMessage(""); try { await createOrder({ item_type: itemType, item_id: itemID, payment_method: "WALLET" }, `${itemType}:${itemID}:${Date.now()}`); setMessage("订单已创建，请到订单页完成支付。"); } catch (err) { setMessage(messageOf(err)); } }
  async function play(episodeID: string) { try { const result = await getPlayURL(episodeID); setVideoURL(result.play_url); setMessage("播放地址已生成，有效期约 10 分钟。"); } catch (err) { setMessage(messageOf(err)); } }
  return <section><button className="link-button back-button" onClick={onBack}>← 返回内容列表</button><div className="detail-header"><div className="cover-large">{item.title.slice(0, 1)}</div><div><p className="eyebrow">{item.content_type}</p><h1>{item.title}</h1><p className="muted">{item.description || "暂无简介"}</p><button className="secondary-button" onClick={() => purchase("CONTENT", item.id)}>购买整部 {money(item.price_cents)}</button></div></div>{videoURL && <div className="video-box"><video controls src={videoURL} /></div>}{message && <p className="notice-text">{message}</p>}<div className="episode-list">{(item.episodes || []).map((episode) => <div className="episode-row" key={episode.id}><div><span className="episode-number">EP {episode.episode_no}</span><strong>{episode.title}</strong><p>{episode.summary}</p></div><div className="episode-actions"><span>{episode.access_type === "FREE" ? "免费" : money(episode.price_cents)}</span>{episode.access_type === "FREE" ? <button className="secondary-button" onClick={() => play(episode.id)}>播放</button> : <button className="secondary-button" onClick={() => purchase("EPISODE", episode.id)}>购买</button>}</div></div>)}</div></section>;
}

function Plans() {
  const [plans, setPlans] = useState<MembershipPlan[]>([]); const [message, setMessage] = useState("");
  useEffect(() => { listPlans().then((result) => setPlans(result.items)).catch((err) => setMessage(messageOf(err))); }, []);
  async function buy(plan: MembershipPlan) { try { await createOrder({ item_type: "MEMBERSHIP", item_id: plan.id, payment_method: "WALLET" }, `MEMBERSHIP:${plan.id}:${Date.now()}`); setMessage("会员订单已创建，请到订单页支付。"); } catch (err) { setMessage(messageOf(err)); } }
  return <section><p className="eyebrow">MEMBERSHIP</p><h1>选择会员计划</h1><p className="muted">固定期限会员，支付成功后异步发放会员权益。</p>{message && <p className="notice-text">{message}</p>}<div className="plan-grid">{plans.map((plan) => <div className="card plan-card" key={plan.id}><p className="eyebrow">{plan.duration_days} DAYS</p><h2>{plan.name}</h2><p className="muted">{plan.description}</p><strong className="plan-price">{money(plan.price_cents)}</strong><button className="primary-button" onClick={() => buy(plan)}>购买会员</button></div>)}</div></section>;
}

function Orders() {
  const [items, setItems] = useState<Order[]>([]); const [message, setMessage] = useState("");
  async function refresh() { try { setItems((await listOrders()).items); } catch (err) { setMessage(messageOf(err)); } }
  useEffect(() => { refresh(); }, []);
  async function walletPay(item: Order) { try { await payOrder(item.id); setMessage("支付成功，权益正在发放。"); await refresh(); } catch (err) { setMessage(messageOf(err)); } }
  async function mockPay(item: Order) { try { await mockPayment(item.order_no); setMessage("模拟支付成功，权益正在发放。"); await refresh(); } catch (err) { setMessage(messageOf(err)); } }
  return <section><p className="eyebrow">ORDERS</p><h1>我的订单</h1>{message && <p className="notice-text">{message}</p>}<div className="order-list">{items.map((item) => <div className="card order-row" key={item.id}><div><span className="order-status">{item.status}</span><h3>{item.item.item_name}</h3><p className="muted">{item.order_no} · {money(item.total_amount)}</p></div>{item.status === "PENDING_PAYMENT" && <div className="episode-actions"><button className="secondary-button" onClick={() => walletPay(item)}>钱包支付</button><button className="link-button" onClick={() => mockPay(item)}>模拟支付</button></div>}</div>)}</div>{!items.length && <EmptyState text="还没有订单。" />}</section>;
}

function WalletView() {
  const [wallet, setWallet] = useState<Wallet | null>(null); const [message, setMessage] = useState("");
  async function refresh() { try { setWallet(await getWallet()); } catch (err) { setMessage(messageOf(err)); } }
  useEffect(() => { refresh(); }, []);
  async function recharge(amount: number) { try { await rechargeWallet(amount, `recharge:${amount}:${Date.now()}`); setMessage("充值成功"); await refresh(); } catch (err) { setMessage(messageOf(err)); } }
  return <section><p className="eyebrow">WALLET</p><h1>虚拟币钱包</h1>{message && <p className="notice-text">{message}</p>}<div className="wallet-card card"><span>当前余额</span><strong>{money(wallet?.balance || 0)}</strong><div><button className="primary-button" onClick={() => recharge(1000)}>充值 ¥10</button><button className="secondary-button" onClick={() => recharge(5000)}>充值 ¥50</button></div></div><p className="muted">充值为演示用 Mock 充值，真实支付暂不接入。</p></section>;
}

function Profile({ user, onUpdated }: { user: User; onUpdated: (user: User) => void }) {
  const [nickname, setNickname] = useState(user.nickname); const [avatarURL, setAvatarURL] = useState(user.avatar_url || ""); const [message, setMessage] = useState("");
  async function save(event: FormEvent) { event.preventDefault(); try { const updated = await updateMe({ nickname, avatar_url: avatarURL }); onUpdated(updated); setMessage("保存成功"); } catch (err) { setMessage(messageOf(err)); } }
  return <section className="profile-grid"><div className="profile-intro"><p className="eyebrow">ACCOUNT</p><h1>个人账户</h1><p className="muted">{user.email}</p><div className="role-badge">{roleName(user.role)}</div></div><div className="card"><div className="avatar-placeholder">{user.nickname.slice(0, 1).toUpperCase()}</div><form onSubmit={save} className="form-stack profile-form"><label>昵称<input value={nickname} onChange={(e) => setNickname(e.target.value)} required /></label><label>头像地址<input value={avatarURL} onChange={(e) => setAvatarURL(e.target.value)} /></label>{message && <p className="notice-text">{message}</p>}<button className="primary-button">保存资料</button></form></div></section>;
}

function Operator() {
  const [items, setItems] = useState<Content[]>([]); const [selected, setSelected] = useState<Content | null>(null); const [message, setMessage] = useState(""); const [title, setTitle] = useState(""); const [description, setDescription] = useState(""); const [price, setPrice] = useState("0");
  const [episodeNo, setEpisodeNo] = useState("1"); const [episodeTitle, setEpisodeTitle] = useState(""); const [videoFile, setVideoFile] = useState<File | null>(null); const [fileInputKey, setFileInputKey] = useState(0); const [accessType, setAccessType] = useState<AccessType>("FREE"); const [episodePrice, setEpisodePrice] = useState("0"); const [uploadPercent, setUploadPercent] = useState<number | null>(null); const [uploading, setUploading] = useState(false);
  async function refresh() { try { setItems((await listOperatorContents()).items); } catch (err) { setMessage(messageOf(err)); } }
  useEffect(() => { refresh(); }, []);
  async function createContent() { try { await createOperatorContent({ title, description, price_cents: Number(price) * 100 }); setTitle(""); setDescription(""); setPrice("0"); setMessage("内容创建成功"); await refresh(); } catch (err) { setMessage(messageOf(err)); } }
  async function publish(id: string) { try { await publishOperatorContent(id); setMessage("内容已发布"); await refresh(); } catch (err) { setMessage(messageOf(err)); } }
  async function createEpisode() {
    if (!selected) return;
    if (!episodeTitle.trim() || !videoFile) { setMessage("请填写 Episode 标题并选择 MP4 视频"); return; }
    if (!videoFile.name.toLowerCase().endsWith(".mp4") || (videoFile.type && videoFile.type !== "video/mp4") || videoFile.size <= 0 || videoFile.size > 1024 * 1024 * 1024) {
      setMessage("请选择不超过 1 GiB 的 MP4 视频"); return;
    }
    setUploading(true); setUploadPercent(0); setMessage("正在申请 R2 上传地址…");
    try {
      const ticket = await createEpisodeUploadTicket(selected.id, videoFile);
      setMessage("正在上传视频到 R2…");
      await uploadEpisodeVideo(ticket.upload_url, videoFile, setUploadPercent);
      const created = await createOperatorEpisode(selected.id, { episode_no: Number(episodeNo), title: episodeTitle, summary: "", access_type: accessType, price_cents: Number(episodePrice) * 100, object_key: ticket.object_key });
      await publishOperatorEpisode(created.id);
      setMessage("上传并发布成功，可以到内容页验证播放。"); setEpisodeNo(String(Number(episodeNo) + 1)); setEpisodeTitle(""); setVideoFile(null); setFileInputKey((key) => key + 1); setUploadPercent(null);
    } catch (err) { setMessage(messageOf(err)); }
    finally { setUploading(false); }
  }
  return <section><p className="eyebrow">OPERATOR CONSOLE</p><h1>内容运营台</h1>{message && <p className="notice-text">{message}</p>}<div className="operator-layout"><div className="card"><h2>创建内容</h2><div className="form-stack"><label>标题<input value={title} onChange={(e) => setTitle(e.target.value)} /></label><label>简介<textarea value={description} onChange={(e) => setDescription(e.target.value)} /></label><label>整部价格（元）<input type="number" value={price} onChange={(e) => setPrice(e.target.value)} /></label><button className="primary-button" onClick={createContent}>创建</button></div></div><div className="card"><h2>内容列表</h2>{items.map((item) => <div className="operator-item" key={item.id}><div><strong>{item.title}</strong><span>{item.status}</span></div><div className="episode-actions"><button className="link-button" onClick={() => setSelected(item)}>添加 Episode</button>{item.status === "DRAFT" && <button className="secondary-button" onClick={() => publish(item.id)}>发布</button>}</div></div>)}</div></div>{selected && <div className="card episode-editor"><h2>为「{selected.title}」添加 Episode</h2><div className="form-stack"><label>Episode 编号<input type="number" min="1" value={episodeNo} onChange={(e) => setEpisodeNo(e.target.value)} /></label><label>Episode 标题<input value={episodeTitle} onChange={(e) => setEpisodeTitle(e.target.value)} /></label><label>选择视频（MP4，最大 1 GiB）<input key={fileInputKey} type="file" accept="video/mp4,.mp4" onChange={(e) => setVideoFile(e.target.files?.[0] || null)} /></label>{videoFile && <p className="muted">{videoFile.name} · {(videoFile.size / 1024 / 1024).toFixed(1)} MiB</p>}{uploadPercent !== null && <div className="upload-progress"><progress max="100" value={uploadPercent} /><span>{uploadPercent}%</span></div>}<label>访问规则<select value={accessType} onChange={(e) => setAccessType(e.target.value as AccessType)}><option value="FREE">免费</option><option value="PURCHASE">购买</option><option value="MEMBERSHIP">会员</option><option value="PURCHASE_OR_MEMBERSHIP">购买或会员</option></select></label><label>单集价格（元）<input type="number" value={episodePrice} onChange={(e) => setEpisodePrice(e.target.value)} /></label><button className="primary-button" disabled={uploading} onClick={createEpisode}>{uploading ? "上传并发布中…" : "上传并发布 Episode"}</button></div></div>}</section>;
}

function AuthCard({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) { return <section className="auth-layout"><div className="hero-copy"><p className="eyebrow">DIGITAL CONTENT PLATFORM</p><h1>{title}</h1><p>{subtitle}</p></div><div className="card auth-card">{children}</div></section>; }
function EmptyState({ text }: { text: string }) { return <div className="empty-state">{text}</div>; }
function roleName(role: Role) { return { USER: "普通用户", OPERATOR: "内容运营", ADMIN: "管理员" }[role]; }
function messageOf(error: unknown) { return error instanceof Error ? error.message : "请求失败"; }
