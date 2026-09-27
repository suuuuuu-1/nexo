import { FormEvent, ReactNode, useEffect, useRef, useState } from "react";
import {
  AccessType,
  clearToken,
  Content,
  createOperatorContent,
  createOperatorEpisode,
  createOrder,
  createEpisodeUploadTicket,
  getContent,
  getMe,
  getPlayURL,
  getToken,
  getWallet,
  Episode,
  listContents,
  listOperatorContents,
  listOrders,
  listPlans,
  login,
  MembershipPlan,
  mockPayment,
  Order,
  payOrder,
  publishOperatorContent,
  publishOperatorEpisode,
  rechargeWallet,
  register,
  Role,
  updateMe,
  updateProgress,
  uploadEpisodeVideo,
  User,
  Wallet,
} from "./api";

type View = "home" | "plans" | "orders" | "wallet" | "profile" | "operator";
type AuthMode = "login" | "register";

const FEATURED_IMAGE = "/images/nexo-featured.png";
const money = (cents: number) => `¥${(cents / 100).toFixed(2)}`;

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [view, setView] = useState<View>("home");
  const [selectedContentID, setSelectedContentID] = useState<string | null>(null);
  const [loadingUser, setLoadingUser] = useState(Boolean(getToken()));
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [authOpen, setAuthOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    if (!getToken()) return;
    getMe()
      .then(setUser)
      .catch(() => clearToken())
      .finally(() => setLoadingUser(false));
  }, []);

  function goTo(nextView: View) {
    setView(nextView);
    setSelectedContentID(null);
    setAuthOpen(false);
    setNotice("");
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function openAuth(mode: AuthMode = "login", message = "") {
    setAuthMode(mode);
    setNotice(message);
    setAuthOpen(true);
  }

  function logout() {
    clearToken();
    setUser(null);
    goTo("home");
    setNotice("已安全退出登录");
  }

  if (loadingUser) return <main className="full-screen-state">正在载入你的 Nexo…</main>;

  return (
    <div className="app-shell">
      <SiteHeader
        user={user}
        view={view}
        search={search}
        onSearch={setSearch}
        onNavigate={goTo}
        onLogin={() => openAuth("login")}
        onRegister={() => openAuth("register")}
        onLogout={logout}
      />

      {notice && !authOpen && <div className="global-notice" role="status">{notice}</div>}

      <main className="main-content">
        {authOpen ? (
          <AuthScreen
            mode={authMode}
            notice={notice}
            onModeChange={setAuthMode}
            onClose={() => setAuthOpen(false)}
            onSuccess={(nextUser) => {
              setUser(nextUser);
              setAuthOpen(false);
              setNotice("欢迎回来，故事已经为你准备好了。");
            }}
          />
        ) : selectedContentID ? (
          <ContentDetail
            id={selectedContentID}
            user={user}
            onBack={() => setSelectedContentID(null)}
            onLogin={() => openAuth("login", "登录后即可播放和购买内容")}
            onOrders={() => goTo("orders")}
          />
        ) : view === "home" ? (
          <Home search={search} onSearch={setSearch} onOpen={setSelectedContentID} onPlans={() => goTo("plans")} />
        ) : view === "plans" ? (
          <Plans user={user} onLogin={() => openAuth("login", "登录后即可购买会员计划")} onOrders={() => goTo("orders")} />
        ) : view === "orders" ? (
          <Orders onLogin={() => openAuth("login", "登录后查看你的订单")} />
        ) : view === "wallet" ? (
          <WalletView onLogin={() => openAuth("login", "登录后使用钱包")} />
        ) : view === "profile" ? (
          user ? <Profile user={user} onUpdated={setUser} /> : <SignInPrompt onLogin={() => openAuth("login")} />
        ) : (
          user && (user.role === "OPERATOR" || user.role === "ADMIN") ? (
            <Operator />
          ) : (
            <SignInPrompt onLogin={() => openAuth("login", "运营台仅对 OPERATOR 和 ADMIN 开放")} />
          )
        )}
      </main>

      <SiteFooter />
    </div>
  );
}

