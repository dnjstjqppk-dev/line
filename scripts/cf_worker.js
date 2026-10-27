/**
 * Cloudflare Worker Edge Gateway & Backend for LINE Messenger Monorepo.
 * Serves static assets for frontend apps and handles full /api/v1 suite directly at the edge.
 * Supports native Cloudflare WebSocketPair real-time broadcasting and multi-console workflows.
 */

// In-memory edge state for the Worker isolate
const users = new Map();
const sessions = new Map();
const qrChallenges = new Map();
const friendships = new Map();
const conversations = new Map();
const messages = new Map();
const callSessions = new Map();
const storeOrders = new Map();
const pinnedMessages = new Map();
const polls = new Map();
const pollVotes = new Map();
const albums = new Map();
const savedMessages = new Map();
const communityRooms = new Map();
const communityPosts = new Map();
const partnerOrgs = new Map();
const partnerAccounts = new Map();
const partnerTickets = new Map();
const partnerCampaigns = new Map();
const devOrgs = new Map();
const devApps = new Map();
const devWebhooks = new Map();
const devMiniApps = new Map();
const creatorProfiles = new Map();
const creatorPacks = new Map();
const creatorItems = new Map();
const adsAdvertisers = new Map();
const adsCampaigns = new Map();
const adsGroups = new Map();
const auditLogs = [];
const activeSockets = new Map();
const wsTickets = new Map();

// Extended messenger state (friends meta, chat meta, reactions, timeline, wallet, shop ...)
const usersById = new Map();
const userPrefs = new Map();
const friendMeta = new Map();
const chatMeta = new Map();
const readCursors = new Map();
const hiddenMsgs = new Map();
const reactionStore = new Map();
const noteStore = new Map();
const eventStore = new Map();
const timelinePosts = [];
const voomPosts = [];
const storyStore = [];
const wallets = new Map();
const ownedPacks = new Map();
const ownedThemes = new Map();
const officialAccounts = new Map();
const followedOfficial = new Map();
const roomMembers = new Map();
const attachmentStore = new Map();
const adminReports = [];
const MAX_ATTACHMENT_B64 = 2_000_000;
const MAX_BODY = 20000;

const STICKER_CATALOG = [
  { id: 'basic', title: '기본 이모티콘', price_minor: 0, stickers: ['😀', '😂', '😍', '😭', '😡', '👍', '🙏', '🎉', '😴', '🤔', '🥳', '😎'] },
  { id: 'pack-1', title: '포근한 동물 친구들', price_minor: 2500, stickers: ['🐻', '🐰', '🐥', '🐱', '🐶', '🦊', '🐼', '🐨', '🐸', '🦁', '🐯', '🐷'] },
  { id: 'pack-2', title: '맛있는 하루', price_minor: 2500, stickers: ['🍕', '🍔', '🍣', '🍜', '🍰', '☕', '🍺', '🍓', '🍙', '🌮', '🍩', '🥐'] },
  { id: 'pack-3', title: '우주 탐험대', price_minor: 3500, stickers: ['🚀', '🌙', '⭐', '🪐', '👽', '🛸', '☄️', '🌍', '🔭', '🌌', '🌠', '🧑‍🚀'] }
];
const THEME_CATALOG = [
  { id: 'mint', title: '민트 브리즈', price_minor: 0 },
  { id: 'ocean', title: '오션 블루', price_minor: 0 },
  { id: 'sunset', title: '선셋 글로우', price_minor: 1500 },
  { id: 'sakura', title: '사쿠라 핑크', price_minor: 1500 },
  { id: 'midnight', title: '미드나잇', price_minor: 0 }
];
const DEFAULT_PREFS = {
  theme: 'mint', dark_mode: false, language: 'ko', font_size: 'medium',
  show_read_receipts: true, searchable_by_id: true, searchable_by_email: false,
  allow_friend_requests: true, notifications: true, notification_preview: true,
  letter_sealing: true, wallpaper: 'default'
};

function now() { return new Date().toISOString(); }
function uid() { return crypto.randomUUID(); }

async function pbkdf2(password, saltHex) {
  try {
    const salt = Uint8Array.from(saltHex.match(/../g).map(h => parseInt(h, 16)));
    const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(password), 'PBKDF2', false, ['deriveBits']);
    const bits = await crypto.subtle.deriveBits({ name: 'PBKDF2', hash: 'SHA-256', salt, iterations: 1000 }, key, 256);
    return Array.from(new Uint8Array(bits)).map(b => b.toString(16).padStart(2, '0')).join('');
  } catch {
    return null;
  }
}
async function setPassword(user, password) {
  user.raw_password = password;
  user.password = password;
  try {
    const salt = Array.from(crypto.getRandomValues(new Uint8Array(16))).map(b => b.toString(16).padStart(2, '0')).join('');
    user.pw_salt = salt;
    user.pw_hash = await pbkdf2(password, salt);
  } catch {
    // fallback
  }
}
async function checkPassword(user, password) {
  if (!user || typeof password !== 'string') return false;
  if (user.password && user.password === password) return true;
  if (user.raw_password && user.raw_password === password) return true;
  if (user.pw_hash && user.pw_salt) {
    const hash = await pbkdf2(password, user.pw_salt);
    if (hash && hash === user.pw_hash) return true;
  }
  return false;
}
async function hashPendingPasswords() {
  for (const u of users.values()) {
    if (u.password && !u.pw_hash) {
      await setPassword(u, u.password);
    }
  }
}

function publicUser(u) {
  if (!u) return null;
  const o = officialAccounts.get(u);
  if (o) return { id: o.id, display_name: o.name, handle: o.handle, bio: o.description, official: true, avatar_color: o.color };
  return {
    id: u.id, display_name: u.display_name, handle: u.handle, bio: u.bio || '',
    status_message: u.status_message || '', avatar_color: u.avatar_color || '#06c755',
    birthday: u.birthday || '', avatar_url: u.avatar_url || ''
  };
}
function userBrief(id) {
  if (officialAccounts.has(id)) return publicUser(id);
  return publicUser(usersById.get(id));
}
function prefsOf(userId) {
  if (!userPrefs.has(userId)) userPrefs.set(userId, { ...DEFAULT_PREFS });
  return userPrefs.get(userId);
}
function walletOf(userId) {
  if (!wallets.has(userId)) wallets.set(userId, { balance: 50000, tx: [] });
  return wallets.get(userId);
}
function packsOf(userId) {
  if (!ownedPacks.has(userId)) ownedPacks.set(userId, new Set(['basic']));
  return ownedPacks.get(userId);
}
function themesOf(userId) {
  if (!ownedThemes.has(userId)) ownedThemes.set(userId, new Set(THEME_CATALOG.filter(t => t.price_minor === 0).map(t => t.id)));
  return ownedThemes.get(userId);
}
function fMeta(me, friend) {
  const k = `${me}:${friend}`;
  if (!friendMeta.has(k)) friendMeta.set(k, { favorite: false, hidden: false, blocked: false, nickname: '' });
  return friendMeta.get(k);
}
function cMeta(cid, userId) {
  const k = `${cid}:${userId}`;
  if (!chatMeta.has(k)) chatMeta.set(k, { pinned: false, muted: false, hidden: false });
  return chatMeta.get(k);
}
function isFriend(a, b) { return friendships.has(`${a}:${b}`); }
function friendIds(userId) {
  const out = [];
  for (const key of friendships.keys()) {
    const [a, b] = key.split(':');
    if (a === userId) out.push(b);
  }
  return out;
}
function reactionCounts(mid) {
  const r = reactionStore.get(mid);
  const out = {};
  if (r) for (const [emoji, set] of r) if (set.size) out[emoji] = set.size;
  return out;
}
function decorateMessage(msg, viewerId, cid) {
  const conv = conversations.get(cid);
  const cursorSeqs = (conv ? conv.members : []).filter(m => m !== msg.sender_id && !officialAccounts.has(m))
    .map(m => readCursors.get(`${cid}:${m}`) || 0);
  const out = { ...msg, reactions: reactionCounts(msg.id), read_count: cursorSeqs.filter(s => s >= msg.sequence).length };
  const mine = [];
  const r = reactionStore.get(msg.id);
  if (r) for (const [emoji, set] of r) if (set.has(viewerId)) mine.push(emoji);
  out.my_reactions = mine;
  if (msg.reply_to) {
    const orig = (messages.get(cid) || []).find(m => m.id === msg.reply_to);
    if (orig) out.reply_preview = { sender_id: orig.sender_id, kind: orig.kind, body: orig.unsent ? '' : String(orig.body).slice(0, 80), unsent: !!orig.unsent };
  }
  return out;
}
function unreadFor(cid, userId) {
  const cursor = readCursors.get(`${cid}:${userId}`) || 0;
  return (messages.get(cid) || []).filter(m => m.sequence > cursor && m.sender_id !== userId && !m.unsent && m.kind !== 'system').length;
}
function mentionedIn(cid, userId) {
  const cursor = readCursors.get(`${cid}:${userId}`) || 0;
  return (messages.get(cid) || []).some(m => m.sequence > cursor && m.sender_id !== userId && !m.unsent &&
    (m.metadata?.mentions || []).some(x => x === userId || x === 'all'));
}
function pushMessage(cid, senderId, fields) {
  const conv = conversations.get(cid);
  conv.last_sequence = (conv.last_sequence || 0) + 1;
  conv.updated_at = now();
  const msg = {
    id: uid(), conversation_id: cid, sender_id: senderId, client_id: fields.client_id || uid(),
    sequence: conv.last_sequence, kind: fields.kind || 'text', body: fields.body ?? '',
    reply_to: fields.reply_to || undefined, metadata: fields.metadata || undefined, created_at: now()
  };
  const list = messages.get(cid) || [];
  list.push(msg);
  messages.set(cid, list);
  if (!officialAccounts.has(senderId)) readCursors.set(`${cid}:${senderId}`, msg.sequence);
  broadcast({ type: 'message.created', conversation_id: cid, message: msg });
  return msg;
}
function systemMessage(cid, body) {
  return pushMessage(cid, 'system', { kind: 'system', body });
}
function chatSummary(conv, viewerId) {
  const list = messages.get(conv.id) || [];
  const last = list.filter(m => !(hiddenMsgs.get(viewerId)?.has(m.id))).at(-1);
  const meta = cMeta(conv.id, viewerId);
  const memberBriefs = conv.members.map(userBrief).filter(Boolean);
  let title = conv.title;
  if (conv.kind === 'direct') {
    const other = conv.members.find(m => m !== viewerId);
    const o = userBrief(other);
    if (o) title = fMeta(viewerId, other).nickname || o.display_name;
  }
  return {
    id: conv.id, kind: conv.kind, title, last_sequence: conv.last_sequence || 0,
    unread_count: unreadFor(conv.id, viewerId), mentioned: mentionedIn(conv.id, viewerId),
    updated_at: conv.updated_at, owner_id: conv.owner_id || null, notice: conv.notice || '',
    pinned: meta.pinned, muted: meta.muted, hidden: meta.hidden,
    member_count: conv.members.length, members: memberBriefs, official: !!conv.official,
    last_message: last ? { id: last.id, kind: last.kind, body: last.unsent ? '' : String(last.body).slice(0, 120), unsent: !!last.unsent, sender_id: last.sender_id, created_at: last.created_at } : null
  };
}
function botReply(text, account) {
  const t = String(text || '').toLowerCase();
  if (/안녕|hello|hi\b/.test(t)) return `안녕하세요! ${account.name} 공식계정입니다. 👋 '도움말'이라고 입력해 보세요.`;
  if (/도움|help/.test(t)) return '사용 가능한 명령: 안녕, 날씨, 쿠폰, 공지, 도움말';
  if (/날씨|weather/.test(t)) return '오늘 서울은 맑음 ☀️ 최고 21°C / 최저 12°C 입니다.';
  if (/쿠폰|coupon/.test(t)) return '🎟️ 신규 친구 10% 할인 쿠폰: WELCOME10';
  if (/공지|notice/.test(t)) return '📢 이번 주 업데이트: 타임라인·VOOM·Pay 기능이 추가되었습니다.';
  return '메시지를 받았습니다. 자세한 안내는 "도움말"을 입력해 주세요.';
}
function seedExtras() {
  const alice = users.get('alice@line-test.local');
  const bob = users.get('bob@line-test.local');
  const demo = users.get('user@line.local');
  const colors = { 'admin@line.local': '#6366f1', 'user@line.local': '#06c755', 'alice@line-test.local': '#f97316', 'bob@line-test.local': '#0ea5e9' };
  const status = { 'admin@line.local': '운영 중 🛠️', 'user@line.local': '오늘도 좋은 하루 ☀️', 'alice@line-test.local': '커피 한 잔의 여유 ☕', 'bob@line-test.local': '코딩 중... 💻' };
  for (const [email, u] of users) {
    u.avatar_color = colors[email] || '#06c755';
    u.status_message = status[email] || '';
    usersById.set(u.id, u);
  }
  friendships.set(`${demo.id}:${alice.id}`, true);
  friendships.set(`${alice.id}:${demo.id}`, true);
  friendships.set(`${demo.id}:${bob.id}`, true);
  friendships.set(`${bob.id}:${demo.id}`, true);
  alice.birthday = new Date().toISOString().slice(5, 10);

  for (const [id, name, handle, description, color] of [
    ['oa-notice', 'LINE 공식 공지', '@line_notice', '서비스 소식과 업데이트를 전해드립니다.', '#06c755'],
    ['oa-weather', '날씨 알리미', '@weather', '매일 아침 날씨를 알려드립니다.', '#0ea5e9'],
    ['oa-deals', '오늘의 특가', '@deals', '친구 전용 쿠폰과 특가 소식', '#f43f5e']
  ]) officialAccounts.set(id, { id, name, handle, description, color });

  const bg = ['linear-gradient(135deg,#06c755,#0ea5e9)', 'linear-gradient(135deg,#f97316,#f43f5e)', 'linear-gradient(135deg,#6366f1,#ec4899)', 'linear-gradient(135deg,#14b8a6,#84cc16)'];
  timelinePosts.push(
    { id: uid(), author_id: alice.id, body: '오늘 한강 노을이 너무 예뻐요 🌇', bg: bg[1], visibility: 'friends', created_at: now(), likes: [bob.id], comments: [] },
    { id: uid(), author_id: bob.id, body: '새 프로젝트 배포 완료! 🚀', bg: bg[0], visibility: 'friends', created_at: now(), likes: [], comments: [] }
  );
  storyStore.push(
    { id: uid(), author_id: alice.id, body: '☕ 카페 투어', bg: bg[1], created_at: now(), viewers: [] },
    { id: uid(), author_id: bob.id, body: '🏃 러닝 5km', bg: bg[3], created_at: now(), viewers: [] }
  );
  [
    ['고양이의 하루 🐱', '🐱', alice.id], ['주말 브런치 레시피 🍳', '🍳', bob.id], ['한강 러닝 vlog 🏃', '🏃', alice.id],
    ['코딩 ASMR ⌨️', '⌨️', bob.id], ['도쿄 야경 🌃', '🌃', alice.id], ['홈카페 라떼아트 ☕', '☕', bob.id]
  ].forEach(([title, emoji, author], i) => voomPosts.push({
    id: uid(), title, emoji, author_id: author, bg: bg[i % bg.length], likes: i * 3, liked_by: [], comments: [], created_at: now()
  }));
  noteStore.set('00000000-0000-0000-0000-000000000010', []);
  eventStore.set('00000000-0000-0000-0000-000000000010', []);
  const conv = conversations.get('00000000-0000-0000-0000-000000000010');
  conv.owner_id = alice.id;
  readCursors.set(`${conv.id}:${alice.id}`, 1);
}

