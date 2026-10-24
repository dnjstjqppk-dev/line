/**
 * Cloudflare Worker Edge Gateway & Backend for LINE Messenger Monorepo.
 * Serves static assets for frontend apps and handles full /api/v1 suite directly at the edge.
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
const partnerOrgs = new Map();
const devOrgs = new Map();
const featureFlags = [
  { key: 'e2ee_letter_sealing', enabled: true, description: 'Signal Letter-Sealing End-to-End Encryption' },
  { key: 'webrtc_group_calling', enabled: true, description: 'Multi-party Voice & Video Calling' },
  { key: 'ai_smart_replies', enabled: true, description: 'AI-assisted quick responses' },
  { key: 'openchat_live_streaming', enabled: false, description: 'Public community voice stages' }
];

// Helper: Seed default accounts if empty
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
  return Array.from(users.values()).find(u => u.id === session.user_id) || null;
}

export default {
  async fetch(request, env, ctx) {
    ensureSeed();
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
    if (url.pathname === '/healthz' || url.pathname === '/readyz') {
      return json({ status: 'ok', runtime: 'cloudflare-worker' });
    }
    if (url.pathname === '/metrics') {
      return new Response('# HELP cf_edge_requests_total\ncf_edge_requests_total 1\n', {
        headers: { 'Content-Type': 'text/plain' }
      });
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
        return json({ id: newUser.id, email: newUser.email, display_name: newUser.display_name }, 201);
      }

      // 2. Auth: Login
      if (p === 'auth/login' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const email = (body.email || '').toLowerCase().trim();
        const user = users.get(email);
        if (!user || user.password !== body.password) {
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

      // 4. Me & Settings
      if (p === 'me') {
        if (request.method === 'GET') {
          return json(authUser);
        }
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
        const q = (url.searchParams.get('q') || '').toLowerCase();
        const found = Array.from(users.values())
          .filter(u => u.id !== authUser.id && (!q || u.display_name.toLowerCase().includes(q) || (u.email && u.email.includes(q))))
          .map(u => ({ id: u.id, display_name: u.display_name, handle: u.handle, bio: u.bio }));
        return json({ users: found });
      }

      // 6. Conversations & Messages
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

      // Conversation detail match
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
          created_at: new Date().toISOString()
        };
        const mlist = messages.get(cid) || [];
        mlist.push(newMsg);
        messages.set(cid, mlist);
        return json({ message: newMsg, duplicate: false }, 201);
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
        return json({ status: 'read' }, 200);
      }

      const reactMatch = p.match(/^conversations\/([a-zA-Z0-9_-]+)\/messages\/([a-zA-Z0-9_-]+)\/reactions$/);
      if (reactMatch && request.method === 'POST') {
        return json({ status: 'reacted' }, 201);
      }

      // 7. Calling & Signaling
      if (p === 'calls' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const callId = crypto.randomUUID();
        callSessions.set(callId, { id: callId, conversation_id: body.conversation_id, kind: body.kind || 'video', state: 'ringing' });
        return json({ id: callId, state: 'ringing', kind: body.kind || 'video' }, 201);
      }
      const callSigMatch = p.match(/^calls\/([a-zA-Z0-9_-]+)\/signal$/);
      if (callSigMatch && request.method === 'POST') {
        return json({ status: 'signaled' });
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
        return json({ packs: [{ id: 'pack-1', title: 'Line Friends Vol.1', price_minor: 2500, currency: 'KRW' }] });
      }
      if (p === 'store/orders' && request.method === 'POST') {
        const body = await request.json().catch(() => ({}));
        const orderId = crypto.randomUUID();
        storeOrders.set(orderId, { id: orderId, buyer: authUser.id, amount: body.price_minor || 1000 });
        return json({ order_id: orderId, status: 'completed' }, 201);
      }

      // 10. Partner & Consoles
      if (p === 'partner/organizations') {
        if (request.method === 'POST') {
          const body = await request.json().catch(() => ({}));
          const orgId = crypto.randomUUID();
          partnerOrgs.set(orgId, { id: orgId, name: body.name || 'Org' });
          return json({ id: orgId, status: 'created' }, 201);
        }
        return json({ organizations: Array.from(partnerOrgs.values()) });
      }

      // 11. Admin & Security
      if (p === 'admin/feature-flags') {
        if (authUser.role !== 'admin') return json({ error: 'forbidden' }, 403);
        return json({ feature_flags: featureFlags });
      }
      if (p === 'admin/stats') {
        if (authUser.role !== 'admin') return json({ error: 'forbidden' }, 403);
        return json({ total_users: users.size, active_sessions: sessions.size, total_conversations: conversations.size });
      }
      if (p === 'admin/users') {
        if (authUser.role !== 'admin') return json({ error: 'forbidden' }, 403);
        return json({ users: Array.from(users.values()).map(u => ({ id: u.id, email: u.email, display_name: u.display_name, role: u.role })) });
      }

      // Default mock pass for other known endpoints
      return json({ status: 'ok', endpoint: p });
    }

    // Static Assets fallback (Vite distribution)
    if (env.ASSETS) {
      return env.ASSETS.fetch(request);
    }

    return new Response('Cloudflare Edge Worker Ready', { status: 200 });
  }
};