function SiteHeader({
  user,
  view,
  search,
  onSearch,
  onNavigate,
  onLogin,
  onRegister,
  onLogout,
}: {
  user: User | null;
  view: View;
  search: string;
  onSearch: (value: string) => void;
  onNavigate: (view: View) => void;
  onLogin: () => void;
  onRegister: () => void;
  onLogout: () => void;
}) {
  const navItems: { label: string; view: View }[] = [
    { label: "发现", view: "home" },
    { label: "会员", view: "plans" },
    ...(user ? [{ label: "订单", view: "orders" as View }, { label: "钱包", view: "wallet" as View }] : []),
    ...(user && (user.role === "OPERATOR" || user.role === "ADMIN") ? [{ label: "运营台", view: "operator" as View }] : []),
  ];

  return (
    <header className="site-header">
      <div className="header-main">
        <button className="brand" onClick={() => onNavigate("home")} aria-label="Nexo 首页">
          <span className="brand-mark">N</span>
          <span>NEXO</span>
        </button>

        <nav className="primary-nav" aria-label="主导航">
          {navItems.map((item) => (
            <button
              key={item.view}
              className={view === item.view ? "nav-link active" : "nav-link"}
              onClick={() => onNavigate(item.view)}
              aria-current={view === item.view ? "page" : undefined}
            >
              {item.label}
            </button>
          ))}
        </nav>

        <label className="header-search">
          <SearchIcon />
          <input value={search} onChange={(event) => onSearch(event.target.value)} placeholder="搜作品、搜故事" aria-label="搜索作品" />
          {search && <button type="button" onClick={() => onSearch("")} aria-label="清除搜索">×</button>}
        </label>

        <div className="header-account">
          {user ? (
            <>
              <button className="account-chip" onClick={() => onNavigate("profile")} title={`${roleName(user.role)} · ${user.email}`}>
                <span className="avatar-small">{initial(user.nickname)}</span>
                <span className="account-name">{user.nickname}</span>
              </button>
              <button className="quiet-button logout-button" onClick={onLogout}>退出</button>
            </>
          ) : (
            <>
              <button className="quiet-button login-button" onClick={onLogin}>登录</button>
              <button className="header-register" onClick={onRegister}>注册</button>
            </>
          )}
        </div>
      </div>

      <nav className="mobile-nav" aria-label="移动端导航">
        {navItems.map((item) => (
          <button key={item.view} className={view === item.view ? "mobile-nav-link active" : "mobile-nav-link"} onClick={() => onNavigate(item.view)}>
            {item.label}
          </button>
        ))}
        {user && <button className="mobile-nav-link" onClick={() => onNavigate("profile")}>账户</button>}
      </nav>
    </header>
  );
}