const featureFlags = [
  { key: 'e2ee_letter_sealing', enabled: true, description: 'Signal Letter-Sealing End-to-End Encryption' },
  { key: 'webrtc_group_calling', enabled: true, description: 'Multi-party Voice & Video Calling' },
  { key: 'ai_smart_replies', enabled: true, description: 'AI-assisted quick responses' },
  { key: 'openchat_live_streaming', enabled: true, description: 'Public community voice stages' }
];

function broadcast(payload) {
  const msg = JSON.stringify(payload);
  const conversationId = payload.conversation_id || callSessions.get(payload.call_id)?.conversation_id;
  const conversation = conversations.get(conversationId);
  const publicRoom = payload.room_id && communityRooms.get(payload.room_id)?.visibility === 'public';
  if (!conversation && !publicRoom) return;
  for (const [ws, userId] of activeSockets) {
    if (conversation && !conversation.members.includes(userId)) continue;
    try {
      ws.send(msg);
    } catch {
      activeSockets.delete(ws);
    }
  }
}

// Helper: Seed default accounts and base data if empty
function ensureSeed() {
  if (users.size > 0) return;

  const seeds = [
    {
      id: '00000000-0000-0000-0000-000000000001',
      email: 'admin@line.local',
      password: 'LineAdmin#2026!',
      display_name: 'System Administrator',
      role: 'admin',
      handle: 'admin',
      bio: 'Cloudflare Edge Master Admin'
    },
    {
      id: '00000000-0000-0000-0000-000000000002',
      email: 'user@line.local',
      password: 'LineUser#2026!',
      display_name: 'LINE Demo User',
      role: 'user',
      handle: 'line_user',
      bio: 'Cloudflare Demo Messenger User'
    },
    {
      id: '00000000-0000-0000-0000-000000000003',
      email: 'alice@line-test.local',
      password: 'AliceStrongPass#2026',
      display_name: 'Alice Kim',
      role: 'user',
      handle: 'alice_kim',
      bio: 'Alice in Cloudflare'
    },
    {
      id: '00000000-0000-0000-0000-000000000004',
      email: 'bob@line-test.local',
      password: 'BobStrongPass#2026',
      display_name: 'Bob Lee',
      role: 'user',
      handle: 'bob_lee',
      bio: 'Bob in Cloudflare'
    }
  ];

  for (const s of seeds) {
    users.set(s.email.toLowerCase(), s);
    usersById.set(s.id, s);
  }

  // Pre-seed friendship between Alice and Bob
  const alice = users.get('alice@line-test.local');
  const bob = users.get('bob@line-test.local');
  friendships.set(`${alice.id}:${bob.id}`, true);
  friendships.set(`${bob.id}:${alice.id}`, true);

  // Pre-seed default direct conversation
  const convId = '00000000-0000-0000-0000-000000000010';
  conversations.set(convId, {
    id: convId,
    kind: 'direct',
    title: 'Alice & Bob',
    members: [alice.id, bob.id],
    last_sequence: 1,
    updated_at: new Date().toISOString()
  });
  messages.set(convId, [
    {
      id: '00000000-0000-0000-0000-000000000100',
      conversation_id: convId,
      sender_id: alice.id,
      client_id: 'welcome-msg-1',
      sequence: 1,
      kind: 'text',
      body: 'Welcome to Cloudflare Global Messenger!',
      created_at: new Date().toISOString()
    }
  ]);

  // Pre-seed OpenChat room
  communityRooms.set('room-1', {
    id: 'room-1',
    name: 'LINE Global Developer Lounge',
    description: 'Cloudflare Edge OpenChat for engineers and creators',
    visibility: 'public',
    created_at: new Date().toISOString()
  });
  communityPosts.set('room-1', [
    { id: 'post-1', room_id: 'room-1', author_name: 'LineBot', body: 'Welcome to OpenChat on Cloudflare Edge!', created_at: new Date().toISOString() }
  ]);
  seedExtras();
}

function json(data, status = 200, headers = {}) {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      'Content-Type': 'application/json; charset=utf-8',
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Headers': 'Authorization, Content-Type',
      'Access-Control-Allow-Methods': 'GET, POST, PUT, PATCH, DELETE, OPTIONS',
      ...headers
    }
  });
}

function getAuthUser(request) {
  const auth = request.headers.get('Authorization') || '';
  if (!auth.startsWith('Bearer ')) return null;
  const token = auth.slice(7).trim();
  const session = sessions.get(token);
  if (!session) return null;
  const user = usersById.get(session.user_id) || Array.from(users.values()).find(u => u.id === session.user_id) || null;
  if (user && user.state === 'suspended') return null;
  return user;
}

function sendTo(userIds, payload) {
  const msg = JSON.stringify(payload);
  const set = new Set(userIds);
  for (const [ws, userId] of activeSockets) {
    if (!set.has(userId)) continue;
    try { ws.send(msg); } catch { activeSockets.delete(ws); }
  }
}
async function readJson(request) {
  const text = await request.text().catch(() => '');
  if (text.length > 3_000_000) return {};
  try { const v = JSON.parse(text || '{}'); return v && typeof v === 'object' ? v : {}; } catch { return {}; }
}
function clip(v, max) { return typeof v === 'string' ? v.trim().slice(0, max) : ''; }
function bad(message, status = 400) { return json({ error: message }, status); }
function intIn(v, min, max) {
  const n = Number(v);
  return Number.isInteger(n) && n >= min && n <= max ? n : null;
}
function findDirect(a, b) {
  for (const c of conversations.values()) {
    if (c.kind === 'direct' && c.members.length === 2 && c.members.includes(a) && c.members.includes(b)) return c;
  }
  return null;
}
function createConversation(fields) {
  const c = { id: uid(), last_sequence: 0, updated_at: now(), ...fields };
  conversations.set(c.id, c);
  messages.set(c.id, []);
  return c;
}
function addAudit(actor, action, target) {
  auditLogs.unshift({ id: auditLogs.length + 1, actor_user_id: actor, action, target_id: target, created_at: now() });
}
function pollView(poll, viewerId) {
  const voters = new Set();
  poll.options.forEach(o => o.voters.forEach(v => voters.add(v)));
  return {
    id: poll.id, conversation_id: poll.conversation_id, question: poll.question, multiple: poll.multiple,
    closed: poll.closed, created_by: poll.created_by, created_at: poll.created_at, total_voters: voters.size,
    options: poll.options.map(o => ({ id: o.id, text: o.text, votes: o.voters.size, voted: o.voters.has(viewerId) }))
  };
}
function timelineView(post, viewerId) {
  return {
    id: post.id, author: userBrief(post.author_id), body: post.body, bg: post.bg, visibility: post.visibility,
    created_at: post.created_at, like_count: post.likes.length, liked: post.likes.includes(viewerId),
    comments: post.comments.map(c => ({ ...c, author: userBrief(c.author_id) }))
  };
}
function voomView(v, viewerId) {
  return {
    id: v.id, title: v.title, emoji: v.emoji, bg: v.bg, author: userBrief(v.author_id), created_at: v.created_at,
    like_count: v.likes + v.liked_by.length, liked: v.liked_by.includes(viewerId),
    comments: v.comments.map(c => ({ ...c, author: userBrief(c.author_id) }))
  };
}
const MESSAGE_KINDS = new Set(['text', 'sticker', 'image', 'video', 'voice', 'file', 'location', 'contact']);
const REACTIONS = ['❤️', '😍', '😂', '😮', '😢', '👍'];
const INLINE_MIME = /^(image\/(png|jpeg|gif|webp)|audio\/(webm|ogg|mp4|mpeg|wav)|video\/(mp4|webm))$/;

/**
 * Extended LINE-class messenger API. Returns a Response when a route is handled, otherwise null.
 * Membership of /conversations/{id}/* and /calls/{id}/* is enforced by the caller before this runs.
 */
