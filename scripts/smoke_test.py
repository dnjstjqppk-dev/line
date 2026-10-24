#!/usr/bin/env python3
"""Comprehensive HTTP integration smoke test against a running local API. No mock success.
Run against a live API (backed by either PostgreSQL or embedded pgsqlite).
Tests full user lifecycle, QR pairing, E2EE keys, attachments, calls, offline sync, and commerce.
"""
import json
import os
import secrets
import sys
import urllib.error
import urllib.request
import uuid

BASE = os.getenv('API_URL', 'http://localhost:8080').rstrip('/')

def call(method, path, body=None, token=None):
    data = json.dumps(body).encode() if body is not None else None
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            content_type = resp.headers.get('Content-Type', '')
            if 'application/json' in content_type:
                return resp.status, json.load(resp)
            return resp.status, resp.read().decode('utf-8', errors='replace')
    except urllib.error.HTTPError as e:
        content_type = e.headers.get('Content-Type', '')
        if 'application/json' in content_type:
            return e.code, json.loads(e.read().decode())
        return e.code, e.read().decode('utf-8', errors='replace')

def expect(actual, expected, where):
    if actual != expected:
        raise AssertionError(f'{where}: expected {expected}, got {actual}')

def main():
    print(f'Starting smoke test against {BASE}...')
    status, ready = call('GET', '/readyz')
    expect(status, 200, 'ready')

    status, metrics = call('GET', '/metrics')
    expect(status, 200, 'metrics')

    suffix = secrets.token_hex(4)
    users = []
    for i in range(2):
        email = f'messenger-smoke-{suffix}-{i}@example.test'
        payload = {
            'email': email,
            'password': 'StrongPassword#123!',
            'display_name': f'Smoke User {i}',
            'phone_e164': f'+8210{suffix}{i}',
            'handle': f'smoke_{suffix}_{i}',
        }
        status, r = call('POST', '/api/v1/auth/register', payload)
        expect(status, 201, 'register')
        status, l = call('POST', '/api/v1/auth/login', {'email': email, 'password': payload['password'], 'platform': 'web', 'device_name': f'Device {i}'})
        expect(status, 200, 'login')
        users.append({
            'user_id': r['id'],
            'token': l['access_token'],
            'device_id': l['device_id'],
        })

    u0, u1 = users[0], users[1]

    # Profile & Settings
    status, me = call('PATCH', '/api/v1/me', {'bio': 'Smoke test user'}, token=u0['token'])
    expect(status, 200, 'update me')

    status, sett = call('PUT', '/api/v1/me/settings', {
        'searchable_by_email': True,
        'searchable_by_phone': True,
        'show_read_receipts': True,
        'allow_friend_requests': True,
        'theme': 'dark'
    }, token=u0['token'])
    expect(status, 200, 'update settings')

    # Friends
    status, _ = call('POST', '/api/v1/friends', {'friend_id': u1['user_id']}, token=u0['token'])
    expect(status, 201, 'add friend')

    status, friends = call('GET', '/api/v1/friends', token=u0['token'])
    expect(status, 200, 'list friends')

    # Direct conversation
    status, conv = call('POST', '/api/v1/conversations', {'kind': 'direct', 'member_ids': [u1['user_id']]}, token=u0['token'])
    expect(status, 201, 'create conv')
    conv_id = conv['id']

    # Send message
    msg_client_id = str(uuid.uuid4())
    status, msg = call('POST', f'/api/v1/conversations/{conv_id}/messages', {
        'client_id': msg_client_id,
        'kind': 'text',
        'body': 'Smoke message from user 0',
    }, token=u0['token'])
    expect(status, 201, 'send message')
    msg_id = msg['message']['id']

    # Mark read & React
    status, _ = call('POST', f'/api/v1/conversations/{conv_id}/read', {'sequence': 1}, token=u1['token'])
    expect(status, 200, 'mark read')

    status, _ = call('POST', f'/api/v1/conversations/{conv_id}/messages/{msg_id}/reactions', {'reaction': '👍'}, token=u1['token'])
    expect(status, 201, 'react')

    # QR Pairing
    status, qr = call('POST', '/api/v1/auth/qr/challenge', {'device_name': 'Desktop-Smoke', 'platform': 'windows'})
    expect(status, 201, 'qr challenge')
    challenge_id = qr['challenge_id']
    qr_token = qr['qr_token']

    status, _ = call('POST', '/api/v1/auth/qr/approve', {'challenge_id': challenge_id, 'qr_token': qr_token}, token=u0['token'])
    expect(status, 200, 'qr approve')

    status, poll = call('POST', '/api/v1/auth/qr/poll', {'challenge_id': challenge_id, 'qr_token': qr_token})
    expect(status, 200, 'qr poll')
    expect(poll['status'], 'authenticated', 'qr poll status')

    # E2EE Keys
    status, _ = call('PUT', f'/api/v1/devices/{u0["device_id"]}/keys', {
        'identity_public_key': 'aWRlbnRpdHlfcHVi',
        'signed_prekey': 'c2lnbmVkX3ByZWtleQ==',
        'signed_prekey_signature': 'c2lnbmF0dXJl',
        'one_time_prekeys': [{'key_id': 1, 'public_key': 'b3RwMQ=='}],
    }, token=u0['token'])
    expect(status, 200, 'upload keys')

    status, _ = call('GET', f'/api/v1/users/{u0["user_id"]}/keys', token=u1['token'])
    expect(status, 200, 'get keys')

    # Calling
    status, call_resp = call('POST', '/api/v1/calls', {'conversation_id': conv_id, 'kind': 'video', 'sdp_offer': 'v=0\r\n...'}, token=u0['token'])
    expect(status, 201, 'start call')
    call_id = call_resp['id']

    status, _ = call('POST', f'/api/v1/calls/{call_id}/signal', {'signal_type': 'offer', 'sdp': 'v=0\r\n...'}, token=u0['token'])
    expect(status, 200, 'call signal')

    status, _ = call('POST', f'/api/v1/calls/{call_id}/quality', {'rtt_ms': 25, 'jitter_ms': 3, 'packet_loss_ratio': 0.0}, token=u0['token'])
    expect(status, 201, 'call quality')

    # Attachment
    status, att = call('POST', '/api/v1/attachments', {
        'original_name': 'test.png',
        'mime_type': 'image/png',
        'size_bytes': 100,
        'checksum_sha256': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
    }, token=u0['token'])
    expect(status, 201, 'upload attachment')

    # Offline sync
    status, sync = call('GET', f'/api/v1/conversations/{conv_id}/sync?after_sequence=0', token=u1['token'])
    expect(status, 200, 'sync messages')
    assert len(sync['messages']) >= 1, 'expected synced messages'

    # Commerce store order
    status, order = call('POST', '/api/v1/store/orders', {
        'product_type': 'sticker_pack',
        'name': 'Smoke Stickers',
        'price_minor': 1000,
        'currency': 'KRW',
        'idempotency_key': f'order-{suffix}'
    }, token=u0['token'])
    expect(status, 201, 'store order')

    # RBAC security & partner organization
    status, flags = call('GET', '/api/v1/admin/feature-flags', token=u0['token'])
    expect(status, 403, 'admin flags forbidden')

    status, org = call('POST', '/api/v1/partner/organizations', {
        'name': 'Smoke Corp',
        'legal_name': 'Smoke Corporation Ltd.',
        'country': 'KR'
    }, token=u0['token'])
    expect(status, 201, 'create partner org')

    print('✅ ALL SMOKE TESTS PASSED (14 domain checkpoints verified)!')

if __name__ == '__main__':
    main()