function Home({ search, onSearch, onOpen, onPlans }: { search: string; onSearch: (value: string) => void; onOpen: (id: string) => void; onPlans: () => void }) {
  const [items, setItems] = useState<Content[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const catalogRef = useRef<HTMLElement>(null);

  async function refresh() {
    setError("");
    setLoading(true);
    try {
      setItems((await listContents()).items || []);
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void refresh(); }, []);

  const query = search.trim().toLocaleLowerCase();
  const visibleItems = items.filter((item) => !query || `${item.title} ${item.description} ${item.content_type}`.toLocaleLowerCase().includes(query));
  const featured = items[0];

  function explore() {
    if (featured) onOpen(featured.id);
    else catalogRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  return (
    <div className="home-page">
      <section className="hero-banner" aria-label="Nexo 精选">
        <img className="hero-image" src={FEATURED_IMAGE} alt="暮色山谷中的奇幻旅人" />
        <div className="hero-shade" />
        <div className="hero-copy">
          <span className="eyebrow hero-eyebrow"><span className="live-dot" /> NEXO · 故事放映厅</span>
          <h1>好故事，<br />值得一集一集看。</h1>
          <p>从免费试看，到单集解锁和会员订阅，<br className="desktop-only" />每一种观看权益都清楚、简单。</p>
          <div className="hero-actions">
            <button className="button-primary" onClick={explore}><PlayIcon /> {featured ? "打开精选" : "浏览内容"}</button>
            <button className="button-glass" onClick={onPlans}>看看会员计划 <ArrowIcon /></button>
          </div>
          <div className="hero-footnote"><span>内容 · 订阅 · 权益</span><span className="footnote-divider" /><span>为每一种好故事留一个位置</span></div>
        </div>
        {featured && (
          <button className="hero-feature-chip" onClick={() => onOpen(featured.id)}>
            <span className="feature-chip-label">本期精选</span>
            <strong>{featured.title}</strong>
            <span className="feature-chip-cta">查看详情 <ArrowIcon /></span>
          </button>
        )}
        <span className="hero-index">01 <i /> 03</span>
      </section>

      <section className="benefit-strip" aria-label="平台权益">
        <div className="benefit-item"><span className="benefit-icon">01</span><div><strong>先看，再决定</strong><span>免费章节直接打开</span></div></div>
        <div className="benefit-item"><span className="benefit-icon">02</span><div><strong>单集或整部解锁</strong><span>按你的观看习惯选择</span></div></div>
        <div className="benefit-item"><span className="benefit-icon">03</span><div><strong>会员权益一目了然</strong><span>购买后统一校验访问权限</span></div></div>
      </section>

      <section className="catalog-section" id="catalog" ref={catalogRef}>
        <div className="section-heading">
          <div>
            <p className="eyebrow">THE LIBRARY</p>
            <h2>{query ? "搜索结果" : "正在这里，发现新故事"}</h2>
            <p className="section-subtitle">{query ? `包含「${search.trim()}」的作品` : "一部内容，一个入口；想从哪里开始，由你决定。"}</p>
          </div>
          <span className="result-count">{loading ? "正在更新…" : `${visibleItems.length} 部作品`}</span>
        </div>

        {error && <MessageBanner tone="error" action={<button className="text-action" onClick={() => void refresh()}>重新加载</button>}>{error}</MessageBanner>}
        {loading ? (
          <div className="content-grid" aria-label="正在加载作品">
            {Array.from({ length: 4 }, (_, index) => <div className="content-skeleton" key={index}><div /><span /><i /></div>)}
          </div>
        ) : visibleItems.length ? (
          <div className="content-grid">
            {visibleItems.map((item, index) => <ContentCard key={item.id} item={item} index={index} onOpen={() => onOpen(item.id)} />)}
          </div>
        ) : (
          <EmptyState
            title={items.length ? "没有找到匹配的作品" : "片单正在准备中"}
            text={items.length ? "试试更短的关键词，或者清空搜索条件。" : "运营人员发布内容后，这里就会出现第一部作品。"}
            action={query ? <button className="button-secondary" onClick={() => onSearch("")}>清空搜索</button> : undefined}
          />
        )}
      </section>
    </div>
  );
}

function ContentCard({ item, index, onOpen }: { item: Content; index: number; onOpen: () => void }) {
  return (
    <button className="content-card" onClick={onOpen}>
      <div className={`poster-art poster-tone-${index % 4}`}>
        {index === 0 && <img src={FEATURED_IMAGE} alt="" loading="lazy" />}
        <span className="poster-vignette" />
        <span className="poster-kicker">{contentTypeName(item.content_type)}</span>
        <span className="poster-title">{item.title}</span>
        <span className="poster-symbol">{item.title.slice(0, 1)}</span>
        <span className="poster-play"><PlayIcon /></span>
      </div>
      <div className="content-card-copy">
        <div className="card-meta"><span>{contentTypeName(item.content_type)}</span><span>{item.price_cents > 0 ? "整部可购" : "开放浏览"}</span></div>
        <strong>{item.title}</strong>
        <p>{item.description || "一段新的故事，等你打开。"}</p>
        <div className="card-bottom"><span>{item.price_cents > 0 ? `整部 ${money(item.price_cents)}` : "免费浏览"}</span><span className="card-arrow"><ArrowIcon /></span></div>
      </div>
    </button>
  );
}

function ContentDetail({
  id,
  user,
  onBack,
  onLogin,
  onOrders,
}: {
  id: string;
  user: User | null;
  onBack: () => void;
  onLogin: () => void;
  onOrders: () => void;
}) {
  const [item, setItem] = useState<Content | null>(null);
  const [error, setError] = useState("");
  const [videoURL, setVideoURL] = useState("");
  const [playingEpisodeID, setPlayingEpisodeID] = useState("");
  const [message, setMessage] = useState("");
  const [loadingPlay, setLoadingPlay] = useState(false);
  const lastProgressSave = useRef(0);

  useEffect(() => {
    setItem(null);
    setError("");
    setVideoURL("");
    setPlayingEpisodeID("");
    getContent(id).then(setItem).catch((err) => setError(messageOf(err)));
  }, [id]);

  if (error) return <PageError message={error} onBack={onBack} />;
  if (!item) return <div className="detail-loading"><div className="loading-orb" /><span>正在打开作品…</span></div>;

  async function purchase(itemType: "EPISODE" | "CONTENT", itemID: string) {
    if (!user) { onLogin(); return; }
    setMessage("");
    try {
      await createOrder({ item_type: itemType, item_id: itemID, payment_method: "WALLET" }, `${itemType}:${itemID}:${Date.now()}`);
      onOrders();
    } catch (err) {
      setMessage(messageOf(err));
    }
  }

  async function play(episodeID: string) {
    if (!user) { onLogin(); return; }
    setLoadingPlay(true);
    setMessage("");
    try {
      const result = await getPlayURL(episodeID);
      setVideoURL(result.play_url);
      setPlayingEpisodeID(episodeID);
      lastProgressSave.current = 0;
      setMessage("访问权限已通过校验，播放地址为短期签名链接。");
      window.setTimeout(() => document.getElementById("nexo-player")?.scrollIntoView({ behavior: "smooth", block: "center" }), 80);
    } catch (err) {
      setMessage(messageOf(err));
    } finally {
      setLoadingPlay(false);
    }
  }

  function saveProgress(video: HTMLVideoElement, force = false) {
    if (!user || !playingEpisodeID || !Number.isFinite(video.duration) || video.duration <= 0) return;
    const position = Math.floor(video.currentTime);
    if (position < 1 || (!force && position - lastProgressSave.current < 15)) return;
    lastProgressSave.current = position;
    void updateProgress(playingEpisodeID, { position_seconds: position, duration_seconds: Math.floor(video.duration) }).catch(() => undefined);
  }

  return (
    <section className="detail-page">
      <button className="back-link" onClick={onBack}><ArrowLeftIcon /> 返回内容库</button>
      <div className="detail-hero">
        <img src={FEATURED_IMAGE} alt="" />
        <div className="detail-hero-shade" />
        <div className="detail-hero-copy">
          <div className="detail-meta"><span className="content-type-pill">{contentTypeName(item.content_type)}</span><span>数字内容 · Nexo 放映</span></div>
          <h1>{item.title}</h1>
          <p>{item.description || "一段新的故事，等你打开。"}</p>
          <div className="detail-actions">
          <button className="button-primary" onClick={() => {
            const firstEpisode = item.episodes?.find((episode) => episode.access_type === "FREE") || item.episodes?.[0];
            if (firstEpisode) void play(firstEpisode.id);
          }} disabled={!item.episodes?.length || loadingPlay}>
              <PlayIcon /> {loadingPlay ? "正在校验权益…" : "开始观看"}
            </button>
            {item.price_cents > 0 && <button className="button-glass" onClick={() => void purchase("CONTENT", item.id)}>整部解锁 · {money(item.price_cents)}</button>}
          </div>
        </div>
        <div className="detail-hero-note"><span>权益由服务端统一校验</span><i />短期播放地址</div>
      </div>

      {videoURL && (
        <div className="player-panel" id="nexo-player">
          <div className="player-heading"><div><p className="eyebrow">NOW PLAYING</p><h2>{item.episodes?.find((episode) => episode.id === playingEpisodeID)?.title || item.title}</h2></div><span className="secure-label"><span /> 权限已验证</span></div>
          <video key={videoURL} controls autoPlay playsInline src={videoURL} onTimeUpdate={(event) => saveProgress(event.currentTarget)} onPause={(event) => saveProgress(event.currentTarget, true)} onEnded={(event) => saveProgress(event.currentTarget, true)} />
          <p className="player-caption">观看进度会自动保存到你的账户。</p>
        </div>
      )}

      {message && <MessageBanner tone={message.startsWith("访问权限") ? "success" : "error"}>{message}</MessageBanner>}

      <div className="episode-section">
        <div className="section-heading compact-heading"><div><p className="eyebrow">EPISODES</p><h2>分集与观看权益</h2></div><span className="result-count">{item.episodes?.length || 0} 集</span></div>
        {item.episodes?.length ? (
          <div className="episode-list">
            {item.episodes.map((episode) => <EpisodeRow key={episode.id} episode={episode} playing={playingEpisodeID === episode.id} onPlay={() => void play(episode.id)} onBuy={() => void purchase("EPISODE", episode.id)} loading={loadingPlay} />)}
          </div>
        ) : (
          <EmptyState title="分集正在准备中" text="运营人员添加并发布 Episode 后，就会显示在这里。" />
        )}
      </div>
    </section>
  );
}

function EpisodeRow({ episode, playing, onPlay, onBuy, loading }: { episode: Episode; playing: boolean; onPlay: () => void; onBuy: () => void; loading: boolean }) {
  const isFree = episode.access_type === "FREE";
  const isMember = episode.access_type === "MEMBERSHIP";
  const isEither = episode.access_type === "PURCHASE_OR_MEMBERSHIP";
  const accessLabel = isFree ? "免费试看" : isMember ? "会员专享" : isEither ? "购买或会员" : `单集 ${money(episode.price_cents)}`;

  return (
    <article className={playing ? "episode-row playing" : "episode-row"}>
      <span className="episode-play-index">{playing ? <span className="equalizer"><i /><i /><i /></span> : <span>{String(episode.episode_no).padStart(2, "0")}</span>}</span>
      <div className="episode-copy"><div className="episode-title-line"><h3>{episode.title}</h3><span className={`access-tag ${isFree ? "free" : "paid"}`}>{accessLabel}</span></div><p>{episode.summary || "按下播放，继续这个故事。"}</p></div>
      <div className="episode-row-action">
        <button className="icon-play-button" onClick={onPlay} disabled={loading} aria-label={`播放${episode.title}`}><PlayIcon /></button>
        {!isFree && !isMember && <button className="button-secondary small" onClick={onBuy}>购买</button>}
        {isMember && <span className="episode-price-note">会员可看</span>}
      </div>
    </article>
  );
}

function Plans({ user, onLogin, onOrders }: { user: User | null; onLogin: () => void; onOrders: () => void }) {
  const [plans, setPlans] = useState<MembershipPlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    listPlans().then((result) => setPlans(result.items || [])).catch((err) => setError(messageOf(err))).finally(() => setLoading(false));
  }, []);

  async function buy(plan: MembershipPlan) {
    if (!user) { onLogin(); return; }
    try {
      await createOrder({ item_type: "MEMBERSHIP", item_id: plan.id, payment_method: "WALLET" }, `MEMBERSHIP:${plan.id}:${Date.now()}`);
      onOrders();
    } catch (err) {
      setError(messageOf(err));
    }
  }

  return (
    <section className="subpage plans-page">
      <PageIntro eyebrow="MEMBERSHIP" title="把喜欢的故事，继续看下去。" subtitle="订阅是访问权益，不是另一份内容；购买成功后由系统自动发放。" />
      {error && <MessageBanner tone="error">{error}</MessageBanner>}
      {loading ? <div className="detail-loading"><div className="loading-orb" /><span>正在载入计划…</span></div> : plans.length ? (
        <div className="plan-grid">
          {plans.map((plan, index) => (
            <article className={`plan-card ${index === 1 ? "featured-plan" : ""}`} key={plan.id}>
              {index === 1 && <span className="plan-ribbon">人气选择</span>}
              <p className="eyebrow">NEXO PASS · {plan.duration_days} DAYS</p>
              <h2>{plan.name}</h2>
              <p className="plan-description">{plan.description || "会员有效期内，按内容规则访问会员专享作品。"}</p>
              <div className="plan-price">{money(plan.price_cents)}<span> / {plan.duration_days} 天</span></div>
              <div className="plan-perks"><span><CheckIcon /> 会员内容访问权益</span><span><CheckIcon /> 订单与订阅状态可追踪</span><span><CheckIcon /> 到期时间清楚可见</span></div>
              <button className={index === 1 ? "button-primary plan-buy" : "button-secondary plan-buy"} onClick={() => void buy(plan)}>选择这个计划 <ArrowIcon /></button>
            </article>
          ))}
        </div>
      ) : (
        <EmptyState title="暂时没有可购买的计划" text="运营配置会员计划后，会显示在这里。" />
      )}
    </section>
  );
}