async function extended(p, request, url, me, token) {
  const method = request.method;
  let m;
  const myId = me.id;

  // ---------- Profile & settings ----------
  if (p === 'me') {
    if (method === 'GET') return json({ ...publicUser(me), email: me.email, role: me.role });
    if (method === 'PATCH') {
      const b = await readJson(request);
      if (b.display_name !== undefined) {
        const v = clip(b.display_name, 40);
        if (!v) return bad('이름을 입력하세요');
        me.display_name = v;
      }
      if (b.bio !== undefined) me.bio = clip(b.bio, 200);
      if (b.status_message !== undefined) me.status_message = clip(b.status_message, 100);
      if (b.birthday !== undefined) {
        if (b.birthday !== '' && !/^(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$/.test(b.birthday)) return bad('생일은 MM-DD 형식입니다');
        me.birthday = b.birthday;
      }
      if (b.avatar_color !== undefined) {
        if (!/^#[0-9a-fA-F]{6}$/.test(b.avatar_color)) return bad('색상 형식이 올바르지 않습니다');
        me.avatar_color = b.avatar_color;
      }
      if (b.handle !== undefined) {
        const h = String(b.handle).toLowerCase();
        if (!/^[a-z0-9_.]{3,20}$/.test(h)) return bad('ID는 영문 소문자·숫자·_ . 3~20자입니다');
        if (h !== me.handle && Array.from(users.values()).some(u => u.handle === h)) return bad('이미 사용 중인 ID입니다', 409);
        me.handle = h;
      }
      return json({ ...publicUser(me), email: me.email, role: me.role });
    }
  }
  if (p === 'me/settings') {
    const prefs = prefsOf(myId);
    if (method === 'GET') return json({ ...prefs });
    if (method === 'PATCH' || method === 'PUT') {
      const b = await readJson(request);
      for (const k of Object.keys(DEFAULT_PREFS)) {
        if (b[k] === undefined) continue;
        if (typeof b[k] !== typeof DEFAULT_PREFS[k]) return bad(`${k} 값 형식이 올바르지 않습니다`);
        if (k === 'language' && !['ko', 'en', 'ja'].includes(b[k])) return bad('지원하지 않는 언어입니다');
        if (k === 'font_size' && !['small', 'medium', 'large'].includes(b[k])) return bad('글자 크기 값 오류');
        if (k === 'theme' && !themesOf(myId).has(b[k])) return bad('보유하지 않은 테마입니다', 403);
        if (k === 'wallpaper' && !['default', 'dots', 'waves', 'grid'].includes(b[k])) return bad('배경 값 오류');
        prefs[k] = b[k];
      }
      return json({ ...prefs });
    }
  }
  if (p === 'me/devices' && method === 'GET') {
    const list = [];
    for (const [tok, s] of sessions) if (s.user_id === myId) list.push({ device_id: s.device_id, current: tok === token, created_at: s.created_at || null });
    return json({ devices: list });
  }
  if ((m = p.match(/^me\/devices\/([\w-]+)$/)) && method === 'DELETE') {
    for (const [tok, s] of Array.from(sessions)) if (s.user_id === myId && s.device_id === m[1]) sessions.delete(tok);
    return json({ status: 'revoked' });
  }
  if (p === 'me/password' && method === 'POST') {
    const b = await readJson(request);
    if (!(await checkPassword(me, b.old_password))) return bad('현재 비밀번호가 일치하지 않습니다', 403);
    if (typeof b.new_password !== 'string' || b.new_password.length < 10) return bad('새 비밀번호는 10자 이상이어야 합니다');
    await setPassword(me, b.new_password);
    for (const [tok, s] of Array.from(sessions)) if (s.user_id === myId && tok !== token) sessions.delete(tok);
    return json({ status: 'changed' });
  }
  if (p === 'me/qr' && method === 'GET') return json({ code: `LINE-FRIEND:${me.handle}`, handle: me.handle });
  if (p === 'me/stickers' && method === 'GET') {
    const owned = packsOf(myId);
    return json({ packs: STICKER_CATALOG.filter(pk => owned.has(pk.id)) });
  }

  // ---------- Friends & users ----------
  if (p === 'friends') {
    if (method === 'GET') {
      const today = now().slice(5, 10);
      const list = friendIds(myId).map(id => {
        const u = usersById.get(id);
        if (!u || u.state === 'suspended') return null;
        const meta = fMeta(myId, id);
        return { ...publicUser(u), is_favorite: meta.favorite, is_hidden: meta.hidden, is_blocked: meta.blocked, nickname: meta.nickname, birthday_today: !!u.birthday && u.birthday === today };
      }).filter(Boolean);
      return json({ friends: list });
    }
    if (method === 'POST') {
      const b = await readJson(request);
      let target = null;
      if (b.friend_id || b.target_user_id) target = usersById.get(b.friend_id || b.target_user_id);
      else if (b.handle || b.qr_code) {
        const h = String(b.handle || String(b.qr_code).replace(/^LINE-FRIEND:/i, '')).toLowerCase().replace(/^@/, '');
        target = Array.from(users.values()).find(u => u.handle === h);
      }
      if (!target || target.state === 'suspended') return bad('사용자를 찾을 수 없습니다', 404);
      if (target.id === myId) return bad('자기 자신은 친구로 추가할 수 없습니다');
      if (isFriend(myId, target.id)) return json({ status: 'already_friends', friend: publicUser(target) });
      if (!prefsOf(target.id).allow_friend_requests || fMeta(target.id, myId).blocked) return bad('친구 추가할 수 없는 사용자입니다', 403);
      friendships.set(`${myId}:${target.id}`, true);
      friendships.set(`${target.id}:${myId}`, true);
      sendTo([target.id, myId], { type: 'friends.updated' });
      return json({ status: 'friend_added', friend: publicUser(target) }, 201);
    }
  }
  if ((m = p.match(/^friends\/([\w-]+)$/))) {
    const fid = m[1];
    if (!isFriend(myId, fid)) return bad('친구가 아닙니다', 404);
    if (method === 'PATCH') {
      const b = await readJson(request);
      const meta = fMeta(myId, fid);
      for (const k of ['favorite', 'hidden', 'blocked']) if (typeof b[k] === 'boolean') meta[k] = b[k];
      if (b.nickname !== undefined) meta.nickname = clip(b.nickname, 30);
      return json({ status: 'updated', meta });
    }
    if (method === 'DELETE') {
      friendships.delete(`${myId}:${fid}`);
      friendships.delete(`${fid}:${myId}`);
      friendMeta.delete(`${myId}:${fid}`);
      friendMeta.delete(`${fid}:${myId}`);
      sendTo([fid, myId], { type: 'friends.updated' });
      return json({ status: 'removed' });
    }
  }
  if (p === 'users' && method === 'GET') {
    const q = clip(url.searchParams.get('q') || url.searchParams.get('search') || '', 50).toLowerCase().replace(/^@/, '');
    if (q.length < 2) return json({ users: [] });
    const found = Array.from(users.values()).filter(u => {
      if (u.id === myId || u.state === 'suspended' || u.role === 'admin') return false;
      const pr = prefsOf(u.id);
      return u.display_name.toLowerCase().includes(q) || (pr.searchable_by_id && u.handle === q) || (pr.searchable_by_email && u.email === q);
    }).slice(0, 20).map(u => ({ ...publicUser(u), is_friend: isFriend(myId, u.id) }));
    return json({ users: found });
  }
  if ((m = p.match(/^users\/([\w-]+)$/)) && method === 'GET') {
    const u = usersById.get(m[1]);
    if (!u || u.state === 'suspended') return bad('사용자를 찾을 수 없습니다', 404);
    return json({ ...publicUser(u), is_friend: isFriend(myId, u.id) });
  }

  // ---------- Official accounts ----------
  if (p === 'official-accounts' && method === 'GET') {
    const mine = followedOfficial.get(myId) || new Map();
    return json({ accounts: Array.from(officialAccounts.values()).map(o => ({ id: o.id, name: o.name, handle: o.handle, description: o.description, color: o.color, followed: mine.has(o.id) })) });
  }
  if ((m = p.match(/^official-accounts\/([\w-]+)\/follow$/))) {
    const acct = officialAccounts.get(m[1]);
    if (!acct) return bad('공식계정을 찾을 수 없습니다', 404);
    if (!followedOfficial.has(myId)) followedOfficial.set(myId, new Map());
    const mine = followedOfficial.get(myId);
    if (method === 'POST') {
      if (mine.has(acct.id)) return json({ status: 'already_following', conversation_id: mine.get(acct.id) });
      const c = createConversation({ kind: 'official', title: acct.name, members: [myId, acct.id], official: true });
      mine.set(acct.id, c.id);
      pushMessage(c.id, acct.id, { kind: 'text', body: `${acct.name} 공식계정을 추가해 주셔서 감사합니다! 🎉 '도움말'을 입력해 보세요.` });
      sendTo([myId], { type: 'conversation.updated', conversation_id: c.id });
      return json({ status: 'following', conversation_id: c.id }, 201);
    }
    if (method === 'DELETE') {
      const cid = mine.get(acct.id);
      if (cid) cMeta(cid, myId).hidden = true;
      mine.delete(acct.id);
      return json({ status: 'unfollowed' });
    }
  }

  // ---------- Conversations ----------
  if (p === 'conversations') {
    if (method === 'GET') {
      const includeHidden = url.searchParams.get('include_hidden') === '1';
      const list = Array.from(conversations.values()).filter(c => c.members.includes(myId))
        .map(c => chatSummary(c, myId)).filter(c => includeHidden || !c.hidden)
        .sort((a, b) => (b.pinned - a.pinned) || String(b.updated_at).localeCompare(String(a.updated_at)));
      return json({ conversations: list });
    }
    if (method === 'POST') {
      const b = await readJson(request);
      const kind = b.kind === 'group' ? 'group' : 'direct';
      const ids = Array.from(new Set((Array.isArray(b.member_ids) ? b.member_ids : []).filter(x => typeof x === 'string' && x !== myId)));
      if (!ids.length || ids.length > 499) return bad('대화 상대를 선택하세요');
      for (const id of ids) {
        const u = usersById.get(id);
        if (!u || u.state === 'suspended') return bad('존재하지 않는 사용자가 포함되어 있습니다', 404);
        if (fMeta(myId, id).blocked) return bad('차단한 사용자와는 대화할 수 없습니다', 403);
        if (!isFriend(myId, id)) return bad('친구만 초대할 수 있습니다', 403);
      }
      if (kind === 'direct') {
        if (ids.length !== 1) return bad('1:1 대화는 상대가 한 명이어야 합니다');
        const existing = findDirect(myId, ids[0]);
        if (existing) { cMeta(existing.id, myId).hidden = false; return json(chatSummary(existing, myId)); }
        const c = createConversation({ kind: 'direct', title: '', members: [myId, ids[0]] });
        sendTo(c.members, { type: 'conversation.updated', conversation_id: c.id });
        return json(chatSummary(c, myId), 201);
      }
      const title = clip(b.title, 50);
      if (!title) return bad('그룹 이름을 입력하세요');
      const c = createConversation({ kind: 'group', title, members: [myId, ...ids], owner_id: myId });
      systemMessage(c.id, `${me.display_name}님이 그룹을 만들었습니다.`);
      sendTo(c.members, { type: 'conversation.updated', conversation_id: c.id });
      return json(chatSummary(c, myId), 201);
    }
  }
  if ((m = p.match(/^conversations\/([\w-]+)$/))) {
    const conv = conversations.get(m[1]);
    if (method === 'GET') return json(chatSummary(conv, myId));
    if (method === 'PATCH') {
      const b = await readJson(request);
      if (b.title !== undefined) {
        if (conv.kind !== 'group') return bad('그룹 채팅방만 이름을 바꿀 수 있습니다');
        const t = clip(b.title, 50);
        if (!t) return bad('그룹 이름을 입력하세요');
        conv.title = t;
      }
      if (b.notice !== undefined) conv.notice = clip(b.notice, 500);
      sendTo(conv.members, { type: 'conversation.updated', conversation_id: conv.id });
      return json(chatSummary(conv, myId));
    }
    if (method === 'DELETE') {
      if (conv.kind === 'group') return await leaveConversation(conv, myId, me);
      cMeta(conv.id, myId).hidden = true;
      const hs = hiddenMsgs.get(myId) || new Set();
      (messages.get(conv.id) || []).forEach(x => hs.add(x.id));
      hiddenMsgs.set(myId, hs);
      return json({ status: 'hidden' });
    }
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/meta$/)) && method === 'PATCH') {
    const b = await readJson(request);
    const meta = cMeta(m[1], myId);
    for (const k of ['pinned', 'muted', 'hidden']) if (typeof b[k] === 'boolean') meta[k] = b[k];
    return json({ status: 'updated', meta });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/members$/)) && method === 'POST') {
    const conv = conversations.get(m[1]);
    if (conv.kind !== 'group') return bad('그룹 채팅방에서만 초대할 수 있습니다');
    const b = await readJson(request);
    const ids = Array.from(new Set((Array.isArray(b.member_ids) ? b.member_ids : []).filter(x => typeof x === 'string')));
    const added = [];
    for (const id of ids) {
      const u = usersById.get(id);
      if (!u || conv.members.includes(id) || !isFriend(myId, id) || fMeta(myId, id).blocked) continue;
      conv.members.push(id);
      added.push(u.display_name);
    }
    if (!added.length) return bad('초대할 수 있는 친구가 없습니다');
    systemMessage(conv.id, `${me.display_name}님이 ${added.join(', ')}님을 초대했습니다.`);
    sendTo(conv.members, { type: 'conversation.updated', conversation_id: conv.id });
    return json(chatSummary(conv, myId), 201);
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/members\/([\w-]+)$/)) && method === 'DELETE') {
    const conv = conversations.get(m[1]);
    if (conv.kind !== 'group') return bad('그룹 채팅방이 아닙니다');
    if (m[2] === myId) return await leaveConversation(conv, myId, me);
    if (conv.owner_id !== myId) return bad('방장만 멤버를 내보낼 수 있습니다', 403);
    if (!conv.members.includes(m[2])) return bad('멤버가 아닙니다', 404);
    const target = usersById.get(m[2]);
    conv.members = conv.members.filter(x => x !== m[2]);
    systemMessage(conv.id, `${target ? target.display_name : '멤버'}님이 내보내졌습니다.`);
    sendTo([...conv.members, m[2]], { type: 'conversation.updated', conversation_id: conv.id });
    return json({ status: 'kicked' });
  }

  // ---------- Messages ----------
  if ((m = p.match(/^conversations\/([\w-]+)\/messages$/))) {
    const cid = m[1];
    const conv = conversations.get(cid);
    if (method === 'GET') {
      const before = intIn(url.searchParams.get('before_sequence') || 0, 0, 1e9) || 0;
      const limit = intIn(url.searchParams.get('limit') || 50, 1, 100) || 50;
      const hs = hiddenMsgs.get(myId);
      let list = (messages.get(cid) || []).filter(x => !(hs && hs.has(x.id)));
      if (before) list = list.filter(x => x.sequence < before);
      return json({ messages: list.slice(-limit).map(x => decorateMessage(x, myId, cid)), has_more: list.length > limit });
    }
    if (method === 'POST') {
      const b = await readJson(request);
      const kind = b.kind || 'text';
      if (!MESSAGE_KINDS.has(kind)) return bad('지원하지 않는 메시지 종류입니다');
      if (conv.kind === 'direct') {
        const other = conv.members.find(x => x !== myId);
        if (other && fMeta(myId, other).blocked) return bad('차단한 사용자에게는 메시지를 보낼 수 없습니다', 403);
      }
      if (b.client_id) {
        const dup = (messages.get(cid) || []).find(x => x.client_id === b.client_id && x.sender_id === myId);
        if (dup) return json({ message: decorateMessage(dup, myId, cid), duplicate: true }, 200);
      }
      let body = typeof b.body === 'string' ? b.body : '';
      let metadata = {};
      const meta = b.metadata && typeof b.metadata === 'object' ? b.metadata : {};
      if (kind === 'text') {
        body = body.trim();
        if (!body) return bad('메시지를 입력하세요');
        if (body.length > MAX_BODY) return bad('메시지가 너무 깁니다');
      } else if (kind === 'sticker') {
        const pack = STICKER_CATALOG.find(pk => pk.id === meta.pack_id);
        if (!pack || !pack.stickers.includes(body)) return bad('올바르지 않은 스티커입니다');
        if (!packsOf(myId).has(pack.id)) return bad('보유하지 않은 스티커입니다', 403);
        metadata.pack_id = pack.id;
      } else if (kind === 'location') {
        const lat = Number(meta.lat), lng = Number(meta.lng);
        if (!Number.isFinite(lat) || !Number.isFinite(lng) || Math.abs(lat) > 90 || Math.abs(lng) > 180) return bad('위치 좌표가 올바르지 않습니다');
        metadata = { lat, lng, label: clip(meta.label || '', 100) };
        body = metadata.label || `위도 ${lat.toFixed(4)}, 경도 ${lng.toFixed(4)}`;
      } else if (kind === 'contact') {
        const u = usersById.get(meta.user_id);
        if (!u) return bad('연락처 사용자를 찾을 수 없습니다', 404);
        metadata = { user_id: u.id, handle: u.handle };
        body = u.display_name;
      } else {
        const att = attachmentStore.get(meta.attachment_id);
        if (!att || att.conversation_id !== cid) return bad('첨부 파일을 찾을 수 없습니다', 404);
        metadata = { attachment_id: att.id, mime_type: att.mime, name: att.name, size_bytes: att.size };
        if (kind === 'voice') metadata.duration = intIn(meta.duration, 0, 300) ?? 0;
        body = clip(body || att.name, 200);
      }
      if (Array.isArray(meta.mentions)) {
        metadata.mentions = Array.from(new Set(meta.mentions.filter(x => (x === 'all' && conv.kind === 'group') || conv.members.includes(x)))).slice(0, 50);
      }
      if (meta.forwarded === true) metadata.forwarded = true;
      let replyTo;
      if (b.reply_to) {
        if (!(messages.get(cid) || []).some(x => x.id === b.reply_to)) return bad('답장할 메시지를 찾을 수 없습니다', 404);
        replyTo = b.reply_to;
      }
      const msg = pushMessage(cid, myId, { client_id: b.client_id, kind, body, reply_to: replyTo, metadata: Object.keys(metadata).length ? metadata : undefined });
      if (conv.official) {
        const acct = officialAccounts.get(conv.members.find(x => officialAccounts.has(x)));
        if (acct && kind === 'text') pushMessage(cid, acct.id, { kind: 'text', body: botReply(body, acct) });
      }
      return json({ message: decorateMessage(msg, myId, cid), duplicate: false }, 201);
    }
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/messages\/([\w-]+)$/))) {
    const cid = m[1];
    const list = messages.get(cid) || [];
    const msg = list.find(x => x.id === m[2]);
    if (!msg) return bad('메시지를 찾을 수 없습니다', 404);
    if (['PATCH', 'DELETE'].includes(method) && msg.sender_id !== myId) return bad('보낸 사람만 수정하거나 취소할 수 있습니다', 403);
    if (method === 'PATCH') {
      if (msg.kind !== 'text' || msg.unsent) return bad('텍스트 메시지만 수정할 수 있습니다');
      const b = await readJson(request);
      const text = clip(b.body, MAX_BODY);
      if (!text) return bad('메시지를 입력하세요');
      msg.body = text;
      msg.edited_at = now();
      broadcast({ type: 'message.updated', conversation_id: cid, message_id: msg.id, body: msg.body });
      return json({ status: 'edited', message: decorateMessage(msg, myId, cid) });
    }
    if (method === 'DELETE') {
      if (Date.now() - Date.parse(msg.created_at) > 24 * 3600 * 1000) return bad('24시간이 지난 메시지는 전송 취소할 수 없습니다');
      msg.unsent = true;
      msg.body = '';
      msg.metadata = undefined;
      const pins = pinnedMessages.get(cid);
      if (pins) pinnedMessages.set(cid, pins.filter(x => x.message_id !== msg.id));
      broadcast({ type: 'message.unsent', conversation_id: cid, message_id: msg.id });
      return json({ status: 'recalled', message_id: msg.id });
    }
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/messages\/([\w-]+)\/hide$/)) && method === 'POST') {
    const hs = hiddenMsgs.get(myId) || new Set();
    hs.add(m[2]);
    hiddenMsgs.set(myId, hs);
    return json({ status: 'hidden' });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/messages\/([\w-]+)\/reactions$/)) && method === 'POST') {
    const cid = m[1], mid = m[2];
    if (!(messages.get(cid) || []).some(x => x.id === mid && !x.unsent)) return bad('메시지를 찾을 수 없습니다', 404);
    const b = await readJson(request);
    if (!REACTIONS.includes(b.reaction)) return bad('지원하지 않는 반응입니다');
    if (!reactionStore.has(mid)) reactionStore.set(mid, new Map());
    const rm = reactionStore.get(mid);
    for (const [emoji, set] of rm) if (emoji !== b.reaction) set.delete(myId);
    if (!rm.has(b.reaction)) rm.set(b.reaction, new Set());
    const set = rm.get(b.reaction);
    const removed = set.has(myId);
    if (removed) set.delete(myId); else set.add(myId);
    const counts = reactionCounts(mid);
    broadcast({ type: 'message.reaction', conversation_id: cid, message_id: mid, reactions: counts });
    return json({ status: removed ? 'removed' : 'reacted', reactions: counts }, removed ? 200 : 201);
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/read$/)) && method === 'POST') {
    const conv = conversations.get(m[1]);
    const b = await readJson(request);
    const seq = intIn(b.sequence ?? conv.last_sequence, 0, 1e9) ?? 0;
    const key = `${conv.id}:${myId}`;
    const next = Math.max(readCursors.get(key) || 0, Math.min(seq, conv.last_sequence || 0));
    readCursors.set(key, next);
    broadcast({ type: 'conversation.read', conversation_id: conv.id, user_id: myId, sequence: next });
    return json({ status: 'read', sequence: next });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/typing$/)) && method === 'POST') {
    broadcast({ type: 'conversation.typing', conversation_id: m[1], user_id: myId });
    return json({ status: 'typing' });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/pins$/))) {
    const cid = m[1];
    const conv = conversations.get(cid);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const pins = pinnedMessages.get(cid) || [];
    const view = () => pins.map(x => {
      const src = (messages.get(cid) || []).find(y => y.id === x.message_id);
      return { ...x, body: src && !src.unsent ? String(src.body).slice(0, 120) : '', kind: src?.kind };
    }).filter(x => x.body || x.kind);
    if (method === 'GET') return json({ pins: view() });
    if (method === 'POST') {
      const b = await readJson(request);
      const src = (messages.get(cid) || []).find(y => y.id === b.message_id && !y.unsent);
      if (!src) return bad('메시지를 찾을 수 없습니다', 404);
      if (!pins.some(x => x.message_id === src.id)) pins.unshift({ message_id: src.id, pinned_by: myId, pinned_at: now() });
      pinnedMessages.set(cid, pins);
      broadcast({ type: 'conversation.pins', conversation_id: cid });
      return json({ status: 'pinned', message_id: src.id }, 201);
    }
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/pins\/([\w-]+)$/)) && method === 'DELETE') {
    const conv = conversations.get(m[1]);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    pinnedMessages.set(m[1], (pinnedMessages.get(m[1]) || []).filter(x => x.message_id !== m[2]));
    broadcast({ type: 'conversation.pins', conversation_id: m[1] });
    return json({ status: 'unpinned' });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/media$/)) && method === 'GET') {
    const conv = conversations.get(m[1]);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const kind = url.searchParams.get('kind') || 'image';
    const hs = hiddenMsgs.get(myId);
    const list = (messages.get(m[1]) || []).filter(x => !x.unsent && !(hs && hs.has(x.id)));
    let out;
    if (kind === 'link') out = list.filter(x => x.kind === 'text' && /https?:\/\/\S+/i.test(x.body));
    else if (kind === 'file') out = list.filter(x => x.kind === 'file');
    else out = list.filter(x => x.kind === 'image' || x.kind === 'video');
    return json({ items: out.slice(-100).reverse().map(x => decorateMessage(x, myId, m[1])) });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/search$/)) && method === 'GET') {
    const conv = conversations.get(m[1]);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const q = clip(url.searchParams.get('q') || '', 100).toLowerCase();
    if (q.length < 1) return json({ messages: [] });
    const hs = hiddenMsgs.get(myId);
    const out = (messages.get(m[1]) || []).filter(x => !x.unsent && x.kind !== 'sticker' && !(hs && hs.has(x.id)) && String(x.body).toLowerCase().includes(q));
    return json({ messages: out.slice(-50).reverse().map(x => decorateMessage(x, myId, m[1])) });
  }

  // ---------- Polls, notes, events, albums ----------
  if ((m = p.match(/^conversations\/([\w-]+)\/polls$/))) {
    const cid = m[1];
    const conv = conversations.get(cid);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    if (method === 'GET') return json({ polls: Array.from(polls.values()).filter(x => x.conversation_id === cid).map(x => pollView(x, myId)) });
    if (method === 'POST') {
      const b = await readJson(request);
      const question = clip(b.question, 200);
      const opts = (Array.isArray(b.options) ? b.options : []).map(o => clip(String(o), 100)).filter(Boolean);
      if (!question || opts.length < 2 || opts.length > 10) return bad('질문과 2~10개의 선택지가 필요합니다');
      const poll = { id: uid(), conversation_id: cid, question, multiple: b.multiple === true, closed: false, created_by: myId, created_at: now(), options: opts.map((t, i) => ({ id: `opt-${i}`, text: t, voters: new Set() })) };
      polls.set(poll.id, poll);
      pushMessage(cid, myId, { kind: 'poll', body: question, metadata: { poll_id: poll.id } });
      broadcast({ type: 'poll.updated', conversation_id: cid, poll_id: poll.id });
      return json(pollView(poll, myId), 201);
    }
  }
  if ((m = p.match(/^polls\/([\w-]+)\/vote$/)) && method === 'POST') {
    const poll = polls.get(m[1]);
    if (!poll || !conversations.get(poll.conversation_id)?.members.includes(myId)) return bad('투표를 찾을 수 없습니다', 404);
    if (poll.closed) return bad('종료된 투표입니다');
    const b = await readJson(request);
    const opt = poll.options.find(o => o.id === b.option_id);
    if (!opt) return bad('선택지를 찾을 수 없습니다');
    if (opt.voters.has(myId)) opt.voters.delete(myId);
    else {
      if (!poll.multiple) poll.options.forEach(o => o.voters.delete(myId));
      opt.voters.add(myId);
    }
    broadcast({ type: 'poll.updated', conversation_id: poll.conversation_id, poll_id: poll.id });
    return json({ status: 'voted', poll: pollView(poll, myId) });
  }
  if ((m = p.match(/^polls\/([\w-]+)\/close$/)) && method === 'POST') {
    const poll = polls.get(m[1]);
    if (!poll || !conversations.get(poll.conversation_id)?.members.includes(myId)) return bad('투표를 찾을 수 없습니다', 404);
    if (poll.created_by !== myId) return bad('투표를 만든 사람만 종료할 수 있습니다', 403);
    poll.closed = true;
    broadcast({ type: 'poll.updated', conversation_id: poll.conversation_id, poll_id: poll.id });
    return json({ status: 'closed', poll: pollView(poll, myId) });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/notes$/))) {
    const cid = m[1];
    const conv = conversations.get(cid);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const list = noteStore.get(cid) || [];
    if (method === 'GET') return json({ notes: list.map(n => ({ ...n, author: userBrief(n.author_id) })) });
    if (method === 'POST') {
      const b = await readJson(request);
      const title = clip(b.title, 80), body = clip(b.body, 3000);
      if (!title) return bad('노트 제목을 입력하세요');
      const note = { id: uid(), title, body, author_id: myId, created_at: now() };
      list.unshift(note);
      noteStore.set(cid, list);
      return json({ ...note, author: userBrief(myId) }, 201);
    }
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/notes\/([\w-]+)$/)) && method === 'DELETE') {
    const conv = conversations.get(m[1]);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const list = noteStore.get(m[1]) || [];
    const note = list.find(n => n.id === m[2]);
    if (!note) return bad('노트를 찾을 수 없습니다', 404);
    if (note.author_id !== myId && conv.owner_id !== myId) return bad('삭제 권한이 없습니다', 403);
    noteStore.set(m[1], list.filter(n => n.id !== m[2]));
    return json({ status: 'deleted' });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/events$/))) {
    const cid = m[1];
    const conv = conversations.get(cid);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const list = eventStore.get(cid) || [];
    const view = e => ({ ...e, rsvps: Object.fromEntries(e.rsvps), my_rsvp: e.rsvps.get(myId) || '', creator: userBrief(e.created_by) });
    if (method === 'GET') return json({ events: list.map(view) });
    if (method === 'POST') {
      const b = await readJson(request);
      const title = clip(b.title, 80);
      const startsAt = Date.parse(b.starts_at);
      if (!title || Number.isNaN(startsAt)) return bad('일정 제목과 시작 시간이 필요합니다');
      const ev = { id: uid(), title, starts_at: new Date(startsAt).toISOString(), location: clip(b.location || '', 100), created_by: myId, created_at: now(), rsvps: new Map([[myId, 'going']]) };
      list.push(ev);
      list.sort((a, b2) => a.starts_at.localeCompare(b2.starts_at));
      eventStore.set(cid, list);
      pushMessage(cid, myId, { kind: 'text', body: `📅 일정 등록: ${title} (${new Date(startsAt).toLocaleString('ko-KR')})` });
      return json(view(ev), 201);
    }
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/events\/([\w-]+)\/rsvp$/)) && method === 'POST') {
    const conv = conversations.get(m[1]);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const ev = (eventStore.get(m[1]) || []).find(e => e.id === m[2]);
    if (!ev) return bad('일정을 찾을 수 없습니다', 404);
    const b = await readJson(request);
    if (!['going', 'maybe', 'no'].includes(b.status)) return bad('참석 상태 값이 올바르지 않습니다');
    ev.rsvps.set(myId, b.status);
    return json({ status: 'saved', rsvps: Object.fromEntries(ev.rsvps) });
  }
  if ((m = p.match(/^conversations\/([\w-]+)\/albums$/))) {
    const cid = m[1];
    const conv = conversations.get(cid);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없거나 접근 권한이 없습니다', 404);
    const list = albums.get(cid) || [];
    if (method === 'GET') return json({ albums: list });
    if (method === 'POST') {
      const b = await readJson(request);
      const title = clip(b.title, 60);
      if (!title) return bad('앨범 제목을 입력하세요');
      const ids = (Array.isArray(b.attachment_ids) ? b.attachment_ids : []).filter(id => attachmentStore.get(id)?.conversation_id === cid && attachmentStore.get(id).mime.startsWith('image/')).slice(0, 100);
      const album = { id: uid(), title, attachment_ids: ids, created_by: myId, created_at: now() };
      list.push(album);
      albums.set(cid, list);
      return json(album, 201);
    }
  }

  // ---------- Attachments ----------
  if (p === 'attachments' && method === 'POST') {
    const b = await readJson(request);
    const conv = conversations.get(b.conversation_id);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없습니다', 404);
    if (typeof b.content_base64 !== 'string' || !b.content_base64 || b.content_base64.length > MAX_ATTACHMENT_B64 || !/^[A-Za-z0-9+/=]+$/.test(b.content_base64)) return bad('파일이 비어 있거나 너무 큽니다 (최대 약 1.5MB)', 413);
    const mime = typeof b.mime_type === 'string' && /^[\w.+-]+\/[\w.+-]+$/.test(b.mime_type) ? b.mime_type.toLowerCase() : 'application/octet-stream';
    const size = Math.floor(b.content_base64.length * 3 / 4);
    const att = { id: uid(), owner: myId, conversation_id: conv.id, name: clip(b.original_name || 'file', 120) || 'file', mime, size, b64: b.content_base64 };
    attachmentStore.set(att.id, att);
    return json({ id: att.id, status: 'ready', original_name: att.name, mime_type: mime, size_bytes: size }, 201);
  }
  if ((m = p.match(/^attachments\/([\w-]+)$/)) && method === 'GET') {
    const att = attachmentStore.get(m[1]);
    if (!att || !conversations.get(att.conversation_id)?.members.includes(myId)) return bad('파일을 찾을 수 없습니다', 404);
    const bin = atob(att.b64);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    const inline = INLINE_MIME.test(att.mime);
    return new Response(bytes, {
      status: 200,
      headers: {
        'Content-Type': inline ? att.mime : 'application/octet-stream',
        'Content-Disposition': `${inline ? 'inline' : 'attachment'}; filename*=UTF-8''${encodeURIComponent(att.name)}`,
        'X-Content-Type-Options': 'nosniff', 'Content-Security-Policy': "default-src 'none'; sandbox",
        'Cache-Control': 'private, max-age=3600', 'Access-Control-Allow-Origin': '*'
      }
    });
  }

  // ---------- Calls ----------
  if (p === 'calls' && method === 'POST') {
    const b = await readJson(request);
    if (!['voice', 'video'].includes(b.kind || 'video')) return bad('통화 종류가 올바르지 않습니다');
    const conv = conversations.get(b.conversation_id);
    if (!conv || !conv.members.includes(myId)) return bad('대화방을 찾을 수 없습니다', 404);
    if (conv.official) return bad('공식계정과는 통화할 수 없습니다');
    const call = { id: uid(), conversation_id: conv.id, kind: b.kind || 'video', state: 'ringing', initiated_by: myId, started_at: now(), connected_at: null, ended_at: null };
    callSessions.set(call.id, call);
    broadcast({ type: 'call.ringing', call_id: call.id, conversation_id: conv.id, initiated_by: myId, kind: call.kind });
    return json({ id: call.id, state: 'ringing', kind: call.kind }, 201);
  }
  if (p === 'calls' && method === 'GET') {
    const out = Array.from(callSessions.values()).filter(c => conversations.get(c.conversation_id)?.members.includes(myId))
      .sort((a, b2) => b2.started_at.localeCompare(a.started_at)).slice(0, 100).map(c => {
        const conv = conversations.get(c.conversation_id);
        const duration = c.connected_at && c.ended_at ? Math.round((Date.parse(c.ended_at) - Date.parse(c.connected_at)) / 1000) : 0;
        return { id: c.id, conversation_id: c.conversation_id, title: chatSummary(conv, myId).title, kind: c.kind, state: c.state, result: c.result || '', duration, outgoing: c.initiated_by === myId, started_at: c.started_at };
      });
    return json({ calls: out });
  }
  if ((m = p.match(/^calls\/([\w-]+)\/state$/)) && method === 'POST') {
    const call = callSessions.get(m[1]);
    const b = await readJson(request);
    if (!['connected', 'ended', 'declined', 'missed'].includes(b.state)) return bad('통화 상태 값이 올바르지 않습니다');
    if (call.ended_at) return json({ status: call.state });
    if ((b.state === 'connected' || b.state === 'declined') && call.initiated_by === myId) return bad('발신자는 이 작업을 할 수 없습니다', 403);
    if (b.state === 'connected') {
      call.state = 'connected';
      call.connected_at = call.connected_at || now();
      broadcast({ type: 'call.state', call_id: call.id, state: 'connected' });
      return json({ status: 'connected' });
    }
    call.ended_at = now();
    call.state = 'ended';
    const duration = call.connected_at ? Math.round((Date.parse(call.ended_at) - Date.parse(call.connected_at)) / 1000) : 0;
    call.result = call.connected_at ? 'completed' : (b.state === 'declined' ? 'declined' : (call.initiated_by === myId ? 'canceled' : 'missed'));
    pushMessage(call.conversation_id, call.initiated_by, { kind: 'call', body: '', metadata: { call_id: call.id, call_kind: call.kind, duration, result: call.result } });
    broadcast({ type: 'call.state', call_id: call.id, state: 'ended', result: call.result });
    return json({ status: 'ended', result: call.result, duration });
  }

  // ---------- Timeline, stories, VOOM ----------
  if (p === 'timeline') {
    if (method === 'GET') {
      const visible = new Set([myId, ...friendIds(myId).filter(f => !fMeta(myId, f).hidden && !fMeta(myId, f).blocked)]);
      const list = timelinePosts.filter(x => visible.has(x.author_id) && (x.visibility === 'friends' || x.author_id === myId)).slice().reverse().slice(0, 100);
      return json({ posts: list.map(x => timelineView(x, myId)) });
    }
    if (method === 'POST') {
      const b = await readJson(request);
      const body = clip(b.body, 1000);
      if (!body) return bad('내용을 입력하세요');
      const bg = typeof b.bg === 'string' && /^(linear-gradient\([\w#%,\s().-]+\)|#[0-9a-fA-F]{6}|)$/.test(b.bg) ? b.bg : '';
      const post = { id: uid(), author_id: myId, body, bg, visibility: b.visibility === 'me' ? 'me' : 'friends', created_at: now(), likes: [], comments: [] };
      timelinePosts.push(post);
      sendTo(friendIds(myId), { type: 'timeline.updated' });
      return json(timelineView(post, myId), 201);
    }
  }
  if ((m = p.match(/^timeline\/([\w-]+)$/)) && method === 'DELETE') {
    const idx = timelinePosts.findIndex(x => x.id === m[1]);
    if (idx < 0) return bad('게시물을 찾을 수 없습니다', 404);
    if (timelinePosts[idx].author_id !== myId) return bad('삭제 권한이 없습니다', 403);
    timelinePosts.splice(idx, 1);
    return json({ status: 'deleted' });
  }
  if ((m = p.match(/^timeline\/([\w-]+)\/(like|comments)$/)) && method === 'POST') {
    const post = timelinePosts.find(x => x.id === m[1]);
    if (!post || (post.author_id !== myId && !isFriend(myId, post.author_id))) return bad('게시물을 찾을 수 없습니다', 404);
    if (m[2] === 'like') {
      const i = post.likes.indexOf(myId);
      if (i >= 0) post.likes.splice(i, 1); else post.likes.push(myId);
      return json(timelineView(post, myId));
    }
    const b = await readJson(request);
    const body = clip(b.body, 300);
    if (!body) return bad('댓글을 입력하세요');
    post.comments.push({ id: uid(), author_id: myId, body, created_at: now() });
    return json(timelineView(post, myId), 201);
  }
  if (p === 'stories') {
    if (method === 'GET') {
      const visible = new Set([myId, ...friendIds(myId).filter(f => !fMeta(myId, f).hidden && !fMeta(myId, f).blocked)]);
      const cutoff = Date.now() - 24 * 3600 * 1000;
      const list = storyStore.filter(x => visible.has(x.author_id) && Date.parse(x.created_at) > cutoff).slice().reverse();
      return json({ stories: list.map(x => ({ id: x.id, author: userBrief(x.author_id), body: x.body, bg: x.bg, created_at: x.created_at, viewed: x.viewers.includes(myId), view_count: x.author_id === myId ? x.viewers.length : undefined })) });
    }
    if (method === 'POST') {
      const b = await readJson(request);
      const body = clip(b.body, 200);
      if (!body) return bad('스토리 내용을 입력하세요');
      const bg = typeof b.bg === 'string' && /^(linear-gradient\([\w#%,\s().-]+\)|#[0-9a-fA-F]{6}|)$/.test(b.bg) ? b.bg : '';
      const story = { id: uid(), author_id: myId, body, bg, created_at: now(), viewers: [] };
      storyStore.push(story);
      sendTo(friendIds(myId), { type: 'timeline.updated' });
      return json({ id: story.id }, 201);
    }
  }
  if ((m = p.match(/^stories\/([\w-]+)\/view$/)) && method === 'POST') {
    const s = storyStore.find(x => x.id === m[1]);
    if (s && s.author_id !== myId && isFriend(myId, s.author_id) && !s.viewers.includes(myId)) s.viewers.push(myId);
    return json({ status: 'viewed' });
  }
  if (p === 'voom') {
    if (method === 'GET') return json({ videos: voomPosts.slice().reverse().map(x => voomView(x, myId)) });
    if (method === 'POST') {
      const b = await readJson(request);
      const title = clip(b.title, 100);
      if (!title) return bad('제목을 입력하세요');
      const emoji = clip(b.emoji || '🎬', 8) || '🎬';
      const bgs = ['linear-gradient(135deg,#06c755,#0ea5e9)', 'linear-gradient(135deg,#f97316,#f43f5e)', 'linear-gradient(135deg,#6366f1,#ec4899)', 'linear-gradient(135deg,#14b8a6,#84cc16)'];
      const v = { id: uid(), title, emoji, author_id: myId, bg: bgs[voomPosts.length % bgs.length], likes: 0, liked_by: [], comments: [], created_at: now() };
      voomPosts.push(v);
      return json(voomView(v, myId), 201);
    }
  }
  if ((m = p.match(/^voom\/([\w-]+)\/(like|comments)$/)) && method === 'POST') {
    const v = voomPosts.find(x => x.id === m[1]);
    if (!v) return bad('영상을 찾을 수 없습니다', 404);
    if (m[2] === 'like') {
      const i = v.liked_by.indexOf(myId);
      if (i >= 0) v.liked_by.splice(i, 1); else v.liked_by.push(myId);
      return json(voomView(v, myId));
    }
    const b = await readJson(request);
    const body = clip(b.body, 300);
    if (!body) return bad('댓글을 입력하세요');
    v.comments.push({ id: uid(), author_id: myId, body, created_at: now() });
    return json(voomView(v, myId), 201);
  }

  // ---------- Pay wallet & shop ----------
  if (p === 'wallet' && method === 'GET') {
    const w = walletOf(myId);
    return json({ balance: w.balance, currency: 'KRW', transactions: w.tx.slice(0, 50) });
  }
  if (p === 'wallet/charge' && method === 'POST') {
    const b = await readJson(request);
    const amount = intIn(b.amount, 1000, 1000000);
    if (amount === null) return bad('충전 금액은 1,000원 ~ 1,000,000원입니다');
    const w = walletOf(myId);
    w.balance += amount;
    w.tx.unshift({ id: uid(), type: 'charge', amount, note: '잔액 충전(시뮬레이션)', created_at: now() });
    return json({ balance: w.balance }, 201);
  }
  if (p === 'wallet/send' && method === 'POST') {
    const b = await readJson(request);
    const amount = intIn(b.amount, 1, 500000);
    if (amount === null) return bad('송금 금액은 1원 ~ 500,000원입니다');
    if (!isFriend(myId, b.to_user_id) || fMeta(myId, b.to_user_id).blocked) return bad('친구에게만 송금할 수 있습니다', 403);
    const mine = walletOf(myId);
    if (mine.balance < amount) return bad('잔액이 부족합니다', 402);
    const theirs = walletOf(b.to_user_id);
    const note = clip(b.note || '', 50);
    mine.balance -= amount;
    theirs.balance += amount;
    const at = now();
    mine.tx.unshift({ id: uid(), type: 'send', amount: -amount, counterparty: userBrief(b.to_user_id)?.display_name, note, created_at: at });
    theirs.tx.unshift({ id: uid(), type: 'receive', amount, counterparty: me.display_name, note, created_at: at });
    let conv = findDirect(myId, b.to_user_id);
    if (!conv) conv = createConversation({ kind: 'direct', title: '', members: [myId, b.to_user_id] });
    pushMessage(conv.id, myId, { kind: 'pay', body: note || '송금', metadata: { amount } });
    return json({ balance: mine.balance }, 201);
  }
  if (p === 'store/stickers' && method === 'GET') {
    const owned = packsOf(myId);
    return json({ packs: STICKER_CATALOG.map(pk => ({ ...pk, owned: owned.has(pk.id), currency: 'KRW' })) });
  }
  if (p === 'store/themes' && method === 'GET') {
    const owned = themesOf(myId);
    return json({ themes: THEME_CATALOG.map(t => ({ ...t, owned: owned.has(t.id), currency: 'KRW' })) });
  }
  if (p === 'store/orders' && method === 'POST') {
    const b = await readJson(request);
    const pack = STICKER_CATALOG.find(x => x.id === b.pack_id);
    const theme = THEME_CATALOG.find(x => x.id === b.theme_id);
    const item = pack || theme;
    if (!item) return bad('상품을 찾을 수 없습니다', 404);
    const owned = pack ? packsOf(myId) : themesOf(myId);
    if (owned.has(item.id)) return bad('이미 보유한 상품입니다', 409);
    const w = walletOf(myId);
    if (w.balance < item.price_minor) return bad('잔액이 부족합니다. Pay에서 충전해 주세요', 402);
    w.balance -= item.price_minor;
    owned.add(item.id);
    const orderId = uid();
    storeOrders.set(orderId, { id: orderId, buyer: myId, amount: item.price_minor });
    w.tx.unshift({ id: orderId, type: 'purchase', amount: -item.price_minor, note: item.title, created_at: now() });
    return json({ order_id: orderId, status: 'completed', balance: w.balance }, 201);
  }

  // ---------- OpenChat ----------
  if (p === 'community/rooms') {
    if (method === 'GET') {
      const q = clip(url.searchParams.get('q') || '', 50).toLowerCase();
      const rooms = Array.from(communityRooms.values()).filter(r => !q || r.name.toLowerCase().includes(q) || (r.description || '').toLowerCase().includes(q))
        .map(r => ({ ...r, member_count: (roomMembers.get(r.id) || new Map()).size, joined: (roomMembers.get(r.id) || new Map()).has(myId) }));
      return json({ rooms });
    }
    if (method === 'POST') {
      const b = await readJson(request);
      const name = clip(b.name, 50);
      if (!name) return bad('오픈채팅 이름을 입력하세요');
      const room = { id: uid(), name, description: clip(b.description || '', 200), visibility: 'public', owner_id: myId, created_at: now() };
      communityRooms.set(room.id, room);
      communityPosts.set(room.id, []);
      roomMembers.set(room.id, new Map([[myId, clip(b.nickname || me.display_name, 20) || me.display_name]]));
      return json({ ...room, member_count: 1, joined: true }, 201);
    }
  }
  if ((m = p.match(/^community\/rooms\/([\w-]+)\/(join|leave)$/)) && method === 'POST') {
    const room = communityRooms.get(m[1]);
    if (!room) return bad('오픈채팅을 찾을 수 없습니다', 404);
    if (!roomMembers.has(room.id)) roomMembers.set(room.id, new Map());
    const members = roomMembers.get(room.id);
    if (m[2] === 'join') {
      const b = await readJson(request);
      members.set(myId, clip(b.nickname || me.display_name, 20) || me.display_name);
      return json({ status: 'joined', nickname: members.get(myId), member_count: members.size }, 201);
    }
    members.delete(myId);
    return json({ status: 'left', member_count: members.size });
  }
  if ((m = p.match(/^community\/rooms\/([\w-]+)\/posts$/))) {
    const room = communityRooms.get(m[1]);
    if (!room) return bad('오픈채팅을 찾을 수 없습니다', 404);
    const members = roomMembers.get(room.id) || new Map();
    const view = x => ({ ...x, liked_by: undefined, likes: (x.liked_by || []).length, liked: (x.liked_by || []).includes(myId) });
    if (method === 'GET') return json({ posts: (communityPosts.get(room.id) || []).slice(0, 200).map(view), joined: members.has(myId), nickname: members.get(myId) || '' });
    if (method === 'POST') {
      if (!members.has(myId)) members.set(myId, clip(me.display_name, 20) || '익명');
      const b = await readJson(request);
      const body = clip(b.body, 1000);
      if (!body) return bad('메시지를 입력하세요');
      const post = { id: uid(), room_id: room.id, author_id: myId, author_name: members.get(myId), body, liked_by: [], created_at: now() };
      const list = communityPosts.get(room.id) || [];
      list.unshift(post);
      communityPosts.set(room.id, list);
      sendTo(Array.from(members.keys()), { type: 'community.post', room_id: room.id, post: view(post) });
      return json(view(post), 201);
    }
  }
  if ((m = p.match(/^community\/rooms\/([\w-]+)\/posts\/([\w-]+)\/like$/)) && method === 'POST') {
    const members = roomMembers.get(m[1]) || new Map();
    if (!members.has(myId)) members.set(myId, clip(me.display_name, 20) || '익명');
    const post = (communityPosts.get(m[1]) || []).find(x => x.id === m[2]);
    if (!post) return bad('게시물을 찾을 수 없습니다', 404);
    post.liked_by = post.liked_by || [];
    const i = post.liked_by.indexOf(myId);
    if (i >= 0) post.liked_by.splice(i, 1); else post.liked_by.push(myId);
    sendTo(Array.from(members.keys()), { type: 'community.post_liked', room_id: m[1], post_id: post.id, likes: post.liked_by.length });
    return json({ status: i >= 0 ? 'unliked' : 'liked', likes: post.liked_by.length });
  }

  // ---------- Search & reports ----------
  if (p === 'search' && method === 'GET') {
    const q = clip(url.searchParams.get('q') || '', 100).toLowerCase();
    if (q.length < 2) return json({ messages: [], users: [], chats: [], official_accounts: [] });
    const mine = Array.from(conversations.values()).filter(c => c.members.includes(myId));
    const hs = hiddenMsgs.get(myId);
    const foundMsgs = [];
    for (const c of mine) {
      const title = chatSummary(c, myId).title;
      for (const x of messages.get(c.id) || []) {
        if (x.unsent || x.kind === 'sticker' || x.kind === 'system' || (hs && hs.has(x.id))) continue;
        if (String(x.body).toLowerCase().includes(q)) foundMsgs.push({ ...x, conversation_title: title });
      }
    }
    const friendsFound = friendIds(myId).map(id => usersById.get(id)).filter(u => u && (u.display_name.toLowerCase().includes(q) || u.handle === q)).map(publicUser);
    const chatsFound = mine.map(c => chatSummary(c, myId)).filter(c => (c.title || '').toLowerCase().includes(q));
    const oaFound = Array.from(officialAccounts.values()).filter(o => o.name.toLowerCase().includes(q) || o.handle.includes(q));
    return json({ messages: foundMsgs.slice(-30).reverse(), users: friendsFound, chats: chatsFound, official_accounts: oaFound });
  }
  if (p === 'reports' && method === 'POST') {
    const b = await readJson(request);
    const category = clip(b.category, 80);
    if (!usersById.has(b.subject_user_id) || category.length < 3) return bad('신고 정보가 올바르지 않습니다');
    const rep = { id: uid(), reporter_id: myId, subject_user_id: b.subject_user_id, category, details: clip(b.details || '', 2000), state: 'open', created_at: now() };
    adminReports.unshift(rep);
    return json({ id: rep.id }, 201);
  }

  // ---------- Admin ----------
  if (p.startsWith('admin/')) {
    if (me.role !== 'admin') return bad('forbidden', 403);
    if (p === 'admin/stats' && method === 'GET') {
      return json({ metrics: {
        users: users.size, active_sessions: sessions.size, conversations: conversations.size,
        messages: Array.from(messages.values()).reduce((n, l) => n + l.length, 0),
        open_reports: adminReports.filter(r => r.state === 'open').length, calls: callSessions.size, orders: storeOrders.size
      } });
    }
    if (p === 'admin/reports' && method === 'GET') return json({ reports: adminReports.slice(0, 100) });
    if (p === 'admin/audit-logs' && method === 'GET') return json({ audit_logs: auditLogs.slice(0, 100) });
    if (p === 'admin/feature-flags' && method === 'PUT') {
      const b = await readJson(request);
      const key = clip(b.key, 60);
      if (!/^[a-z0-9_]{3,60}$/.test(key) || typeof b.enabled !== 'boolean') return bad('플래그 키(영문 소문자·숫자·_)와 enabled 값이 필요합니다');
      const target = featureFlags.find(f => f.key === key);
      if (target) target.enabled = b.enabled; else featureFlags.push({ key, enabled: b.enabled, description: '' });
      addAudit(myId, `feature_flag.${b.enabled ? 'enable' : 'disable'}`, key);
      return json({ status: 'updated', feature_flags: featureFlags });
    }
    if (p === 'admin/moderation/actions' && method === 'POST') {
      const b = await readJson(request);
      const uidTarget = b.subject_user_id || b.user_id;
      const target = usersById.get(uidTarget);
      if (!target || !['warn', 'suspend', 'activate', 'dismiss'].includes(b.action)) return bad('잘못된 제재 요청입니다');
      if (target.role === 'admin') return bad('관리자 계정에는 제재를 적용할 수 없습니다', 403);
      if (b.action === 'suspend') {
        target.state = 'suspended';
        for (const [tok, s] of Array.from(sessions)) if (s.user_id === target.id) sessions.delete(tok);
      } else if (b.action === 'activate') target.state = 'active';
      if (b.report_id) {
        const rep = adminReports.find(r => r.id === b.report_id);
        if (rep) rep.state = b.action === 'dismiss' ? 'dismissed' : 'resolved';
      }
      addAudit(myId, `moderation.${b.action}`, target.id);
      return json({ status: 'action_applied', user_id: target.id, action: b.action });
    }
  }

  return null;
}

async function leaveConversation(conv, myId, me) {
  conv.members = conv.members.filter(x => x !== myId);
  cMeta(conv.id, myId).hidden = true;
  if (!conv.members.length) {
    conversations.delete(conv.id);
    return json({ status: 'left' });
  }
  if (conv.owner_id === myId) conv.owner_id = conv.members[0];
  systemMessage(conv.id, `${me.display_name}님이 나갔습니다.`);
  sendTo([...conv.members, myId], { type: 'conversation.updated', conversation_id: conv.id });
  return json({ status: 'left' });
}

let booted = null;
function boot() {
  if (!booted) {
    booted = (async () => {
      ensureSeed();
      seedExtras();
      await hashPendingPasswords();
    })();
  }
  return booted;
}
const loginFailures = new Map();

export default {
  async fetch(request, env, ctx) {
    await boot();
    const url = new URL(request.url);

    // Handle CORS preflight
    if (request.method === 'OPTIONS') {
      return new Response(null, {
        status: 204,
        headers: {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Headers': 'Authorization, Content-Type',
          'Access-Control-Allow-Methods': 'GET, POST, PUT, PATCH, DELETE, OPTIONS'
        }
      });
    }

    // Health / Readyz / Metrics
    if (url.pathname === '/healthz' || url.pathname === '/readyz' || url.pathname === '/api/v1/health') {
      return json({ status: 'ok', runtime: 'cloudflare-worker' });
    }
    if (url.pathname === '/metrics') {
      return new Response('# HELP cf_edge_requests_total\ncf_edge_requests_total 1\n', {
        headers: { 'Content-Type': 'text/plain' }
      });
    }

    // WebSocket real-time connection upgrade
    if (url.pathname === '/api/v1/ws') {
      const upgradeHeader = request.headers.get('Upgrade');
      if (!upgradeHeader || upgradeHeader.toLowerCase() !== 'websocket') {
        return new Response('Expected Upgrade: websocket', { status: 426 });
      }
      const ticket = url.searchParams.get('ticket');
      const grant = wsTickets.get(ticket);
      wsTickets.delete(ticket);
      if (!grant || grant.expiresAt <= Date.now()) {
        return json({ error: 'invalid or expired websocket ticket' }, 401);
      }
      const pair = new WebSocketPair();
      const [client, server] = Object.values(pair);
      server.accept();
      activeSockets.set(server, grant.userId);
      server.addEventListener('message', event => {
        try {
          const d = JSON.parse(event.data);
          if (d.type === 'ping') {
            server.send(JSON.stringify({ type: 'pong' }));
          }
        } catch {}
      });
      server.addEventListener('close', () => {
        activeSockets.delete(server);
      });
      return new Response(null, { status: 101, webSocket: client });
    }

    // If API route
    if (url.pathname.startsWith('/api/v1/')) {
      const p = url.pathname.replace('/api/v1/', '');

      // 1. Auth: Register
      if (p === 'auth/register' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const email = (body.email || '').toLowerCase().trim();
        if (!email || !body.password) return json({ error: 'invalid email or password' }, 400);
        if (users.has(email)) return json({ error: 'account already exists' }, 409);

        const newUser = {
          id: crypto.randomUUID(),
          email,
          password: body.password,
          display_name: body.display_name || email.split('@')[0],
          role: 'user',
          handle: body.handle || `user_${crypto.randomUUID().slice(0, 6)}`,
          bio: 'New Messenger User',
          created_at: new Date().toISOString()
        };
        users.set(email, newUser);
        usersById.set(newUser.id, newUser);
        return json({ id: newUser.id, email: newUser.email, display_name: newUser.display_name }, 201);
      }

      // 2. Auth: Login
      if (p === 'auth/login' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const identifier = (body.email || body.email_or_phone || body.username || '').toLowerCase().trim();
        const user = users.get(identifier) || Array.from(users.values()).find(u =>
          (u.email && u.email.toLowerCase() === identifier) ||
          (u.handle && u.handle.toLowerCase() === identifier) ||
          (u.phone && u.phone === identifier)
        );
        if (!user) {
          return json({ error: 'invalid credentials' }, 401);
        }
        const valid = (user.password && user.password === body.password) ||
                      (user.pw_hash && await checkPassword(user, body.password));
        if (!valid) {
          return json({ error: 'invalid credentials' }, 401);
        }
        const token = `cf-token-${crypto.randomUUID()}`;
        const deviceId = crypto.randomUUID();
        sessions.set(token, { user_id: user.id, role: user.role, device_id: deviceId });
        return json({
          access_token: token,
          refresh_token: `cf-refresh-${crypto.randomUUID()}`,
          token_type: 'Bearer',
          expires_in: 86400,
          user_id: user.id,
          role: user.role,
          device_id: deviceId
        });
      }

      // 3. Auth: QR Login
      if (p === 'auth/qr/challenge' && request.method === 'POST') {
        const challId = crypto.randomUUID();
        const qrToken = `qr-tok-${crypto.randomUUID()}`;
        qrChallenges.set(challId, { token: qrToken, status: 'pending', user_id: null });
        return json({ challenge_id: challId, qr_token: qrToken, expires_in: 180 }, 201);
      }
      if (p === 'auth/qr/approve' && request.method === 'POST') {
        const authUser = getAuthUser(request);
        if (!authUser) return json({ error: 'unauthorized' }, 401);
        const body = await request.json().catch(() => ({}));
        const c = qrChallenges.get(body.challenge_id);
        if (!c || c.token !== body.qr_token) return json({ error: 'invalid qr challenge' }, 400);
        c.status = 'authenticated';
        c.user_id = authUser.id;
        return json({ status: 'approved' });
      }
      if (p === 'auth/qr/poll' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const c = qrChallenges.get(body.challenge_id);
        if (!c || c.token !== body.qr_token) return json({ error: 'invalid qr challenge' }, 401);
        if (c.status === 'authenticated') {
          const u = Array.from(users.values()).find(usr => usr.id === c.user_id);
          const token = `cf-token-${crypto.randomUUID()}`;
          sessions.set(token, { user_id: u.id, role: u.role, device_id: crypto.randomUUID() });
          return json({ status: 'authenticated', access_token: token, user_id: u.id, role: u.role });
        }
        return json({ status: 'pending' }, 202);
      }

      // Protected routes check
      const authUser = getAuthUser(request);
      if (!authUser) {
        return json({ error: 'unauthorized' }, 401);
      }

      if (p === 'ws-ticket' && request.method === 'POST') {
        for (const [key, grant] of wsTickets) {
          if (grant.expiresAt <= Date.now()) wsTickets.delete(key);
        }
        const ticket = `wst-${crypto.randomUUID()}`;
        wsTickets.set(ticket, { userId: authUser.id, expiresAt: Date.now() + 60000 });
        return json({ ticket, expires_in: 60 });
      }

      // All conversation subresources share the same membership boundary.
      const conversationPath = p.match(/^conversations\/([^/]+)(?:\/|$)/);
      if (conversationPath) {
        const conversation = conversations.get(conversationPath[1]);
        if (!conversation || !conversation.members.includes(authUser.id)) {
          return json({ error: 'conversation not found' }, 404);
        }
      }
      const callPath = p.match(/^calls\/([^/]+)(?:\/|$)/);
      if (callPath) {
        const call = callSessions.get(callPath[1]);
        if (!call || !conversations.get(call.conversation_id)?.members.includes(authUser.id)) {
          return json({ error: 'call not found' }, 404);
        }
      }

      const extResp = await extended(p, request, url, authUser, request.headers.get('Authorization')?.slice(7)?.trim());
      if (extResp) return extResp;

      // 4. Me & Settings
      if (p === 'me') {
        if (request.method === 'GET') return json(authUser);
        if (request.method === 'PATCH') {
          const body = await request.json().catch(() => ({}));
          if (body.bio !== undefined) authUser.bio = body.bio;
          if (body.display_name) authUser.display_name = body.display_name;
          return json(authUser);
        }
      }
      if (p === 'me/settings') {
        return json({
          searchable_by_email: true,
          searchable_by_phone: true,
          show_read_receipts: true,
          allow_friend_requests: true,
          theme: 'dark'
        });
      }

      // 5. Friends & Users
      if (p === 'friends') {
        if (request.method === 'GET') {
          const flist = [];
          for (const [key] of friendships) {
            const [u1, u2] = key.split(':');
            if (u1 === authUser.id) {
              const f = Array.from(users.values()).find(u => u.id === u2);
              if (f) flist.push({ id: f.id, display_name: f.display_name, handle: f.handle, bio: f.bio });
            }
          }
          return json({ friends: flist });
        }
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const targetId = body.friend_id || body.target_user_id;
          friendships.set(`${authUser.id}:${targetId}`, true);
          friendships.set(`${targetId}:${authUser.id}`, true);
          return json({ status: 'friend_added' }, 201);
        }
      }
      if (p === 'users' && request.method === 'GET') {
        const q = (url.searchParams.get('q') || url.searchParams.get('search') || '').toLowerCase();
        const found = Array.from(users.values())
          .filter(u => u.id !== authUser.id && (!q || u.display_name.toLowerCase().includes(q) || (u.email && u.email.includes(q))))
          .map(u => ({ id: u.id, display_name: u.display_name, handle: u.handle, bio: u.bio }));
        return json({ users: found });
      }

      // 6. Conversations
      if (p === 'conversations') {
        if (request.method === 'GET') {
          const list = Array.from(conversations.values())
            .filter(c => c.members.includes(authUser.id))
            .map(c => ({
              id: c.id,
              kind: c.kind,
              title: c.title,
              last_sequence: c.last_sequence || 0,
              unread_count: 0,
              updated_at: c.updated_at
            }));
          return json({ conversations: list });
        }
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const cid = crypto.randomUUID();
          const members = Array.from(new Set([authUser.id, ...(body.member_ids || [])]));
          const newConv = {
            id: cid,
            kind: body.kind || 'direct',
            title: body.title || 'Conversation',
            members,
            last_sequence: 0,
            updated_at: new Date().toISOString()
          };
          conversations.set(cid, newConv);
          messages.set(cid, []);
          return json(newConv, 201);
        }
      }

      // Messages CRUD & WebSocket Broadcast
      const convMsgMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/messages$/);
      if (convMsgMatch && request.method === 'POST') {
        const cid = convMsgMatch[1];
        const body = await request.json().catch(() => ({}));
        const conv = conversations.get(cid);
        if (!conv) return json({ error: 'not found' }, 404);

        conv.last_sequence = (conv.last_sequence || 0) + 1;
        conv.updated_at = new Date().toISOString();
        const newMsg = {
          id: crypto.randomUUID(),
          conversation_id: cid,
          sender_id: authUser.id,
          client_id: body.client_id || crypto.randomUUID(),
          sequence: conv.last_sequence,
          kind: body.kind || 'text',
          body: body.body || '',
          reply_to: body.reply_to || undefined,
          metadata: body.metadata || undefined,
          created_at: new Date().toISOString()
        };
        const mlist = messages.get(cid) || [];
        mlist.push(newMsg);
        messages.set(cid, mlist);

        broadcast({
          type: 'message.created',
          conversation_id: cid,
          message: newMsg
        });

        return json({ message: newMsg, duplicate: false }, 201);
      }

      const convMsgDetail = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/messages\/([a-zA-Z0-9_-]+)$/);
      if (convMsgDetail) {
        const cid = convMsgDetail[1];
        const mid = convMsgDetail[2];
        const mlist = messages.get(cid) || [];
        const msg = mlist.find(m => m.id === mid);
        if (!msg) return json({ error: 'message not found' }, 404);
        if (['PATCH', 'DELETE'].includes(request.method) && msg.sender_id !== authUser.id) {
          return json({ error: 'only the sender can modify a message' }, 403);
        }

        if (request.method === 'PATCH') {
          const body = await request.json().catch(() => ({}));
          msg.body = body.body || msg.body;
          msg.edited_at = new Date().toISOString();
          broadcast({ type: 'message.updated', conversation_id: cid, message_id: mid, body: msg.body });
          return json({ status: 'edited', message: msg });
        }
        if (request.method === 'DELETE') {
          const idx = mlist.findIndex(m => m.id === mid);
          if (idx !== -1) mlist.splice(idx, 1);
          broadcast({ type: 'message.deleted', conversation_id: cid, message_id: mid });
          return json({ status: 'recalled', message_id: mid });
        }
      }

      const convSyncMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/sync$/);
      if (convSyncMatch && request.method === 'GET') {
        const cid = convSyncMatch[1];
        const after = parseInt(url.searchParams.get('after_sequence') || '0', 10);
        const mlist = (messages.get(cid) || []).filter(m => m.sequence > after);
        return json({ messages: mlist, last_sequence: (conversations.get(cid) || {}).last_sequence || 0 });
      }

      const convReadMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/read$/);
      if (convReadMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        broadcast({
          type: 'conversation.read',
          conversation_id: convReadMatch[1],
          user_id: authUser.id,
          sequence: body.sequence || 1
        });
        return json({ status: 'read' }, 200);
      }

      const convTypingMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/typing$/);
      if (convTypingMatch && request.method === 'POST') {
        broadcast({
          type: 'conversation.typing',
          conversation_id: convTypingMatch[1],
          user_id: authUser.id
        });
        return json({ status: 'typing' });
      }

      const reactMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/messages\/([a-zA-Z0-9_-]+)\/reactions$/);
      if (reactMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        broadcast({
          type: 'message.reaction',
          conversation_id: reactMatch[1],
          message_id: reactMatch[2],
          reaction: body.reaction || '👍',
          user_id: authUser.id
        });
        return json({ status: 'reacted' }, 201);
      }

      // Pinned messages
      const pinsMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/pins$/);
      if (pinsMatch) {
        const cid = pinsMatch[1];
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const pinList = pinnedMessages.get(cid) || [];
          pinList.unshift({ message_id: body.message_id, pinned_by: authUser.id, pinned_at: new Date().toISOString() });
          pinnedMessages.set(cid, pinList);
          return json({ status: 'pinned', message_id: body.message_id }, 201);
        }
        if (request.method === 'GET') {
          return json({ pins: pinnedMessages.get(cid) || [] });
        }
      }

      // Polls
      const pollMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/polls$/);
      if (pollMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const pollId = crypto.randomUUID();
        const poll = {
          id: pollId,
          conversation_id: pollMatch[1],
          question: body.question,
          options: (body.options || []).map((o, idx) => ({ id: `opt-${idx}`, text: o, votes: 0 })),
          created_by: authUser.id,
          created_at: new Date().toISOString()
        };
        polls.set(pollId, poll);
        return json(poll, 201);
      }
      const pollVoteMatch = p.match(/^polls\/([a-zA-Z0-9_-]+)\/vote$/);
      if (pollVoteMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const poll = polls.get(pollVoteMatch[1]);
        if (!poll || !conversations.get(poll.conversation_id)?.members.includes(authUser.id)) {
          return json({ error: 'poll not found' }, 404);
        }
        const opt = poll.options.find(o => o.id === body.option_id);
        if (opt) opt.votes += 1;
        return json({ status: 'voted', poll });
      }

      // Albums
      const albumMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/albums$/);
      if (albumMatch) {
        const cid = albumMatch[1];
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const aid = crypto.randomUUID();
          const alist = albums.get(cid) || [];
          const alb = { id: aid, title: body.title || 'Album', image_url: body.image_url || '', created_at: new Date().toISOString() };
          alist.push(alb);
          albums.set(cid, alist);
          return json(alb, 201);
        }
        if (request.method === 'GET') {
          return json({ albums: albums.get(cid) || [] });
        }
      }

      // Saved Messages (Keep)
      if (p === 'saved-messages') {
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const sid = crypto.randomUUID();
          const slist = savedMessages.get(authUser.id) || [];
          const smsg = { id: sid, body: body.body || '', kind: body.kind || 'text', created_at: new Date().toISOString() };
          slist.unshift(smsg);
          savedMessages.set(authUser.id, slist);
          return json(smsg, 201);
        }
        if (request.method === 'GET') {
          return json({ saved_messages: savedMessages.get(authUser.id) || [] });
        }
      }
      const savedDelMatch = p.match(/^saved-messages\/([a-zA-Z0-9_-]+)$/);
      if (savedDelMatch && request.method === 'DELETE') {
        const sid = savedDelMatch[1];
        let slist = savedMessages.get(authUser.id) || [];
        slist = slist.filter(item => item.id !== sid && item.message_id !== sid);
        savedMessages.set(authUser.id, slist);
        return json({ status: 'deleted', id: sid });
      }

      // 7. Calling & Signaling
      if (p === 'calls' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        if (!conversations.get(body.conversation_id)?.members.includes(authUser.id)) {
          return json({ error: 'conversation not found' }, 404);
        }
        const callId = crypto.randomUUID();
        callSessions.set(callId, { id: callId, conversation_id: body.conversation_id, kind: body.kind || 'video', state: 'ringing' });
        broadcast({
          type: 'call.ringing',
          call_id: callId,
          conversation_id: body.conversation_id,
          initiated_by: authUser.id,
          kind: body.kind || 'video'
        });
        return json({ id: callId, state: 'ringing', kind: body.kind || 'video' }, 201);
      }
      const callSigMatch = p.match(/^calls\/([a-zA-Z0-9_-]+)\/signal$/);
      if (callSigMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        broadcast({
          type: 'call.signal',
          call_id: callSigMatch[1],
          from_user_id: authUser.id,
          signal_type: body.signal_type,
          sdp: body.sdp
        });
        return json({ status: 'signaled' });
      }
      const callStateMatch = p.match(/^calls\/([a-zA-Z0-9_-]+)\/state$/);
      if (callStateMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        broadcast({
          type: 'call.state',
          call_id: callStateMatch[1],
          state: body.state
        });
        return json({ status: body.state });
      }
      const callQualMatch = p.match(/^calls\/([a-zA-Z0-9_-]+)\/quality$/);
      if (callQualMatch && request.method === 'POST') {
        return json({ status: 'recorded' }, 201);
      }

      // 8. Attachments
      if (p === 'attachments' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const attId = crypto.randomUUID();
        return json({ id: attId, status: 'ready', original_name: body.original_name || 'file' }, 201);
      }

      // 9. Store Orders & Stickers
      if (p === 'store/stickers') {
        return json({
          packs: [
            { id: 'pack-1', title: 'Line Friends Special Edition', price_minor: 2500, currency: 'KRW' },
            { id: 'pack-2', title: 'Brown & Cony Daily Life', price_minor: 2500, currency: 'KRW' }
          ]
        });
      }
      if (p === 'store/orders' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const orderId = crypto.randomUUID();
        storeOrders.set(orderId, { id: orderId, buyer: authUser.id, amount: body.price_minor || 1000 });
        return json({ order_id: orderId, status: 'completed' }, 201);
      }

      // 10. Community / OpenChat
      if (p === 'community/rooms') {
        if (request.method === 'GET') {
          return json({ rooms: Array.from(communityRooms.values()) });
        }
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const rid = crypto.randomUUID();
          const room = { id: rid, name: body.name || 'OpenChat', description: body.description || '', visibility: 'public', created_at: new Date().toISOString() };
          communityRooms.set(rid, room);
          communityPosts.set(rid, []);
          return json(room, 201);
        }
      }
      const roomPostMatch = p.match(/^community\/rooms\/([a-zA-Z0-9_-]+)\/posts$/);
      if (roomPostMatch) {
        const rid = roomPostMatch[1];
        if (request.method === 'GET') {
          return json({ posts: communityPosts.get(rid) || [] });
        }
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const pid = crypto.randomUUID();
          const post = { id: pid, room_id: rid, author_name: authUser.display_name, body: body.body || '', created_at: new Date().toISOString() };
          const plist = communityPosts.get(rid) || [];
          plist.unshift(post);
          communityPosts.set(rid, plist);
          return json(post, 201);
        }
      }
      const roomPostLikeMatch = p.match(/^community\/rooms\/([a-zA-Z0-9_-]+)\/posts\/([a-zA-Z0-9_-]+)\/like$/);
      if (roomPostLikeMatch && request.method === 'POST') {
        const rid = roomPostLikeMatch[1];
        const pid = roomPostLikeMatch[2];
        const plist = communityPosts.get(rid) || [];
        const post = plist.find(item => item.id === pid);
        if (post) {
          post.likes = (post.likes || 0) + 1;
          broadcast({ type: 'community.post_liked', room_id: rid, post_id: pid, likes: post.likes });
          return json({ status: 'liked', likes: post.likes });
        }
        return json({ error: 'not found' }, 404);
      }

      // 11. Partner Console
      if (p === 'partner/organizations') {
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const orgId = crypto.randomUUID();
          const org = { id: orgId, name: body.name || 'Partner Org', country: body.country || 'KR' };
          partnerOrgs.set(orgId, org);
          const accId = crypto.randomUUID();
          partnerAccounts.set(accId, { id: accId, org_id: orgId, name: `${org.name} Official`, status: 'active' });
          return json(org, 201);
        }
        return json({ organizations: Array.from(partnerOrgs.values()) });
      }
      const partnerTickMatch = p.match(/^partner\/accounts\/([a-zA-Z0-9_-]+)\/tickets$/);
      if (partnerTickMatch) {
        const aid = partnerTickMatch[1];
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const tid = crypto.randomUUID();
          const t = { id: tid, account_id: aid, title: body.title || 'Inquiry', priority: body.priority || 'medium', state: 'open' };
          const tlist = partnerTickets.get(aid) || [];
          tlist.push(t);
          partnerTickets.set(aid, tlist);
          return json(t, 201);
        }
        return json({ tickets: partnerTickets.get(aid) || [] });
      }
      const partnerCampMatch = p.match(/^partner\/accounts\/([a-zA-Z0-9_-]+)\/campaigns$/);
      if (partnerCampMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const cid = crypto.randomUUID();
        const camp = { id: cid, title: body.title || 'Campaign', status: 'scheduled' };
        return json(camp, 201);
      }

      // 12. Developer Console
      if (p === 'developer/organizations') {
        return json({ organizations: [{ id: 'dev-org-1', name: 'LINE Dev Team' }] });
      }
      if (p === 'developer/apps') {
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const app = { id: crypto.randomUUID(), name: body.name || 'Bot App', client_id: `cli_${crypto.randomUUID().slice(0, 8)}`, client_secret: crypto.randomUUID() };
          devApps.set(app.id, app);
          return json(app, 201);
        }
        return json({ apps: Array.from(devApps.values()) });
      }
      const devWebMatch = p.match(/^developer\/apps\/([a-zA-Z0-9_-]+)\/webhooks$/);
      if (devWebMatch && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        return json({ id: crypto.randomUUID(), endpoint: body.endpoint_url, active: true }, 201);
      }

      // 13. Creator Console
      if (p === 'creator/profile') {
        return json({ id: 'creator-1', display_name: authUser.display_name, verified: true });
      }
      if (p === 'creator/packs') {
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const pack = { id: crypto.randomUUID(), title: body.title || 'Stickers', price_minor: body.price_minor || 2500 };
          creatorPacks.set(pack.id, pack);
          return json(pack, 201);
        }
        return json({ packs: Array.from(creatorPacks.values()) });
      }
      if (p === 'creator/settlements') {
        return json({ settlements: [{ id: 'stl-1', total_sales: 125000, revenue_share: 87500, currency: 'KRW', status: 'settled' }] });
      }

      // 14. Ads Console
      if (p === 'ads/advertisers') {
        return json({ advertisers: [{ id: 'adv-1', company_name: 'LINE Global Ads', status: 'verified' }] });
      }
      if (p === 'ads/campaigns') {
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const camp = { id: crypto.randomUUID(), name: body.name || 'Ad Campaign', daily_budget_minor: 50000 };
          adsCampaigns.set(camp.id, camp);
          return json(camp, 201);
        }
        return json({ campaigns: Array.from(adsCampaigns.values()) });
      }

      // 15. Admin & Security
      if (p === 'admin/feature-flags') {
        if (authUser.role !== 'admin') return json({ error: 'forbidden' }, 403);
        if (request.method === 'PUT') {
          const body = await request.json().catch(() => ({}));
          const target = featureFlags.find(f => f.key === body.key);
          if (target) target.enabled = body.enabled;
          return json({ status: 'updated', feature_flags: featureFlags });
        }
        return json({ feature_flags: featureFlags });
      }
      if (p === 'admin/stats') {
        if (authUser.role !== 'admin') return json({ error: 'forbidden' }, 403);
        return json({ total_users: users.size, active_sessions: sessions.size, total_conversations: conversations.size });
      }
      if (p === 'admin/users') {
        if (authUser.role !== 'admin') return json({ error: 'forbidden' }, 403);
        return json({ users: Array.from(users.values()).map(u => ({ id: u.id, email: u.email, display_name: u.display_name, role: u.role, state: u.state || 'active' })) });
      }
      if (p === 'admin/moderation/actions' && request.method === 'POST') {
        if (authUser.role !== 'admin') return json({ error: 'forbidden' }, 403);
        const body = await request.json().catch(() => ({}));
        const uid = body.subject_user_id || body.user_id;
        const target = Array.from(users.values()).find(u => u.id === uid);
        if (target) {
          if (body.action === 'suspend') target.state = 'suspended';
          else if (body.action === 'activate') target.state = 'active';
        }
        if (body.report_id) {
          const rep = adminReports.find(r => r.id === body.report_id);
          if (rep) rep.state = (body.action === 'dismiss' ? 'dismissed' : 'resolved');
        }
        auditLogs.unshift({ action: body.action, user_id: uid, actor: authUser.id, created_at: new Date().toISOString() });
        return json({ status: 'action_applied', user_id: uid, action: body.action });
      }

      // Search
      if (p === 'search' && request.method === 'GET') {
        const q = (url.searchParams.get('q') || '').toLowerCase();
        const foundMsgs = [];
        for (const [cid, mlist] of messages) {
          for (const m of mlist) {
            if (m.body.toLowerCase().includes(q)) foundMsgs.push(m);
          }
        }
        return json({ messages: foundMsgs, users: [] });
      }

      return json({ status: 'ok', endpoint: p });
    }

    // Sub-console route redirects
    for (const sub of ['admin', 'partner', 'developer', 'creator', 'ads']) {
      if (url.pathname === `/${sub}`) {
        return Response.redirect(`${url.origin}/${sub}/`, 301);
      }
    }

    // Static Assets fallback (Vite distribution)
    if (env.ASSETS) {
      return env.ASSETS.fetch(request);
    }

    return new Response('Cloudflare Edge Worker Ready', { status: 200 });
  }
};
