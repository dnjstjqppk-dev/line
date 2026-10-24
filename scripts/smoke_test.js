#!/usr/bin/env node
/**
 * Comprehensive HTTP integration smoke test against a running local API.
 * Uses native Node.js fetch (Node 18+ / 24).
 * Run against a live API server: node scripts/smoke_test.js
 */

const BASE = (process.env.API_URL || 'http://localhost:8080').replace(/\/+$/, '');
const crypto = require('crypto');

async function call(method, path, body = null, token = null) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const opts = { method, headers };
  if (body !== null) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`${BASE}${path}`, opts);
  const ct = res.headers.get('content-type') || '';
  let data;
  if (ct.includes('application/json')) {
    data = await res.json();
  } else {
    data = await res.text();
  }
  return { status: res.status, data };
}

function expect(actual, expected, where) {
  if (actual !== expected) {
    throw new Error(`${where}: expected ${expected}, got ${actual}`);
  }
}

async function main() {
  console.log(`Starting smoke test against ${BASE}...`);

  // 1. Healthz / Readyz / Metrics
  let res = await call('GET', '/readyz');
  expect(res.status, 200, 'readyz');
  res = await call('GET', '/metrics');
  expect(res.status, 200, 'metrics');

  const suffix = crypto.randomBytes(4).toString('hex');
  const users = [];

  // 2. Multi-user Registration & Login
  for (let i = 0; i < 2; i++) {
    const email = `smoke-${suffix}-${i}@example.test`;
    const payload = {
      email,
      password: 'StrongPassword#123!',
      display_name: `Smoke User ${i}`,
      phone_e164: `+8210${suffix}${i}`,
      handle: `smk_${suffix}_${i}`
    };
    res = await call('POST', '/api/v1/auth/register', payload);
    expect(res.status, 201, `register-${i}`);
    const userId = res.data.id;

    res = await call('POST', '/api/v1/auth/login', {
      email,
      password: payload.password,
      device_name: `Smoke Device ${i}`,
      platform: 'web'
    });
    expect(res.status, 200, `login-${i}`);
    users.push({ ...payload, ...res.data, user_id: userId, token: res.data.access_token });
  }

  const u0 = users[0];
  const u1 = users[1];

  // 3. User Settings & Profile
  res = await call('PATCH', '/api/v1/me', { bio: 'Verified Live Monorepo' }, u0.token);
  expect(res.status, 200, 'patch me');
  res = await call('PUT', '/api/v1/me/settings', {
    searchable_by_email: true,
    searchable_by_phone: true,
    show_read_receipts: true,
    allow_friend_requests: true,
    theme: 'dark'
  }, u0.token);
  expect(res.status, 200, 'put settings');

  // 4. Friend Operations
  res = await call('POST', '/api/v1/friends', { friend_id: u1.user_id }, u0.token);
  expect(res.status, 201, 'add friend');
  res = await call('GET', '/api/v1/friends', null, u0.token);
  expect(res.status, 200, 'list friends');

  // 5. Direct Conversation & Messaging
  res = await call('POST', '/api/v1/conversations', {
    kind: 'direct',
    member_ids: [u1.user_id]
  }, u0.token);
  expect(res.status, 201, 'create direct conv');
  const convId = res.data.id;

  const clientMsgId = crypto.randomUUID();
  res = await call('POST', `/api/v1/conversations/${convId}/messages`, {
    client_id: clientMsgId,
    kind: 'text',
    body: 'Live Monorepo integration message!'
  }, u0.token);
  expect(res.status, 201, 'send message');
  const msgId = res.data.message ? res.data.message.id : res.data.id;

  // 6. Read Cursor & Reactions
  res = await call('POST', `/api/v1/conversations/${convId}/read`, { sequence: 1 }, u1.token);
  expect(res.status, 200, 'mark read');

  res = await call('POST', `/api/v1/conversations/${convId}/messages/${msgId}/reactions`, { reaction: '🔥' }, u1.token);
  expect(res.status, 201, 'reaction');

  // 7. QR Login Challenge / Approve / Poll flow
  res = await call('POST', '/api/v1/auth/qr/challenge', {
    device_name: `Alice-PC-${suffix}`,
    platform: 'windows'
  });
  expect(res.status, 201, 'qr challenge');
  const challengeId = res.data.challenge_id;
  const qrToken = res.data.qr_token;

  res = await call('POST', '/api/v1/auth/qr/approve', {
    challenge_id: challengeId,
    qr_token: qrToken
  }, u0.token);
  expect(res.status, 200, 'qr approve');

  res = await call('POST', '/api/v1/auth/qr/poll', {
    challenge_id: challengeId,
    qr_token: qrToken
  });
  expect(res.status, 200, 'qr poll');
  expect(res.data.status, 'authenticated', 'qr status');

  // 8. E2EE Public Key Bundle
  res = await call('PUT', `/api/v1/devices/${u0.device_id}/keys`, {
    identity_public_key: 'YWxpY2UtaWRlbnRpdHktcHVibGljLWtleQ==',
    signed_prekey: 'YWxpY2Utc2lnbmVkLXByZWtleQ==',
    signed_prekey_signature: 'YWxpY2Utc2lnbmF0dXJl',
    one_time_prekeys: [{ key_id: 1, public_key: 'YWxpY2Utb3RwLTE=' }]
  }, u0.token);
  expect(res.status, 200, 'put e2ee keys');

  res = await call('GET', `/api/v1/users/${u0.user_id}/keys`, null, u1.token);
  expect(res.status, 200, 'get user keys');

  // 9. WebRTC Call Signaling
  res = await call('POST', '/api/v1/calls', {
    conversation_id: convId,
    kind: 'video',
    sdp_offer: 'v=0\r\no=...'
  }, u0.token);
  expect(res.status, 201, 'start call');
  const callId = res.data.id;

  res = await call('POST', `/api/v1/calls/${callId}/signal`, {
    signal_type: 'offer',
    sdp: 'v=0\r\no=...'
  }, u0.token);
  expect(res.status, 200, 'call signal offer');

  res = await call('POST', `/api/v1/calls/${callId}/quality`, {
    rtt_ms: 28,
    jitter_ms: 4,
    packet_loss_ratio: 0.001
  }, u0.token);
  expect(res.status, 201, 'call quality');

  // 10. Attachment Upload
  res = await call('POST', '/api/v1/attachments', {
    original_name: 'test-diagram.png',
    mime_type: 'image/png',
    size_bytes: 1024,
    checksum_sha256: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
  }, u0.token);
  expect(res.status, 201, 'upload attachment');

  // 11. Delta Sync
  res = await call('GET', `/api/v1/conversations/${convId}/sync?after_sequence=0`, null, u1.token);
  expect(res.status, 200, 'delta sync');
  expect(Array.isArray(res.data.messages), true, 'sync messages array');

  // 12. Sticker Store Order & Ledger
  res = await call('POST', '/api/v1/store/orders', {
    product_type: 'sticker_pack',
    name: 'Official Sticker Pack',
    price_minor: 2500,
    currency: 'KRW',
    idempotency_key: `order-${suffix}`
  }, u0.token);
  expect(res.status, 201, 'store order');

  // 13. Security RBAC & Business Partner Organization
  res = await call('GET', '/api/v1/admin/feature-flags', null, u0.token);
  expect(res.status, 403, 'admin flags forbidden for normal user');

  res = await call('POST', '/api/v1/partner/organizations', {
    name: 'Smoke Corp',
    legal_name: 'Smoke Corporation Ltd.',
    country: 'KR'
  }, u0.token);
  expect(res.status, 201, 'create partner org');

  console.log('✅ ALL SMOKE TESTS PASSED (14 domain checkpoints verified)!');
}

main().catch(err => {
  console.error('❌ SMOKE TEST FAILED:', err);
  process.exit(1);
});
