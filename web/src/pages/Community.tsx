import {
  useDeferredValue,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { useQuery } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import {
  ArrowDown,
  ArrowRight,
  Award,
  Bookmark,
  Check,
  ChefHat,
  Clock3,
  Heart,
  ImagePlus,
  Leaf,
  Medal,
  Plus,
  Search,
  Sparkles,
  Sprout,
  Sun,
  Trophy,
  X,
} from "lucide-react";
import { api } from "../api";
import { catalogPhoto } from "../catalogPhotos";
import type { CatalogItem } from "../CatalogArt";
import type { components } from "../generated/api";
import { useBase, ErrorLine } from "../app/shared";
import { Button, Field, Notice } from "../ui";
import { useUI } from "../store";
import "./Community.css";

type Post = components["schemas"]["CommunityPost"];
type Account = components["schemas"]["CommunityAccount"];
type Weekly = components["schemas"]["CommunityWeekly"];
type Candidate = components["schemas"]["CommunityCandidate"];
type Draft = components["schemas"]["CommunityDraft"];
const tags = ["家常", "快手", "早餐", "轻食", "汤羹", "烘焙"] as const;
const shortDate = (date: string) =>
  new Date(date).toLocaleDateString("zh-CN", {
    month: "numeric",
    day: "numeric",
    timeZone: "Asia/Shanghai",
  });
const reasonNames: Record<string, string> = {
  publish: "分享完整配方",
  like: "收到一次新的赞",
  selected: "配方收录食谱库",
  redeem: "兑换社区装饰",
};

function Modal({
  title,
  description,
  children,
  onClose,
  wide = false,
}: {
  title: string;
  description: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
}) {
  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="community-overlay" />
        <Dialog.Content
          className={`community-dialog ${wide ? "community-dialog-wide" : ""}`}
        >
          <div className="community-dialog-heading">
            <div>
              <Dialog.Title className="text-xl font-bold text-emerald-950">
                {title}
              </Dialog.Title>
              <Dialog.Description className="mt-1 text-sm leading-6 text-slate-500">
                {description}
              </Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <button aria-label="关闭窗口" className="community-icon-button">
                <X size={20} />
              </button>
            </Dialog.Close>
          </div>
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function PostPhoto({
  post,
  token,
  large = false,
}: {
  post: Post;
  token: string;
  large?: boolean;
}) {
  const [url, setURL] = useState("");
  useEffect(() => {
    setURL("");
    if (!post.has_cover) return;
    const controller = new AbortController();
    let objectURL = "";
    let stopped = false;
    fetch(`/api/community/posts/${post.id}/cover?v=${post.review_version}`, {
      headers: { Authorization: `Bearer ${token}` },
      signal: controller.signal,
    })
      .then((r) => {
        if (!r.ok) throw Error("photo unavailable");
        return r.blob();
      })
      .then((blob) => {
        if (!stopped) {
          objectURL = URL.createObjectURL(blob);
          setURL(objectURL);
        }
      })
      .catch(() => {});
    return () => {
      stopped = true;
      controller.abort();
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [post.has_cover, post.id, post.review_version, token]);
  const ingredient = post.ingredients[0]?.name;
  const photo = catalogPhoto(ingredient || "");
  const source = url || photo?.src;
  return (
    <div className={`community-photo ${large ? "community-photo-large" : ""}`}>
      {source ? (
        <img
          src={source}
          alt={url ? post.title : `${ingredient}食材示意`}
          loading="lazy"
        />
      ) : (
        <ChefHat size={64} strokeWidth={1.1} />
      )}
      <span className="community-photo-caption">
        {url ? "作者的厨房" : `食材示意${ingredient ? " · " + ingredient : ""}`}
      </span>
      {!!post.recipe_id && (
        <span className="community-featured-stamp">
          <Award size={14} /> 食光收录
        </span>
      )}
    </div>
  );
}
function Author({ post }: { post: Post }) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <span
        className={`community-avatar ${post.frame === "sunshine" ? "community-avatar-sun" : ""}`}
      >
        {post.author_name.slice(0, 1)}
      </span>
      <div className="min-w-0">
        <span className="block truncate text-sm font-semibold text-slate-700">
          {post.author_name}
        </span>
        <span className="flex items-center gap-1 text-xs text-slate-500">
          {post.badge === "seedling" ? (
            <>
              <Sprout size={11} /> 新芽创作者
            </>
          ) : (
            `分享于 ${shortDate(post.created_at)}`
          )}
        </span>
      </div>
    </div>
  );
}

export function Community() {
  const { token, q } = useBase();
  const [tab, setTab] = useState<"feed" | "weekly" | "shop">("feed");
  const [filter, setFilter] = useState("latest"),
    [search, setSearch] = useState(""),
    [tag, setTag] = useState(""),
    [offset, setOffset] = useState(0);
  const searchText = useDeferredValue(search);
  const [publishing, setPublishing] = useState(false),
    [editing, setEditing] = useState<Post | null>(null),
    [detail, setDetail] = useState<Post | null>(null),
    [review, setReview] = useState<Candidate | null>(null);
  const [busy, setBusy] = useState(""),
    [error, setError] = useState(""),
    [message, setMessage] = useState("");
  const account = useQuery({
    queryKey: ["community", "account", token],
    queryFn: () => api<Account>("/community/account", token),
  });
  const weekly = useQuery({
    queryKey: ["community", "weekly", token],
    queryFn: () => api<Weekly>("/community/weekly", token),
  });
  const feedPath = `/community/posts?filter=${filter}&q=${encodeURIComponent(searchText)}&tag=${encodeURIComponent(tag)}&offset=${offset}`;
  const feed = useQuery({
    queryKey: ["community", "feed", feedPath, token],
    queryFn: () => api<components["schemas"]["CommunityFeed"]>(feedPath, token),
    enabled: tab === "feed",
    refetchInterval: (query) =>
      query.state.data?.posts.some((post) =>
        ["pending", "reviewing"].includes(post.review_status),
      )
        ? 1500
        : false,
  });
  async function refresh() {
    await q.invalidateQueries({ queryKey: ["community"] });
  }
  async function action(
    key: string,
    task: () => Promise<unknown>,
    success = "",
  ) {
    if (busy) return;
    setBusy(key);
    setError("");
    setMessage("");
    try {
      await task();
      await refresh();
      setMessage(success);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy("");
    }
  }
  async function react(post: Post, kind: "like" | "save") {
    await action(post.id + kind, async () => {
      await api(
        `/community/posts/${post.id}/reactions/${kind}`,
        token,
        "POST",
        { active: kind === "like" ? !post.liked : !post.saved },
      );
      if (detail?.id === post.id)
        setDetail(await api<Post>(`/community/posts/${post.id}`, token));
    });
  }
  function changeFilter(next: string) {
    setFilter(next);
    setOffset(0);
  }
  return (
    <div
      className={`community-page ${account.data?.theme === "amber" ? "community-theme-amber" : ""}`}
    >
      <section className="community-hero" aria-labelledby="community-heading">
        <div className="community-hero-copy">
          <span className="community-eyebrow">
            <span /> THE COMMUNITY TABLE · 食光社区
          </span>
          <h1 id="community-heading">
            让你的拿手菜，
            <br />
            成为大家的<span>下一餐。</span>
          </h1>
          <p>
            分享厨房里的小发现，让认真做饭的人彼此相遇。
            <br className="hidden sm:block" />
            每一道好配方，都值得被看见。
          </p>
          <div className="flex flex-wrap items-center gap-3">
            <Button onClick={() => setPublishing(true)} size="lg">
              <Plus size={18} /> 分享我的配方
            </Button>
            <button
              className="community-text-button"
              onClick={() => setTab("weekly")}
            >
              看看每周精选 <ArrowRight size={16} />
            </button>
          </div>
          <div className="community-hero-note">
            <Leaf size={13} /> 一道配方，一份心意。好菜谱由你我一起收藏。
          </div>
        </div>
        <div className="community-hero-art" aria-hidden="true">
          <div className="community-orbit orbit-one" />
          <div className="community-orbit orbit-two" />
          <span className="community-floating-leaf">
            <Sprout size={28} />
          </span>
          <div className="community-recipe-paper">
            <span>FROM OUR KITCHENS</span>
            <div className="community-paper-photo">
              <img src={catalogPhoto("番茄")?.src} alt="" />
              <span className="community-paper-photo-dot">
                <Heart size={18} fill="currentColor" />
              </span>
            </div>
            <h2>
              日常的食材
              <br />
              也有不寻常的滋味
            </h2>
            <div className="community-paper-rule" />
            <small>新鲜灵感，正在发生</small>
          </div>
          <div className="community-art-stamp">
            <Trophy size={21} />
            <span>
              每周三道
              <br />
              <strong>发现好配方</strong>
            </span>
          </div>
        </div>
      </section>
      <div className="community-toolbar">
        <div className="community-tabs" role="tablist" aria-label="社区栏目">
          {(
            [
              ["feed", "厨房动态", ChefHat],
              ["weekly", "每周精选", Trophy],
              ["shop", "积分小铺", Sparkles],
            ] as const
          ).map(([id, label, Icon]) => (
            <button
              key={id}
              role="tab"
              id={`community-tab-${id}`}
              aria-controls={`community-panel-${id}`}
              tabIndex={tab === id ? 0 : -1}
              aria-selected={tab === id}
              onKeyDown={(event) => {
                const tabs = ["feed", "weekly", "shop"] as const;
                const position = tabs.indexOf(id);
                const next =
                  event.key === "ArrowRight"
                    ? tabs[(position + 1) % 3]
                    : event.key === "ArrowLeft"
                      ? tabs[(position + 2) % 3]
                      : event.key === "Home"
                        ? tabs[0]
                        : event.key === "End"
                          ? tabs[2]
                          : null;
                if (next) {
                  event.preventDefault();
                  setTab(next);
                  setError("");
                  document.getElementById(`community-tab-${next}`)?.focus();
                }
              }}
              onClick={() => {
                setTab(id);
                setError("");
              }}
              className={tab === id ? "active" : ""}
            >
              <Icon size={17} />
              {label}
            </button>
          ))}
        </div>
        <button
          className="community-balance-chip"
          onClick={() => setTab("shop")}
        >
          <Sprout size={15} />
          <strong>{account.data?.balance ?? "—"}</strong>
          <span className="community-balance-label">食光积分</span>
          <ArrowRight size={13} />
        </button>
      </div>
      {error && (
        <div className="mb-4">
          <Notice tone="error">{error}</Notice>
        </div>
      )}
      {message && (
        <div className="mb-4" role="status">
          <Notice tone="success">{message}</Notice>
        </div>
      )}
      <ErrorLine error={account.error} />
      {tab === "feed" && (
        <div
          className="community-layout"
          role="tabpanel"
          id="community-panel-feed"
          aria-labelledby="community-tab-feed"
        >
          <section aria-label="厨房动态" className="min-w-0">
            <div className="community-feed-heading">
              <div>
                <span className="community-eyebrow">FROM REAL KITCHENS</span>
                <h2>厨房里的新鲜灵感</h2>
              </div>
              <span>好配方，来自每一个认真做饭的人</span>
            </div>
            <div className="community-feed-controls">
              <div className="flex flex-wrap gap-1">
                {[
                  ["latest", "最新分享"],
                  ["popular", "大家喜欢"],
                  ["featured", "已收录"],
                  ["saved", "我的收藏"],
                  ["mine", "我的配方"],
                ].map(([id, label]) => (
                  <button
                    key={id}
                    aria-pressed={filter === id}
                    className={`community-filter ${filter === id ? "active" : ""}`}
                    onClick={() => changeFilter(id)}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <label className="community-search">
                <Search size={17} />
                <input
                  aria-label="搜索社区配方"
                  placeholder="寻找一道菜、一点灵感…"
                  maxLength={60}
                  value={search}
                  onChange={(e) => {
                    setSearch(e.target.value);
                    setOffset(0);
                  }}
                />
              </label>
            </div>
            <div className="community-tag-row">
              <button
                aria-pressed={!tag}
                onClick={() => {
                  setTag("");
                  setOffset(0);
                }}
              >
                全部口味
              </button>
              {tags.map((value) => (
                <button
                  key={value}
                  aria-pressed={tag === value}
                  onClick={() => {
                    setTag(tag === value ? "" : value);
                    setOffset(0);
                  }}
                >
                  {value}
                </button>
              ))}
            </div>
            {feed.isLoading ? (
              <div className="community-card-grid" aria-label="正在加载配方">
                {[0, 1, 2, 3].map((n) => (
                  <div className="community-skeleton" key={n}>
                    <div />
                    <span />
                    <span />
                  </div>
                ))}
              </div>
            ) : feed.error ? (
              <>
                <ErrorLine error={feed.error} />
                <Button
                  className="mt-3"
                  variant="outline"
                  onClick={() => feed.refetch()}
                >
                  重新加载
                </Button>
              </>
            ) : feed.data?.posts.length ? (
              <>
                <div className="community-card-grid">
                  {feed.data.posts.map((post) => (
                    <article className="community-post-card" key={post.id}>
                      <button
                        className="community-cover-button"
                        onClick={() => setDetail(post)}
                        aria-label={`查看配方：${post.title}`}
                      >
                        <PostPhoto post={post} token={token} />
                      </button>
                      <div className="community-post-body">
                        <PublicationReview post={post} />
                        <div className="community-post-tags">
                          {post.tags.map((t) => (
                            <span key={t}>{t}</span>
                          ))}
                          <span className="ml-auto">
                            <Clock3 size={12} /> {post.minutes} 分钟
                          </span>
                        </div>
                        <button
                          className="community-post-title"
                          onClick={() => setDetail(post)}
                        >
                          {post.title}
                        </button>
                        <p className="community-post-story">
                          {post.story || "把家里的味道，认真写成一份配方。"}
                        </p>
                        <div className="community-post-footer">
                          <Author post={post} />
                          <div className="flex gap-1">
                            <button
                              className={`community-reaction ${post.liked ? "is-liked" : ""}`}
                              aria-label={`${post.liked ? "取消点赞" : "点赞"}：${post.title}`}
                              aria-pressed={post.liked}
                              disabled={
                                !!busy ||
                                !publicPost(post) ||
                                post.author_id === account.data?.user_id
                              }
                              onClick={() => react(post, "like")}
                            >
                              <Heart
                                size={17}
                                fill={post.liked ? "currentColor" : "none"}
                              />
                              <span>{post.likes}</span>
                            </button>
                            <button
                              className={`community-reaction ${post.saved ? "is-saved" : ""}`}
                              aria-label={`${post.saved ? "取消收藏" : "收藏"}：${post.title}`}
                              aria-pressed={post.saved}
                              disabled={!!busy || !publicPost(post)}
                              onClick={() => react(post, "save")}
                            >
                              <Bookmark
                                size={17}
                                fill={post.saved ? "currentColor" : "none"}
                              />
                            </button>
                          </div>
                        </div>
                        {post.author_id === account.data?.user_id &&
                          editableSubmission(post) && (
                            <Button
                              variant="outline"
                              size="sm"
                              className="mt-3"
                              onClick={() => setEditing(post)}
                            >
                              修改并重新提交
                            </Button>
                          )}
                      </div>
                    </article>
                  ))}
                </div>
                <div className="mt-7 flex justify-center gap-3">
                  {offset > 0 && (
                    <Button
                      variant="outline"
                      onClick={() => setOffset(Math.max(0, offset - 20))}
                    >
                      上一页
                    </Button>
                  )}
                  {feed.data.has_more && (
                    <Button
                      variant="outline"
                      onClick={() => setOffset(offset + 20)}
                    >
                      下一页 <ArrowDown size={15} />
                    </Button>
                  )}
                </div>
              </>
            ) : (
              <Empty
                title={
                  search || tag
                    ? "还没有找到这道灵感"
                    : filter === "saved"
                      ? "给喜欢的配方留一个位置"
                      : "第一道好菜，等你来分享"
                }
                text={
                  search || tag
                    ? "换一个关键词，或者看看全部口味。"
                    : filter === "saved"
                      ? "点击菜谱上的书签，就能在这里再次找到它。"
                      : "写下食材与步骤，让你的日常成为别人的灵感。"
                }
                action={
                  filter === "saved" ? undefined : () => setPublishing(true)
                }
              />
            )}
          </section>
          <aside className="community-sidebar">
            <section className="community-week-preview">
              <span className="community-eyebrow">WEEKLY PICKS</span>
              <h2>
                <Trophy size={20} /> 这一周的好味道
              </h2>
              <p>每周三道候选，审核后收录食谱库</p>
              <div className="community-week-list">
                {weekly.data?.current.length ? (
                  weekly.data.current.map((post, i) => (
                    <button onClick={() => setDetail(post)} key={post.id}>
                      <span className="community-rank">0{i + 1}</span>
                      <span className="min-w-0 flex-1">
                        <strong>{post.title}</strong>
                        <small>{post.author_name}</small>
                      </span>
                      <span className="community-vote">
                        <Heart size={12} />
                        {post.weekly_likes}
                      </span>
                    </button>
                  ))
                ) : (
                  <div className="community-mini-empty">
                    <Sprout size={27} />
                    <span>
                      这一周的灵感正在萌芽
                      <br />
                      给喜欢的配方投出第一份心意
                    </span>
                  </div>
                )}
              </div>
              <button
                className="community-text-button"
                onClick={() => setTab("weekly")}
              >
                查看完整周榜 <ArrowRight size={14} />
              </button>
              <ErrorLine error={weekly.error} />
            </section>
            <section className="community-contribution-card">
              <span className="community-contribution-icon">
                <Leaf size={25} />
              </span>
              <h2>把认真，变成小小的奖励</h2>
              <p>
                分享、被喜欢、被收录。
                <br />
                每一份用心都会积累食光积分。
              </p>
              <div>
                <span>
                  完整配方 <strong>+5</strong>
                  <small>每天一次</small>
                </span>
                <span>
                  每周收录 <strong>+50</strong>
                  <small>每份配方一次</small>
                </span>
              </div>
              <button
                className="community-text-button"
                onClick={() => setTab("shop")}
              >
                逛逛积分小铺 <ArrowRight size={14} />
              </button>
            </section>
            <p className="community-side-note">
              慢慢做饭，认真分享。
              <br />
              我们一起把食谱库养得更丰富。
            </p>
          </aside>
        </div>
      )}
      {tab === "weekly" && (
        <section
          role="tabpanel"
          id="community-panel-weekly"
          aria-labelledby="community-tab-weekly"
        >
          <div className="community-section-heading">
            <div>
              <span className="community-eyebrow">GOOD FOOD, SHARED</span>
              <h2>一周的心意，三道好配方</h2>
              <p>
                以北京时间自然周统计有效点赞，每位作者最多占一个席位。入榜是候选，审核通过后才收录。
              </p>
            </div>
            <span className="community-week-date">
              {weekly.data
                ? `${shortDate(weekly.data.week_start)} — ${shortDate(new Date(new Date(weekly.data.week_end + "T00:00:00+08:00").getTime() - 86400000).toISOString())}`
                : "本周"}
            </span>
          </div>
          <ErrorLine error={weekly.error} />
          {weekly.isLoading ? (
            <p className="py-12 text-center">周榜加载中…</p>
          ) : weekly.data?.current.length ? (
            <div className="community-podium">
              {weekly.data.current.map((post, i) => (
                <button
                  className={`community-podium-card place-${i}`}
                  key={post.id}
                  onClick={() => setDetail(post)}
                >
                  <span className="community-podium-medal">
                    <Medal size={19} />
                    {i === 0 ? "本周人气" : `人气 0${i + 1}`}
                  </span>
                  <PostPhoto post={post} token={token} />
                  <div>
                    <h3>{post.title}</h3>
                    <p>
                      {post.author_name} · {post.minutes} 分钟
                    </p>
                    <span>
                      <Heart size={15} /> {post.weekly_likes} 份心意
                    </span>
                  </div>
                </button>
              ))}
            </div>
          ) : (
            <Empty
              title="本周榜单，正在等一份新灵感"
              text="分享配方，再为你喜欢的味道点赞。榜单只统计本周的有效点赞。"
              action={() => setPublishing(true)}
            />
          )}
          <div className="community-review-panel">
            <div className="community-section-heading">
              <div>
                <h2>上周候选 · 收录进度</h2>
                <p>
                  {weekly.data?.previous_week} 起的一周 ·
                  设备、用量和步骤经管理员确认后入库
                </p>
              </div>
              {account.data?.is_admin &&
                !weekly.data?.finalized &&
                weekly.data && (
                  <Button
                    disabled={!!busy}
                    variant="outline"
                    onClick={() =>
                      action(
                        "finalize",
                        () =>
                          api(
                            `/community/weeks/${weekly.data!.previous_week}/finalize`,
                            token,
                            "POST",
                          ),
                        "上周候选已生成",
                      )
                    }
                  >
                    生成上周候选
                  </Button>
                )}
            </div>
            {weekly.data?.candidates.length ? (
              <div className="space-y-3">
                {weekly.data.candidates.map((candidate) => (
                  <CandidateRow
                    key={candidate.id}
                    candidate={candidate}
                    token={token}
                    admin={!!account.data?.is_admin}
                    onView={setDetail}
                    onReview={() => setReview(candidate)}
                  />
                ))}
              </div>
            ) : (
              <p className="community-muted-box">
                {weekly.data?.finalized
                  ? "上周还没有符合条件的配方。新的一周，从你的一道拿手菜开始。"
                  : "上周榜单尚未结算。后台会每小时检查，并生成已经结束的自然周候选。"}
              </p>
            )}
          </div>
        </section>
      )}
      {tab === "shop" && (
        <section
          role="tabpanel"
          id="community-panel-shop"
          aria-labelledby="community-tab-shop"
        >
          <div className="community-shop-summary">
            <div>
              <span className="community-eyebrow">A LITTLE THANK YOU</span>
              <h2>每份用心，都值得一份小奖励。</h2>
              <p>用分享积累的积分，装饰你在食光的日常。</p>
              <div className="community-account-stats">
                <span>
                  <strong>{account.data?.balance ?? "—"}</strong>可用积分
                </span>
                <span>
                  <strong>{account.data?.published ?? "—"}</strong>分享配方
                </span>
                <span>
                  <strong>{account.data?.selected ?? "—"}</strong>食谱收录
                </span>
              </div>
            </div>
            <div className="community-shop-sprout" aria-hidden="true">
              <Sprout size={100} strokeWidth={1} />
              <span>一点一滴，生长成光</span>
            </div>
          </div>
          <div className="community-reward-grid">
            {account.data?.rewards.map((reward) => (
              <article
                className={`community-reward-card reward-${reward.id}`}
                key={reward.id}
              >
                <div className="community-reward-art">
                  {reward.kind === "badge" ? (
                    <span className="community-badge-art">
                      <Sprout size={40} />
                    </span>
                  ) : reward.kind === "frame" ? (
                    <span className="community-frame-art">
                      <Sun size={32} />
                    </span>
                  ) : (
                    <span className="community-theme-art">
                      <i />
                      <i />
                      <i />
                      <i />
                    </span>
                  )}
                  <span className="community-reward-kind">
                    {{
                      badge: "创作者徽章",
                      frame: "社区头像框",
                      theme: "专属主题",
                    }[reward.kind] || reward.kind}
                  </span>
                </div>
                <div className="p-5">
                  <h3>{reward.name}</h3>
                  <p>{reward.description}</p>
                  <div className="mt-6 flex items-center justify-between">
                    <span className="community-reward-price">
                      <Leaf size={15} />
                      {reward.cost} <small>积分</small>
                    </span>
                    <Button
                      size="sm"
                      variant={reward.owned ? "outline" : "default"}
                      disabled={
                        !!busy ||
                        reward.equipped ||
                        (!reward.owned &&
                          (account.data?.balance ?? 0) < reward.cost)
                      }
                      onClick={() => {
                        if (reward.owned)
                          action(
                            reward.id,
                            () =>
                              api(
                                `/community/rewards/${reward.id}/equip`,
                                token,
                                "POST",
                              ),
                            "装饰已启用",
                          );
                        else if (
                          window.confirm(
                            `使用 ${reward.cost} 积分兑换「${reward.name}」？`,
                          )
                        )
                          action(
                            reward.id,
                            () =>
                              api(
                                `/community/rewards/${reward.id}/redeem`,
                                token,
                                "POST",
                              ),
                            "兑换成功，装饰已启用",
                          );
                      }}
                    >
                      {reward.equipped ? (
                        <>
                          <Check size={14} />
                          使用中
                        </>
                      ) : reward.owned ? (
                        "使用装饰"
                      ) : (
                        "兑换"
                      )}
                    </Button>
                  </div>
                </div>
              </article>
            ))}
          </div>
          <div className="community-points-bottom">
            <section>
              <h3>积分怎么来？</h3>
              <p>
                <strong>+5</strong> 发布完整配方，每天最多奖励一次
              </p>
              <p>
                <strong>+1</strong> 收到不同用户的首次点赞，每天最多 20 分
              </p>
              <p>
                <strong>+50</strong> 配方审核收录，每道配方只奖励一次
              </p>
              <small>
                取消点赞后重新点赞不重复奖励。积分用于社区装饰，不兑换现金或实物。
              </small>
            </section>
            <section>
              <h3>最近的积分足迹</h3>
              {account.data?.ledger.length ? (
                account.data.ledger.slice(0, 8).map((entry, i) => (
                  <div className="community-ledger-row" key={i}>
                    <span>
                      {reasonNames[entry.reason] || entry.reason}
                      <small>{shortDate(entry.created_at)}</small>
                    </span>
                    <strong className={entry.amount > 0 ? "positive" : ""}>
                      {entry.amount > 0 ? "+" : ""}
                      {entry.amount}
                    </strong>
                  </div>
                ))
              ) : (
                <p className="text-slate-500">
                  还没有积分记录。你的第一份配方，就是开始。
                </p>
              )}
            </section>
          </div>
        </section>
      )}
      <footer className="community-footer">
        <Leaf size={14} /> 好好吃饭，也好好分享。
        <a
          href="/ingredient-photos/credits.html"
          target="_blank"
          rel="noreferrer"
        >
          食材示意图署名
        </a>
      </footer>
      {(publishing || editing) && (
        <PublishDialog
          initial={editing || undefined}
          token={token}
          onClose={() => {
            setPublishing(false);
            setEditing(null);
          }}
          onPublished={async () => {
            setPublishing(false);
            setEditing(null);
            setTab("feed");
            changeFilter("mine");
            setTag("");
            setSearch("");
            setMessage(
              "配方已提交初审，通过后会公开。你可以在「我的配方」查看进度和修改建议。",
            );
            await refresh();
          }}
        />
      )}
      {detail && (
        <Modal
          title={detail.title}
          description={`${detail.servings} 人份 · ${detail.minutes} 分钟 · 作者 ${detail.author_name}`}
          onClose={() => setDetail(null)}
          wide
        >
          <PostPhoto key={detail.id} post={detail} token={token} large />
          <div className="community-detail-content">
            <PublicationReview post={detail} />
            <Author post={detail} />
            {detail.story && (
              <p className="community-detail-story">{detail.story}</p>
            )}
            <div className="community-detail-grid">
              <section>
                <h3>
                  食材清单 <span>{detail.servings} 人份</span>
                </h3>
                {detail.ingredients.map((item) => (
                  <div
                    className="community-ingredient-line"
                    key={item.catalog_id}
                  >
                    <span>{item.name}</span>
                    <strong>
                      {Number(item.quantity)} <small>{item.unit}</small>
                    </strong>
                  </div>
                ))}
              </section>
              <section>
                <h3>把好味道做出来</h3>
                <ol className="community-step-list">
                  {detail.steps.map((step, i) => (
                    <li key={i}>
                      <span>{String(i + 1).padStart(2, "0")}</span>
                      <p>{step}</p>
                    </li>
                  ))}
                </ol>
              </section>
            </div>
            <div className="flex flex-wrap gap-2 border-t border-slate-100 pt-5">
              <Button
                variant="outline"
                disabled={
                  !!busy ||
                  !publicPost(detail) ||
                  detail.author_id === account.data?.user_id
                }
                onClick={() => react(detail, "like")}
              >
                <Heart size={16} />
                {detail.liked ? "取消点赞" : "喜欢这份配方"} · {detail.likes}
              </Button>
              <Button
                variant="outline"
                disabled={!!busy || !publicPost(detail)}
                onClick={() => react(detail, "save")}
              >
                <Bookmark size={16} />
                {detail.saved ? "取消收藏" : "收藏配方"}
              </Button>
              {detail.recipe_id && (
                <Button
                  onClick={() => {
                    setDetail(null);
                    useUI.getState().setPage("week");
                  }}
                >
                  去安排菜单 <ArrowRight size={15} />
                </Button>
              )}
              {(detail.author_id === account.data?.user_id ||
                account.data?.is_admin) && (
                <Button
                  className="ml-auto"
                  variant="ghost"
                  disabled={!!busy}
                  onClick={() => {
                    if (
                      window.confirm(
                        "确认撤下这条动态？已收录的食谱版本会继续保留，已获积分不重新发放。",
                      )
                    )
                      action(
                        "remove",
                        async () => {
                          await api(
                            `/community/posts/${detail.id}`,
                            token,
                            "DELETE",
                          );
                          setDetail(null);
                        },
                        "动态已撤下",
                      );
                  }}
                >
                  撤下动态
                </Button>
              )}
            </div>
            <p className="mt-3 text-xs leading-5 text-slate-400">
              配方由作者提供，份量与耗时为估计；请结合食材状态确认熟透。已收录食谱保留审核时的版本。
            </p>
          </div>
          <ErrorLine error={error} />
        </Modal>
      )}
      {review && (
        <ReviewDialog
          candidate={review}
          token={token}
          onClose={() => setReview(null)}
          onReviewed={async () => {
            setReview(null);
            setMessage("审核结果已保存");
            await refresh();
            await q.invalidateQueries({ queryKey: ["recipes"] });
          }}
        />
      )}
    </div>
  );
}

function Empty({
  title,
  text,
  action,
}: {
  title: string;
  text: string;
  action?: () => void;
}) {
  return (
    <div className="community-empty">
      <div className="community-empty-art">
        <span />
        <ChefHat size={42} strokeWidth={1.3} />
        <Leaf size={20} />
      </div>
      <h2>{title}</h2>
      <p>{text}</p>
      {action && (
        <Button variant="outline" onClick={action}>
          <Plus size={16} /> 分享第一份心意
        </Button>
      )}
    </div>
  );
}

function CandidateRow({
  candidate,
  token,
  admin,
  onView,
  onReview,
}: {
  candidate: Candidate;
  token: string;
  admin: boolean;
  onView: (post: Post) => void;
  onReview: () => void;
}) {
  const post = useQuery({
    queryKey: ["community", "candidate", candidate.post_id, token],
    queryFn: () => api<Post>(`/community/posts/${candidate.post_id}`, token),
  });
  return (
    <div className="community-candidate-row">
      <span className="community-rank">0{candidate.rank}</span>
      <div className="min-w-0 flex-1">
        <button
          className="text-left font-semibold text-emerald-950"
          disabled={!post.data}
          onClick={() => post.data && onView(post.data)}
        >
          {post.data?.title ||
            (post.error ? "动态已撤下或暂不可用" : "正在加载…")}
        </button>
        <p className="mt-1 text-xs text-slate-500">
          {post.data?.author_name} · {candidate.votes} 个有效赞
        </p>
        {candidate.review_note && (
          <p className="mt-1 text-xs text-slate-500">
            审核说明：{candidate.review_note}
          </p>
        )}
      </div>
      <span className={`community-status status-${candidate.status}`}>
        {
          { pending: "待审核", accepted: "已收录", rejected: "未收录" }[
            candidate.status
          ]
        }
      </span>
      {admin && candidate.status === "pending" && (
        <Button size="sm" variant="outline" onClick={onReview}>
          审核配方
        </Button>
      )}
    </div>
  );
}

async function prepareCover(file: File): Promise<string> {
  if (
    !["image/jpeg", "image/png"].includes(file.type) ||
    file.size > 10 * 1024 * 1024
  )
    throw Error("请选择 10MB 以内的 JPG 或 PNG");
  const bitmap = await createImageBitmap(file);
  try {
    const scale = Math.min(1, 1400 / Math.max(bitmap.width, bitmap.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.round(bitmap.width * scale);
    canvas.height = Math.round(bitmap.height * scale);
    const ctx = canvas.getContext("2d");
    if (!ctx) throw Error("图片处理不可用");
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
    const encoded = canvas.toDataURL("image/jpeg", 0.82).split(",")[1];
    if (encoded.length > (2 * 1024 * 1024 * 4) / 3)
      throw Error("图片仍然太大，请使用更小的照片");
    return encoded;
  } finally {
    bitmap.close();
  }
}
const publicPost = (post: Post) =>
  ["approved", "legacy"].includes(post.review_status);
const editableSubmission = (post: Post) =>
  ["pending", "rejected", "unavailable"].includes(post.review_status);
function PublicationReview({ post }: { post: Post }) {
  if (post.review_status === "legacy") return null;
  const labels: Record<string, string> = {
    pending: "待初审",
    reviewing: "初审中",
    approved: "初审通过",
    rejected: "需要修改",
    unavailable: "审核暂不可用",
  };
  return (
    <div
      className={`community-publication-review review-${post.review_status}`}
      role="status"
      aria-label="发布初审状态"
    >
      <strong>
        {post.review_status === "approved" ? (
          <Check size={15} />
        ) : (
          <Clock3 size={15} />
        )}
        {labels[post.review_status]}
      </strong>
      <p>
        {post.review_note ||
          "配方与上传的封面会一起检查，通过后公开。审核期间仅自己可见。"}
      </p>
    </div>
  );
}
function PublishDialog({
  token,
  initial,
  onClose,
  onPublished,
}: {
  token: string;
  initial?: Post;
  onClose: () => void;
  onPublished: () => Promise<void>;
}) {
  const catalogue = useQuery({
    queryKey: ["ingredient-catalog"],
    queryFn: () => api<CatalogItem[]>("/ingredient-catalog", token),
  });
  const [draft, setDraft] = useState<Draft>({
    title: initial?.title || "",
    story: initial?.story || "",
    servings: initial?.servings || 2,
    minutes: initial?.minutes || 20,
    tags: initial?.tags.filter((tag): tag is (typeof tags)[number] =>
      tags.some((allowed) => allowed === tag),
    ) || ["家常"],
    ingredients: initial?.ingredients.map(({ catalog_id, quantity, unit }) => ({
      catalog_id,
      quantity,
      unit,
    })) || [{ catalog_id: "", quantity: "", unit: "g" }],
    steps: initial?.steps || [""],
    confirm_public: false,
    confirm_review: false,
  });
  const [cover, setCover] = useState(""),
    [removeCover, setRemoveCover] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [processing, setProcessing] = useState(false);
  const request = useRef({ body: "", key: "" });
  async function publish(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    const body = { ...draft, cover_base64: cover, remove_cover: removeCover };
    const signature = JSON.stringify(body);
    if (request.current.body !== signature)
      request.current = { body: signature, key: crypto.randomUUID() };
    try {
      await api(
        initial ? `/community/posts/${initial.id}` : "/community/posts",
        token,
        initial ? "PATCH" : "POST",
        body,
        request.current.key,
      );
      await onPublished();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title={initial ? "修改这份配方" : "分享一道拿手菜"}
      description="提交后先检查配方和封面，通过后公开并发放发布积分。每天最多提交两道新配方。"
      onClose={busy || processing ? () => {} : onClose}
      wide
    >
      <form className="community-publish-form" onSubmit={publish}>
        <Notice>
          智能初审会检查配方完整性、明显危险做法、广告及不适合公开的文字和图片。初审不保证食品安全；每周收录仍需管理员确认。
        </Notice>
        {initial?.review_note && <Notice>{initial.review_note}</Notice>}
        <label className={`community-upload ${cover ? "has-cover" : ""}`}>
          {cover ? (
            <img src={`data:image/jpeg;base64,${cover}`} alt="待发布的封面" />
          ) : initial?.has_cover && !removeCover ? (
            <PostPhoto post={initial} token={token} />
          ) : (
            <>
              <ImagePlus size={30} strokeWidth={1.3} />
              <strong>
                {processing ? "正在整理照片…" : "放一张你厨房里的照片"}
              </strong>
              <span>可选 · JPG / PNG · 自动缩小并去除照片元信息</span>
            </>
          )}
          <input
            aria-label="上传菜谱封面"
            type="file"
            accept="image/jpeg,image/png"
            disabled={busy || processing}
            onChange={async (e) => {
              const file = e.target.files?.[0];
              if (!file) return;
              setProcessing(true);
              setError("");
              try {
                setCover(await prepareCover(file));
                setRemoveCover(false);
              } catch (err) {
                setError(String(err));
              } finally {
                setProcessing(false);
                e.target.value = "";
              }
            }}
          />
          {cover && (
            <span className="community-upload-change">点击更换封面</span>
          )}
        </label>
        {(cover || (initial?.has_cover && !removeCover)) && (
          <button
            type="button"
            className="community-text-button"
            onClick={() => {
              setCover("");
              setRemoveCover(true);
            }}
          >
            移除封面
          </button>
        )}
        <Field
          label="给这道菜起个名字"
          placeholder="例如：下班后的一碗番茄鸡蛋面"
          required
          minLength={2}
          maxLength={60}
          value={draft.title}
          onChange={(e) => setDraft({ ...draft, title: e.target.value })}
        />
        <label className="community-form-label">
          厨房里的小故事
          <textarea
            placeholder="为什么喜欢这道菜？有什么小秘诀？"
            maxLength={1000}
            rows={3}
            value={draft.story}
            onChange={(e) => setDraft({ ...draft, story: e.target.value })}
          />
        </label>
        <div className="grid grid-cols-2 gap-4">
          <Field
            label="几人份"
            type="number"
            min={1}
            max={20}
            required
            value={draft.servings}
            onChange={(e) =>
              setDraft({ ...draft, servings: Number(e.target.value) })
            }
          />
          <Field
            label="预计耗时（分钟）"
            type="number"
            min={1}
            max={240}
            required
            value={draft.minutes}
            onChange={(e) =>
              setDraft({ ...draft, minutes: Number(e.target.value) })
            }
          />
        </div>
        <fieldset>
          <legend className="community-form-label">
            给配方贴个标签 <small>最多选 3 个</small>
          </legend>
          <div className="community-tag-row">
            {tags.map((tag) => (
              <button
                type="button"
                key={tag}
                aria-pressed={draft.tags.includes(tag)}
                onClick={() =>
                  setDraft({
                    ...draft,
                    tags: draft.tags.includes(tag)
                      ? draft.tags.filter((t) => t !== tag)
                      : draft.tags.length < 3
                        ? [...draft.tags, tag]
                        : draft.tags,
                  })
                }
              >
                {tag}
              </button>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend className="community-form-label">
            食材与用量 <small>使用食材目录，方便日后安排菜单</small>
          </legend>
          <ErrorLine error={catalogue.error} />
          <div className="mt-3 space-y-3">
            {draft.ingredients.map((item, i) => (
              <div className="community-ingredient-editor" key={i}>
                <select
                  aria-label={`食材 ${i + 1}`}
                  required
                  value={item.catalog_id}
                  onChange={(e) => {
                    const chosen = catalogue.data?.find(
                      (v) => v.id === e.target.value,
                    );
                    setDraft({
                      ...draft,
                      ingredients: draft.ingredients.map((v, n) =>
                        n === i
                          ? {
                              ...v,
                              catalog_id: e.target.value,
                              unit: chosen?.default_unit || "g",
                            }
                          : v,
                      ),
                    });
                  }}
                >
                  <option value="">选择食材</option>
                  {catalogue.data?.map((v) => (
                    <option value={v.id} key={v.id}>
                      {v.name}
                    </option>
                  ))}
                </select>
                <input
                  aria-label={`用量 ${i + 1}`}
                  required
                  inputMode="decimal"
                  placeholder="例如 200"
                  pattern="[0-9]+(\.[0-9]{1,3})?"
                  value={item.quantity}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      ingredients: draft.ingredients.map((v, n) =>
                        n === i ? { ...v, quantity: e.target.value } : v,
                      ),
                    })
                  }
                />
                <span>{item.unit}</span>
                <button
                  type="button"
                  aria-label={`移除食材 ${i + 1}`}
                  disabled={draft.ingredients.length === 1}
                  onClick={() =>
                    setDraft({
                      ...draft,
                      ingredients: draft.ingredients.filter((_, n) => n !== i),
                    })
                  }
                >
                  <X size={16} />
                </button>
              </div>
            ))}
          </div>
          <Button
            className="mt-3"
            type="button"
            variant="ghost"
            disabled={draft.ingredients.length >= 30}
            onClick={() =>
              setDraft({
                ...draft,
                ingredients: [
                  ...draft.ingredients,
                  { catalog_id: "", quantity: "", unit: "g" },
                ],
              })
            }
          >
            <Plus size={15} />
            再加一种食材
          </Button>
        </fieldset>
        <fieldset>
          <legend className="community-form-label">
            把好味道做出来 <small>步骤清楚一点，第一次做也不慌</small>
          </legend>
          <div className="mt-3 space-y-3">
            {draft.steps.map((step, i) => (
              <div className="community-step-editor" key={i}>
                <span>{String(i + 1).padStart(2, "0")}</span>
                <textarea
                  aria-label={`步骤 ${i + 1}`}
                  required
                  maxLength={500}
                  rows={2}
                  placeholder="写下这一步的做法与需要留意的细节"
                  value={step}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      steps: draft.steps.map((v, n) =>
                        n === i ? e.target.value : v,
                      ),
                    })
                  }
                />
                <button
                  type="button"
                  aria-label={`移除步骤 ${i + 1}`}
                  disabled={draft.steps.length === 1}
                  onClick={() =>
                    setDraft({
                      ...draft,
                      steps: draft.steps.filter((_, n) => n !== i),
                    })
                  }
                >
                  <X size={16} />
                </button>
              </div>
            ))}
          </div>
          <Button
            type="button"
            variant="ghost"
            className="mt-3"
            disabled={draft.steps.length >= 20}
            onClick={() => setDraft({ ...draft, steps: [...draft.steps, ""] })}
          >
            <Plus size={15} />
            添加下一步
          </Button>
        </fieldset>
        <label className="community-confirm">
          <input
            type="checkbox"
            required
            checked={draft.confirm_public}
            onChange={(e) =>
              setDraft({
                ...draft,
                confirm_public: e.target.checked,
                confirm_review: e.target.checked,
              })
            }
          />
          <span>
            确认公开分享这份配方、照片及我的昵称；同意将配方文字与封面发送至审核模型进行初审，通过后公开，并同意管理员审核后署名收录食谱库。
          </span>
        </label>
        <ErrorLine error={error} />
        <div className="community-form-footer">
          <span>
            <Sprout size={14} />
            每天首份初审通过的配方奖励 5 积分
          </span>
          <Button
            type="submit"
            disabled={
              busy ||
              processing ||
              !draft.confirm_public ||
              !draft.confirm_review ||
              catalogue.isLoading ||
              !!catalogue.error
            }
          >
            {busy ? "正在提交…" : initial ? "重新提交初审" : "发布这份心意"}
            <ArrowRight size={15} />
          </Button>
        </div>
      </form>
    </Modal>
  );
}
function ReviewDialog({
  candidate,
  token,
  onClose,
  onReviewed,
}: {
  candidate: Candidate;
  token: string;
  onClose: () => void;
  onReviewed: () => Promise<void>;
}) {
  const post = useQuery({
    queryKey: ["community", "review", candidate.post_id, token],
    queryFn: () => api<Post>(`/community/posts/${candidate.post_id}`, token),
  });
  const [accept, setAccept] = useState(true),
    [note, setNote] = useState(""),
    [confirm, setConfirm] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [workflow, setWorkflow] = useState<
    { equipment: string; duration_seconds: number }[]
  >([]);
  useEffect(() => {
    if (post.data)
      setWorkflow(
        post.data.steps.map(() => ({
          equipment: "board",
          duration_seconds: 60,
        })),
      );
  }, [post.data]);
  async function submit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      await api(`/community/candidates/${candidate.id}/review`, token, "POST", {
        accept,
        note,
        confirm,
        workflow,
      });
      await onReviewed();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title="审核社区配方"
      description="收录会创建固定版本食谱，并一次性奖励作者 50 积分。请核对配方，确认每步设备与预计耗时。"
      onClose={busy ? () => {} : onClose}
      wide
    >
      <form className="community-publish-form" onSubmit={submit}>
        <ErrorLine error={post.error} />
        {post.data && (
          <>
            <h3 className="text-lg font-semibold">
              {post.data.title} · {post.data.servings} 人份
            </h3>
            <p className="text-sm text-slate-500">
              作者：{post.data.author_name} · {post.data.minutes} 分钟
            </p>
            <div className="community-muted-box">
              {post.data.ingredients
                .map((i) => `${i.name} ${Number(i.quantity)} ${i.unit}`)
                .join(" · ")}
            </div>
            <label className="community-form-label">
              审核结果
              <select
                value={accept ? "accept" : "reject"}
                onChange={(e) => setAccept(e.target.value === "accept")}
              >
                <option value="accept">收录到食谱库</option>
                <option value="reject">暂不收录</option>
              </select>
            </label>
            {accept &&
              post.data.steps.map((step, i) => (
                <div className="community-review-step" key={i}>
                  <p>
                    {i + 1}. {step}
                  </p>
                  <div className="grid grid-cols-2 gap-3">
                    <label className="community-form-label">
                      设备
                      <select
                        aria-label={`审核步骤 ${i + 1} 设备`}
                        value={workflow[i]?.equipment || "board"}
                        onChange={(e) =>
                          setWorkflow(
                            workflow.map((v, n) =>
                              n === i ? { ...v, equipment: e.target.value } : v,
                            ),
                          )
                        }
                      >
                        <option value="board">操作台 / 案板</option>
                        <option value="stove">灶台</option>
                      </select>
                    </label>
                    <Field
                      label={`审核步骤 ${i + 1} 秒数`}
                      type="number"
                      required
                      min={1}
                      max={14400}
                      value={workflow[i]?.duration_seconds || 60}
                      onChange={(e) =>
                        setWorkflow(
                          workflow.map((v, n) =>
                            n === i
                              ? {
                                  ...v,
                                  duration_seconds: Number(e.target.value),
                                }
                              : v,
                          ),
                        )
                      }
                    />
                  </div>
                </div>
              ))}
            <p className="text-xs text-slate-500">
              当前厨房专注模式仅支持操作台与灶台；需烤箱等设备的配方可暂不收录，并注明原因。
            </p>
          </>
        )}
        <label className="community-form-label">
          审核说明
          <textarea
            rows={3}
            maxLength={300}
            required={!accept}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="未收录时，给作者一条具体的改进建议"
          />
        </label>
        <label className="community-confirm">
          <input
            type="checkbox"
            required
            checked={confirm}
            onChange={(e) => setConfirm(e.target.checked)}
          />
          <span>
            我已核对食材用量、步骤、设备及预计耗时，确认本次审核结果。
          </span>
        </label>
        <ErrorLine error={error} />
        <Button
          type="submit"
          disabled={
            busy || !confirm || post.isLoading || (accept && !post.data)
          }
        >
          {busy ? "正在保存…" : "确认审核结果"}
        </Button>
      </form>
    </Modal>
  );
}
