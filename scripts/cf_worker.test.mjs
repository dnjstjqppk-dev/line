import { readFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
import test from 'node:test';

const source = await readFile(new URL('./cf_worker.js', import.meta.url), 'utf8');
const { default: worker } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
const nativeResponse = globalThis.Response;
globalThis.Response = class extends nativeResponse {
  constructor(body, init) {
    super(body, init?.status === 101 ? { status: 200 } : init);
    if (init?.status === 101) Object.defineProperty(this, 'status', { value: 101 });
  }
};
const sockets = [];
globalThis.WebSocketPair = class {
  constructor() {
    const server = { events: [], accept() {}, addEventListener() {}, send(data) { this.events.push(JSON.parse(data)); } };
    sockets.push(server);
    this[0] = {};
    this[1] = server;
  }
};
async function api(path, token, method = 'GET', body, extraHeaders = {}) {
  return worker.fetch(new Request(`https://example.test/api/v1/${path}`, {
    method, headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...extraHeaders },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  }), {});
}
async function login(email, password) {
  return (await (await api('auth/login', null, 'POST', { email, password })).json()).access_token;
}

test('private chat access and one-use realtime tickets', async () => {
  const alice = await login('alice@line-test.local', 'AliceStrongPass#2026');
  const bob = await login('bob@line-test.local', 'BobStrongPass#2026');
  const outsider = await login('user@line.local', 'LineUser#2026!');
  const cid = '00000000-0000-0000-0000-000000000010';
  assert.equal((await api('ws-ticket', null, 'POST')).status, 401);
  assert.equal((await api('ws', null, 'GET', undefined, { Upgrade: 'websocket' })).status, 401);
  for (const token of [alice, bob, outsider]) {
    const { ticket } = await (await api('ws-ticket', token, 'POST')).json();
    assert.equal((await api(`ws?ticket=${ticket}`, null, 'GET', undefined, { Upgrade: 'websocket' })).status, 101);
    assert.equal((await api(`ws?ticket=${ticket}`, null, 'GET', undefined, { Upgrade: 'websocket' })).status, 401);
  }
  const { ticket } = await (await api('ws-ticket', alice, 'POST')).json();
  const now = Date.now;
  try {
    Date.now = () => now() + 61000;
    assert.equal((await api(`ws?ticket=${ticket}`, null, 'GET', undefined, { Upgrade: 'websocket' })).status, 401);
  } finally { Date.now = now; }
  for (const path of ['messages', 'sync', 'pins', 'albums']) {
    assert.equal((await api(`conversations/${cid}/${path}`, outsider)).status, 404);
  }
  assert.equal((await api(`conversations/${cid}/messages`, outsider, 'POST', { body: 'intrusion' })).status, 404);
  const sent = await api(`conversations/${cid}/messages`, alice, 'POST', { body: 'private' });
  assert.equal(sent.status, 201);
  const { message } = await sent.json();
  assert.equal(sockets[0].events.at(-1).message.body, 'private');
  assert.equal(sockets[1].events.at(-1).message.body, 'private');
  assert.equal(sockets[2].events.length, 0);
  for (const method of ['PATCH', 'DELETE']) {
    assert.equal((await api(`conversations/${cid}/messages/${message.id}`, bob, method, { body: 'tamper' })).status, 403);
  }
  assert.equal((await api(`conversations/${cid}/messages/${message.id}`, alice, 'PATCH', { body: 'edited' })).status, 200);
  assert.equal((await api('calls', outsider, 'POST', { conversation_id: cid })).status, 404);
  const call = await (await api('calls', alice, 'POST', { conversation_id: cid })).json();
  assert.equal((await api(`calls/${call.id}/state`, outsider, 'POST', { state: 'ended' })).status, 404);
  await api(`calls/${call.id}/signal`, bob, 'POST', { signal_type: 'offer', sdp: 'private-sdp' });
  assert.equal(sockets[0].events.at(-1).sdp, 'private-sdp');
  assert.equal(sockets[2].events.length, 0);
  const poll = await (await api(`conversations/${cid}/polls`, alice, 'POST', { question: 'Lunch?', options: ['A', 'B'] })).json();
  assert.equal((await api(`polls/${poll.id}/vote`, outsider, 'POST', { option_id: 'opt-0' })).status, 404);
  for (const socket of sockets.slice(0, 2)) {
    assert.deepEqual(socket.events.at(-1), { type: 'poll.updated', conversation_id: cid, poll_id: poll.id });
  }
  assert.equal(sockets[2].events.length, 0);
  const vote = async (token, option_id) => {
    const response = await api(`polls/${poll.id}/vote`, token, 'POST', { option_id });
    assert.equal(response.status, 200);
    return (await response.json()).poll;
  };
  await vote(alice, 'opt-0');
  let updated = await vote(bob, 'opt-1');
  assert.deepEqual(updated.options.map(o => o.votes), [1, 1]);
  assert.deepEqual(updated.options.map(o => o.voted), [false, true]);
  updated = await vote(alice, 'opt-1');
  assert.deepEqual(updated.options.map(o => o.votes), [0, 2]);
  updated = await vote(alice, 'opt-1');
  assert.deepEqual(updated.options.map(o => o.votes), [0, 1]);
  const listed = await (await api(`conversations/${cid}/polls`, bob)).json();
  assert.deepEqual(listed.polls.find(p => p.id === poll.id).options.map(o => o.voted), [false, true]);
  assert.equal((await api(`polls/${poll.id}/close`, bob, 'POST')).status, 403);
  assert.equal((await api(`polls/${poll.id}/close`, outsider, 'POST')).status, 404);
  assert.equal((await api(`polls/${poll.id}/close`, alice, 'POST')).status, 200);
  assert.equal((await api(`polls/${poll.id}/vote`, bob, 'POST', { option_id: 'opt-0' })).status, 400);
  assert.equal(sockets[2].events.length, 0);

  // Security audit: Non-member access to sub-resources must return 404
  assert.equal((await api(`conversations/${cid}/pins`, outsider)).status, 404);
  assert.equal((await api(`conversations/${cid}/media`, outsider)).status, 404);
  assert.equal((await api(`conversations/${cid}/search?q=test`, outsider)).status, 404);
  assert.equal((await api(`conversations/${cid}/notes`, outsider)).status, 404);
  assert.equal((await api(`conversations/${cid}/events`, outsider)).status, 404);
  assert.equal((await api(`conversations/${cid}/albums`, outsider)).status, 404);

  // Relay notes: creation by member, listing by member
  const notePost = await api(`conversations/${cid}/notes`, alice, 'POST', { title: 'Team Meeting Notes', body: 'Sprint goals discussion' });
  assert.equal(notePost.status, 201);
  const noteObj = await notePost.json();
  assert.equal(noteObj.title, 'Team Meeting Notes');
  const bobNotes = await (await api(`conversations/${cid}/notes`, bob)).json();
  assert.ok(bobNotes.notes.some(n => n.id === noteObj.id));

  // Schedule events & RSVP: creation and member RSVP
  const eventPost = await api(`conversations/${cid}/events`, alice, 'POST', {
    title: 'Product Launch Party',
    starts_at: new Date(Date.now() + 86400000).toISOString(),
    location: 'Gangnam Office'
  });
  assert.equal(eventPost.status, 201);
  const eventObj = await eventPost.json();
  assert.equal((await api(`conversations/${cid}/events/${eventObj.id}/rsvp`, outsider, 'POST', { status: 'going' })).status, 404);
  const bobRsvp = await api(`conversations/${cid}/events/${eventObj.id}/rsvp`, bob, 'POST', { status: 'going' });
  assert.equal(bobRsvp.status, 200);

  // Keep Vault test: save, list, delete
  const saveRes = await api('saved-messages', alice, 'POST', { body: 'Important keep note', kind: 'text' });
  assert.equal(saveRes.status, 201);
  const savedItem = await saveRes.json();
  assert.equal(savedItem.body, 'Important keep note');

  const listRes = await api('saved-messages', alice, 'GET');
  assert.equal(listRes.status, 200);
  const { saved_messages } = await listRes.json();
  assert.ok(saved_messages.some(item => item.id === savedItem.id));

  const delRes = await api(`saved-messages/${savedItem.id}`, alice, 'DELETE');
  assert.equal(delRes.status, 200);

  // OpenChat Community test: create room, post, like
  const roomRes = await api('community/rooms', alice, 'POST', { name: 'Developers Lounge', description: 'Tech talk' });
  assert.equal(roomRes.status, 201);
  const room = await roomRes.json();

  const postRes = await api(`community/rooms/${room.id}/posts`, bob, 'POST', { body: 'Hello community' });
  assert.equal(postRes.status, 201);
  const post = await postRes.json();

  const likeRes = await api(`community/rooms/${room.id}/posts/${post.id}/like`, alice, 'POST');
  assert.equal(likeRes.status, 200);
  const { likes } = await likeRes.json();
  assert.equal(likes, 1);

  // Admin Moderation test
  const adminToken = await login('admin@line.local', 'LineAdmin#2026!');
  const modRes = await api('admin/moderation/actions', adminToken, 'POST', {
    subject_user_id: '00000000-0000-0000-0000-000000000003',
    action: 'warn',
    reason: 'Spam warning'
  });
  assert.equal(modRes.status, 200);
});