function Orders({ onLogin }: { onLogin: () => void }) {
  const [items, setItems] = useState<Order[]>([]);
  const [userReady, setUserReady] = useState(Boolean(getToken()));
  const [loading, setLoading] = useState(Boolean(getToken()));
  const [message, setMessage] = useState("");

  async function refresh() {
    if (!getToken()) { setUserReady(false); setLoading(false); return; }
    setUserReady(true);
    setLoading(true);
    try { setItems((await listOrders()).items || []); setMessage(""); }
    catch (err) { setMessage(messageOf(err)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void refresh(); }, []);

  async function walletPay(item: Order) {
    try { await payOrder(item.id); setMessage("钱包扣款成功，权益正在异步发放。"); await refresh(); }
    catch (err) { setMessage(messageOf(err)); }
  }

  async function testPay(item: Order) {
    try { await mockPayment(item.order_no); setMessage("模拟支付回调已接收，权益正在异步发放。"); await refresh(); }
    catch (err) { setMessage(messageOf(err)); }
  }

  if (!userReady) return <SignInPrompt onLogin={onLogin} title="订单属于你的账户" text="登录后可以查看订单状态、支付方式和权益履约进度。" />;

  return (
    <section className="subpage">
      <PageIntro eyebrow="YOUR ORDERS" title="每笔订单，都有清楚的状态。" subtitle="支付成功后，内容权益会通过后台履约流程发放。" />
      {message && <MessageBanner tone={message.includes("成功") ? "success" : "info"}>{message}</MessageBanner>}
      <div className="list-toolbar"><span>{loading ? "正在同步…" : `${items.length} 笔订单`}</span><button className="text-action" onClick={() => void refresh()}>刷新列表 <RefreshIcon /></button></div>
      {loading ? <div className="order-skeleton" /> : items.length ? (
        <div className="order-list">
          {items.map((item) => (
            <article className="order-card" key={item.id}>
              <div className="order-card-main"><span className={`order-status status-${item.status.toLowerCase()}`}>{orderStatusName(item.status)}</span><h3>{item.item.item_name}</h3><p>订单号 {item.order_no}</p></div>
              <div className="order-card-detail"><span>支付方式 · {item.payment_method === "WALLET" ? "钱包余额" : "模拟支付"}</span><strong>{money(item.total_amount)}</strong></div>
              {item.status === "PENDING_PAYMENT" && <div className="order-card-actions"><button className="button-secondary small" onClick={() => void walletPay(item)}>钱包支付</button><button className="text-action" onClick={() => void testPay(item)}>模拟支付回调</button></div>}
            </article>
          ))}
        </div>
      ) : <EmptyState title="还没有订单" text="浏览一部作品，购买单集或会员计划后，订单会出现在这里。" />}
    </section>
  );
}

function WalletView({ onLogin }: { onLogin: () => void }) {
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(Boolean(getToken()));

  async function refresh() {
    if (!getToken()) { setLoading(false); return; }
    try { setWallet(await getWallet()); setMessage(""); }
    catch (err) { setMessage(messageOf(err)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void refresh(); }, []);

  async function recharge(amount: number) {
    if (!getToken()) { onLogin(); return; }
    setLoading(true);
    try { setWallet(await rechargeWallet(amount, `recharge:${amount}:${Date.now()}`)); setMessage("演示充值已完成，余额和账本已更新。"); }
    catch (err) { setMessage(messageOf(err)); }
    finally { setLoading(false); }
  }

  if (!getToken()) return <SignInPrompt onLogin={onLogin} title="钱包安全地绑定在账户上" text="登录后可以查看余额，并用演示充值体验钱包支付。" />;

  return (
    <section className="subpage wallet-page">
      <PageIntro eyebrow="NEXO WALLET" title="购买之前，先看看钱包。" subtitle="余额以最小货币单位存储；扣款通过数据库事务和钱包行锁完成。" />
      {message && <MessageBanner tone={message.includes("完成") ? "success" : "info"}>{message}</MessageBanner>}
      <div className="wallet-layout">
        <article className="wallet-balance-card"><span className="wallet-overline">AVAILABLE BALANCE</span><strong>{loading && !wallet ? "—" : money(wallet?.balance || 0)}</strong><span className="wallet-caption">Nexo 演示余额</span><div className="wallet-card-orbit orbit-one" /><div className="wallet-card-orbit orbit-two" /></article>
        <article className="wallet-recharge-card"><p className="eyebrow">DEMO TOP-UP</p><h2>添加演示余额</h2><p>充值接口使用幂等键；重复提交不会重复入账。</p><div className="recharge-options"><button onClick={() => void recharge(1000)} disabled={loading}>充入 <strong>¥10</strong></button><button onClick={() => void recharge(5000)} disabled={loading}>充入 <strong>¥50</strong></button></div><span className="small-note">仅用于本地 Demo，不接入真实支付渠道。</span></article>
      </div>
    </section>
  );
}

function Profile({ user, onUpdated }: { user: User; onUpdated: (user: User) => void }) {
  const [nickname, setNickname] = useState(user.nickname);
  const [avatarURL, setAvatarURL] = useState(user.avatar_url || "");
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setMessage("");
    try { const updated = await updateMe({ nickname, avatar_url: avatarURL }); onUpdated(updated); setMessage("资料已保存。"); }
    catch (err) { setMessage(messageOf(err)); }
    finally { setSaving(false); }
  }

  return (
    <section className="subpage profile-page">
      <PageIntro eyebrow="YOUR ACCOUNT" title="账户资料" subtitle="角色权限由服务端决定，个人资料只包含你可以编辑的字段。" />
      <div className="profile-layout">
        <aside className="profile-summary"><div className="avatar-large">{initial(user.nickname)}</div><h2>{user.nickname}</h2><p>{user.email}</p><span className="role-pill">{roleName(user.role)}</span></aside>
        <form className="form-card" onSubmit={save}><p className="eyebrow">PROFILE DETAILS</p><h2>更新公开资料</h2><label>昵称<input value={nickname} onChange={(event) => setNickname(event.target.value)} maxLength={64} required /></label><label>头像地址（可选）<input type="url" value={avatarURL} onChange={(event) => setAvatarURL(event.target.value)} placeholder="https://…" /></label>{message && <MessageBanner tone={message.includes("保存") ? "success" : "error"}>{message}</MessageBanner>}<button className="button-primary" disabled={saving}>{saving ? "保存中…" : "保存更改"}</button></form>
      </div>
    </section>
  );
}

function Operator() {
  const [items, setItems] = useState<Content[]>([]);
  const [selected, setSelected] = useState<Content | null>(null);
  const [message, setMessage] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [price, setPrice] = useState("0");
  const [episodeNo, setEpisodeNo] = useState("1");
  const [episodeTitle, setEpisodeTitle] = useState("");
  const [videoFile, setVideoFile] = useState<File | null>(null);
  const [fileInputKey, setFileInputKey] = useState(0);
  const [accessType, setAccessType] = useState<AccessType>("FREE");
  const [episodePrice, setEpisodePrice] = useState("0");
  const [uploadPercent, setUploadPercent] = useState<number | null>(null);
  const [uploading, setUploading] = useState(false);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    try { setItems((await listOperatorContents()).items || []); setMessage(""); }
    catch (err) { setMessage(messageOf(err)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void refresh(); }, []);

  async function createContent() {
    if (!title.trim()) { setMessage("请先填写作品名称。"); return; }
    const priceValue = Number(price);
    if (!Number.isFinite(priceValue) || priceValue < 0) { setMessage("整部价格不能小于 0。"); return; }
    try { await createOperatorContent({ title: title.trim(), description: description.trim(), price_cents: Math.round(priceValue * 100) }); setTitle(""); setDescription(""); setPrice("0"); setMessage("作品草稿已创建；添加至少一集后再发布。"); await refresh(); }
    catch (err) { setMessage(messageOf(err)); }
  }

  async function publish(id: string) {
    try { await publishOperatorContent(id); setMessage("作品已发布到内容库。"); await refresh(); }
    catch (err) { setMessage(messageOf(err)); }
  }

  async function createEpisode() {
    if (!selected) return;
    if (!episodeTitle.trim() || !videoFile) { setMessage("请填写分集名称并选择 MP4 视频。"); return; }
    const priceValue = Number(episodePrice);
    if (!Number.isFinite(priceValue) || priceValue < 0) { setMessage("单集价格不能小于 0。"); return; }
    if (!videoFile.name.toLowerCase().endsWith(".mp4") || (videoFile.type && videoFile.type !== "video/mp4") || videoFile.size <= 0 || videoFile.size > 1024 * 1024 * 1024) {
      setMessage("请选择不超过 1 GiB 的 MP4 视频。"); return;
    }

    setUploading(true);
    setUploadPercent(0);
    setMessage("正在申请 R2 上传地址…");
    try {
      const ticket = await createEpisodeUploadTicket(selected.id, videoFile);
      setMessage("正在将视频直传到 R2…");
      await uploadEpisodeVideo(ticket.upload_url, videoFile, setUploadPercent);
      const created = await createOperatorEpisode(selected.id, {
        episode_no: Number(episodeNo),
        title: episodeTitle.trim(),
        summary: "",
        access_type: accessType,
        price_cents: Math.round(priceValue * 100),
        object_key: ticket.object_key,
      });
      await publishOperatorEpisode(created.id);
      setMessage("上传并发布成功；现在可以用用户账户验证播放和权益规则。");
      setEpisodeNo(String(Number(episodeNo) + 1));
      setEpisodeTitle("");
      setVideoFile(null);
      setFileInputKey((key) => key + 1);
      setUploadPercent(null);
      await refresh();
    } catch (err) {
      setMessage(messageOf(err));
    } finally {
      setUploading(false);
    }
  }

  return (
    <section className="subpage operator-page">
      <PageIntro eyebrow="STUDIO / OPERATOR" title="把一部作品，整理成可访问的内容。" subtitle="先创建草稿，再上传 Episode；发布前会校验 R2 对象与视频元数据。" />
      {message && <MessageBanner tone={message.includes("成功") || message.includes("已发布") ? "success" : "info"}>{message}</MessageBanner>}
      <div className="operator-layout">
        <form className="form-card" onSubmit={(event) => { event.preventDefault(); void createContent(); }}>
          <p className="eyebrow">01 · NEW TITLE</p><h2>创建作品</h2>
          <label>作品名称<input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="例如：雾海来信" required /></label>
          <label>作品简介<textarea value={description} onChange={(event) => setDescription(event.target.value)} rows={4} placeholder="用几句话介绍这段故事" /></label>
          <label>整部购买价格（元）<input type="number" min="0" step="0.01" value={price} onChange={(event) => setPrice(event.target.value)} /></label>
          <button className="button-primary" type="submit">创建草稿 <ArrowIcon /></button>
        </form>

        <div className="form-card operator-library">
          <div className="operator-heading"><div><p className="eyebrow">02 · LIBRARY</p><h2>作品管理</h2></div><button className="text-action" onClick={() => void refresh()}>刷新 <RefreshIcon /></button></div>
          {loading ? <div className="detail-loading"><div className="loading-orb" /><span>载入草稿…</span></div> : items.length ? items.map((item) => (
            <article className="operator-title-row" key={item.id}>
              <div className="operator-title-icon">{initial(item.title)}</div>
              <div className="operator-title-copy"><strong>{item.title}</strong><span>{statusName(item.status)} · {money(item.price_cents)}</span></div>
              <div className="operator-row-actions"><button className="text-action" onClick={() => setSelected(item)}>添加分集</button>{item.status === "DRAFT" && <button className="button-secondary small" onClick={() => void publish(item.id)}>发布作品</button>}</div>
            </article>
          )) : <EmptyState title="内容库还是空的" text="创建一部作品草稿，就可以开始添加分集。" />}
        </div>
      </div>

      {selected && (
        <div className="form-card episode-editor">
          <div className="operator-heading"><div><p className="eyebrow">03 · EPISODE UPLOAD</p><h2>为「{selected.title}」添加分集</h2></div><button className="close-button" onClick={() => setSelected(null)} aria-label="关闭分集表单">×</button></div>
          <div className="episode-form-grid">
            <label>集数<input type="number" min="1" value={episodeNo} onChange={(event) => setEpisodeNo(event.target.value)} /></label>
            <label>分集名称<input value={episodeTitle} onChange={(event) => setEpisodeTitle(event.target.value)} placeholder="例如：第一集 · 雨夜" /></label>
            <label className="upload-field">MP4 视频（最大 1 GiB）<input key={fileInputKey} type="file" accept="video/mp4,.mp4" onChange={(event) => setVideoFile(event.target.files?.[0] || null)} /></label>
            <label>访问规则<select value={accessType} onChange={(event) => setAccessType(event.target.value as AccessType)}><option value="FREE">免费</option><option value="PURCHASE">单集购买</option><option value="MEMBERSHIP">会员专享</option><option value="PURCHASE_OR_MEMBERSHIP">购买或会员</option></select></label>
            <label>单集价格（元）<input type="number" min="0" step="0.01" value={episodePrice} onChange={(event) => setEpisodePrice(event.target.value)} /></label>
          </div>
          {videoFile && <p className="file-note">已选择：{videoFile.name} · {(videoFile.size / 1024 / 1024).toFixed(1)} MiB</p>}
          {uploadPercent !== null && <div className="upload-progress"><progress max="100" value={uploadPercent} /><span>{uploadPercent}%</span></div>}
          <div className="episode-form-actions"><span>文件会从浏览器直传到 Cloudflare R2，不经过 Go API。</span><button className="button-primary" disabled={uploading} onClick={() => void createEpisode()}>{uploading ? "正在上传并发布…" : "上传并发布分集"} <ArrowIcon /></button></div>
        </div>
      )}
    </section>
  );
}

function AuthScreen({ mode, notice, onModeChange, onClose, onSuccess }: { mode: AuthMode; notice: string; onModeChange: (mode: AuthMode) => void; onClose: () => void; onSuccess: (user: User) => void }) {
  const [email, setEmail] = useState("");
  const [nickname, setNickname] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (mode === "register" && password.length < 8) { setError("密码至少需要 8 位。"); return; }
    if (mode === "register" && password !== confirmPassword) { setError("两次输入的密码不一致。"); return; }
    setSubmitting(true);
    try {
      const nextUser = mode === "login"
        ? await login({ email, password })
        : await register({ email, nickname, password });
      onSuccess(nextUser);
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="auth-stage">
      <div className="auth-visual"><img src={FEATURED_IMAGE} alt="" /><div className="auth-visual-shade" /><div><span className="eyebrow">NEXO · STORIES, WITH RIGHTS</span><h1>想看的故事，<br />现在开始。</h1><p>内容由你发现，权益由 Nexo 认真管理。</p></div></div>
      <div className="auth-panel">
        <button className="back-link" onClick={onClose}><ArrowLeftIcon /> 返回浏览</button>
        <div className="auth-panel-heading"><p className="eyebrow">YOUR NEXO ACCOUNT</p><h2>{mode === "login" ? "欢迎回来" : "创建一个账户"}</h2><p>{mode === "login" ? "登录后继续播放、购买和管理权益。" : "注册后即可开始收藏你的下一段故事。"}</p></div>
        {notice && <MessageBanner tone="info">{notice}</MessageBanner>}
        <form className="form-stack" onSubmit={(event) => void submit(event)}>
          <label>邮箱<input type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} required /></label>
          {mode === "register" && <label>昵称（可选）<input value={nickname} onChange={(event) => setNickname(event.target.value)} maxLength={64} placeholder="怎么称呼你？" /></label>}
          <div className="form-field">
            <label htmlFor="auth-password">密码</label>
            <div className="password-input-wrap">
              <input id="auth-password" type={showPassword ? "text" : "password"} autoComplete={mode === "login" ? "current-password" : "new-password"} value={password} onChange={(event) => setPassword(event.target.value)} minLength={mode === "register" ? 8 : undefined} required />
              <button className="password-toggle" type="button" onClick={() => setShowPassword((visible) => !visible)} aria-label={showPassword ? "隐藏密码" : "显示密码"} aria-pressed={showPassword}>{showPassword ? "隐藏" : "显示"}</button>
            </div>
          </div>
          {mode === "register" && (
            <div className="form-field">
              <label htmlFor="auth-confirm-password">确认密码</label>
              <div className="password-input-wrap">
                <input id="auth-confirm-password" type={showConfirmPassword ? "text" : "password"} autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} required aria-describedby="confirm-password-hint" />
                <button className="password-toggle" type="button" onClick={() => setShowConfirmPassword((visible) => !visible)} aria-label={showConfirmPassword ? "隐藏密码" : "显示密码"} aria-pressed={showConfirmPassword}>{showConfirmPassword ? "隐藏" : "显示"}</button>
              </div>
              <span className={confirmPassword && password !== confirmPassword ? "field-hint mismatch" : "field-hint"} id="confirm-password-hint">
                {confirmPassword && password !== confirmPassword ? "两次输入的密码不一致。" : "再输入一次密码以确认。"}
              </span>
            </div>
          )}
          {error && <MessageBanner tone="error">{error}</MessageBanner>}
          <button className="button-primary auth-submit" disabled={submitting}>{submitting ? "请稍候…" : mode === "login" ? "登录 Nexo" : "创建账户"} <ArrowIcon /></button>
        </form>
        <p className="auth-switch">{mode === "login" ? "第一次来？" : "已经有账户？"}<button className="text-action" onClick={() => { setError(""); setConfirmPassword(""); setShowPassword(false); setShowConfirmPassword(false); onModeChange(mode === "login" ? "register" : "login"); }}>{mode === "login" ? "注册一个账户" : "返回登录"}</button></p>
        <p className="auth-footnote">公开内容无需登录即可浏览；播放和购买时再登录也来得及。</p>
      </div>
    </section>
  );
}

