import { readFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
import test from 'node:test';

const source = await readFile(new URL('./cf_worker.js', import.meta.url), 'utf8');
const { default: worker } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);

test('persistent gateway forwards API and upgrade requests without demo fallback', async () => {
  const originalFetch = globalThis.fetch;
  const env = { API_BACKEND_ORIGIN: 'https://backend.example' };
  const received = [];
  const response = new Response('upstream', { status: 409 });
  try {
    globalThis.fetch = async request => { received.push(request); return response; };
    const result = await worker.fetch(new Request('https://edge.example/api/v1/saved-messages?q=1', {
      method: 'POST', headers: { Authorization: 'Bearer test', 'Content-Type': 'application/json' },
      body: '{"message_id":"test"}',
    }), env);
    assert.equal(result, response);
    assert.equal(received[0].url, 'https://backend.example/api/v1/saved-messages?q=1');
    assert.equal(received[0].headers.get('Authorization'), 'Bearer test');
    assert.equal(received[0].method, 'POST');
    assert.equal(await received[0].text(), '{"message_id":"test"}');
    assert.equal(received[0].redirect, 'manual');
    await worker.fetch(new Request('https://edge.example/api/v1/ws?ticket=one-use', {
      headers: { Upgrade: 'websocket', Origin: 'https://edge.example' },
    }), env);
    assert.equal(received[1].headers.get('Upgrade'), 'websocket');
    assert.equal(received[1].headers.get('Origin'), 'https://edge.example');
    assert.equal(received[1].url, 'https://backend.example/api/v1/ws?ticket=one-use');
    await worker.fetch(new Request('https://edge.example/readyz'), env);
    assert.equal(received[2].url, 'https://backend.example/readyz');
    for (const origin of ['http://backend.example', 'https://edge.example', 'invalid', 'https://backend.example/path', 'https://user:pass@backend.example']) {
      assert.equal((await worker.fetch(new Request('https://edge.example/api/v1/me'), { API_BACKEND_ORIGIN: origin })).status, 503);
    }
    assert.equal(received.length, 3);
    globalThis.fetch = async () => { throw new Error('offline'); };
    assert.equal((await worker.fetch(new Request('https://edge.example/api/v1/me'), env)).status, 502);
  } finally { globalThis.fetch = originalFetch; }
});
