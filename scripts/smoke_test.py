#!/usr/bin/env python3
"""Real HTTP integration smoke test against a running local API. No mock success.
Run only after docker compose up --build and the /readyz endpoint is ready.
Creates two new accounts and permanently stores their messages in the dev DB.
"""
import json
import os
import secrets
import sys
import urllib.error
import urllib.request
from urllib.parse import quote

BASE=os.getenv('API_URL','http://localhost:8080').rstrip('/')
def call(method,path,body=None,token=None):
    data=json.dumps(body).encode() if body is not None else None
    headers={'Content-Type':'application/json'}
    if token: headers['Authorization']='Bearer '+token
    req=urllib.request.Request(BASE+path,data=data,headers=headers,method=method)
    try:
        with urllib.request.urlopen(req,timeout=10) as resp: return resp.status,json.load(resp)
    except urllib.error.HTTPError as e:
        return e.code,json.loads(e.read().decode())
def expect(actual,expected,where):
    if actual!=expected: raise AssertionError(f'{where}: expected {expected}, got {actual}')

def main():
    status,ready=call('GET','/readyz');expect(status,200,'ready')
    suffix=secrets.token_hex(6)
    users=[]
    for i in range(2):
        email=f'messenger-smoke-{suffix}-{i}@example.test'
        payload={'email':email,'password':'integration-password-123!','display_name':f'Smoke User {i}'}
        status,r=call('POST','/api/v1/auth/register',payload);expect(status,201,'register')
        status,l=call('POST','/api/v1/auth/login',{'email':email,'password':payload['password']});expect(status,200,'login')
        users.append({'id':r['id'],'token':l['access_token']})
    a,b=users
    status,denied=call('GET','/api/v1/admin/stats',token=a['token']);expect(status,403,'user denied admin')
    status,_=call('POST','/api/v1/friends',{'friend_id':b['id']},a['token']);expect(status,201,'add friend')
    status,c=call('POST','/api/v1/conversations',{'kind':'direct','title':'Smoke direct chat','member_ids':[b['id']]},a['token']);expect(status,201,'create conversation')
    conv=c['id'];client_id='smoke-'+secrets.token_hex(10)
    payload={'client_id':client_id,'kind':'text','body':'Real PostgreSQL persistence smoke test'}
    status,m=call('POST',f'/api/v1/conversations/{conv}/messages',payload,a['token']);expect(status,201,'send')
    status,m2=call('POST',f'/api/v1/conversations/{conv}/messages',payload,a['token']);expect(status,200,'idempotent resend')
    if m2['message']['id']!=m['message']['id']: raise AssertionError('message idempotency violated')
    status,history=call('GET',f'/api/v1/conversations/{conv}/messages',token=b['token']);expect(status,200,'receive')
    if len(history['messages'])!=1 or history['messages'][0]['body']!=payload['body']: raise AssertionError('message history missing')
    status,_=call('POST',f'/api/v1/conversations/{conv}/read',{'sequence':1},b['token']);expect(status,200,'mark read')
    status,forbidden=call('GET','/api/v1/conversations/'+secrets.token_hex(16)+'/messages',token=b['token']);expect(status,403,'unknown conversation denied')
    print('PASS: health, 2 register/login, RBAC, friendship, conversation, send, duplicate idempotency, history, read, denial')
    print('Conversation:',conv)
if __name__=='__main__':
    try: main()
    except Exception as e: print('FAIL:',e,file=sys.stderr);sys.exit(1)