function SignInPrompt({ onLogin, title = "登录后继续", text = "登录 Nexo 后，就能查看账户专属内容和权益。" }: { onLogin: () => void; title?: string; text?: string }) {
  return <section className="sign-in-prompt"><div className="prompt-symbol"><span>N</span></div><p className="eyebrow">NEXO ACCOUNT</p><h1>{title}</h1><p>{text}</p><button className="button-primary" onClick={onLogin}>登录 Nexo <ArrowIcon /></button></section>;
}

function PageIntro({ eyebrow, title, subtitle }: { eyebrow: string; title: string; subtitle: string }) {
  return <header className="page-intro"><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p>{subtitle}</p></header>;
}

function EmptyState({ title, text, action }: { title: string; text: string; action?: ReactNode }) {
  return <div className="empty-state"><span className="empty-orbit"><i /></span><h3>{title}</h3><p>{text}</p>{action}</div>;
}

function MessageBanner({ children, tone = "info", action }: { children: ReactNode; tone?: "info" | "success" | "error"; action?: ReactNode }) {
  return <div className={`message-banner tone-${tone}`} role={tone === "error" ? "alert" : "status"}><span className="message-indicator" /><span>{children}</span>{action && <div className="message-action">{action}</div>}</div>;
}

function PageError({ message, onBack }: { message: string; onBack: () => void }) {
  return <section className="error-page"><p className="eyebrow">NOT AVAILABLE</p><h1>暂时无法打开这部作品</h1><p>{message}</p><button className="button-secondary" onClick={onBack}><ArrowLeftIcon /> 返回内容库</button></section>;
}

function SiteFooter() {
  return <footer className="site-footer"><button className="brand footer-brand" onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}><span className="brand-mark">N</span><span>NEXO</span></button><p>内容属于作品，权益属于用户。Nexo 负责把两者清楚地连接起来。</p><span>© Nexo · V1 Demo</span></footer>;
}

function roleName(role: Role) { return { USER: "普通用户", OPERATOR: "内容运营", ADMIN: "管理员" }[role]; }
function initial(value: string) { return value.trim().slice(0, 1).toLocaleUpperCase() || "N"; }
function contentTypeName(type: string) { return type === "VIDEO" ? "短片 / 影集" : type; }
function statusName(status: string) { return ({ DRAFT: "草稿", PUBLISHED: "已发布", OFFLINE: "已下线" } as Record<string, string>)[status] || status; }
function orderStatusName(status: string) { return ({ PENDING_PAYMENT: "待支付", PAID: "已支付", FULFILLING: "权益发放中", COMPLETED: "已完成", CANCELLED: "已取消", FAILED: "失败" } as Record<string, string>)[status] || status; }
function messageOf(error: unknown) { return error instanceof Error ? error.message : "请求失败，请稍后重试。"; }

function SearchIcon() { return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="10.8" cy="10.8" r="6.5" /><path d="m16 16 4.2 4.2" /></svg>; }
function PlayIcon() { return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m8 5 11 7-11 7z" /></svg>; }
function ArrowIcon() { return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 12h15M13 5l7 7-7 7" /></svg>; }
function ArrowLeftIcon() { return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M20 12H5m7 7-7-7 7-7" /></svg>; }
function CheckIcon() { return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg>; }
function RefreshIcon() { return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M20 7v5h-5M4 17v-5h5" /><path d="M6.2 9A7 7 0 0 1 18 6l2 2M4 16l2 2a7 7 0 0 0 11.8-3" /></svg>; }
