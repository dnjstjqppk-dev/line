import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';

const apiBase = String.fromEnvironment('API_URL', defaultValue: 'http://10.0.2.2:8080');
const storage = FlutterSecureStorage();
void main() => runApp(const MessengerApp());

class Api {
  String? token;
  Future<Map<String, dynamic>> call(String path, {String method = 'GET', Map<String, dynamic>? data}) async {
    final uri = Uri.parse('$apiBase$path');
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (token != null) headers['Authorization'] = 'Bearer $token';
    late http.Response res;
    try {
      switch (method) {
        case 'POST':
          res = await http.post(uri, headers: headers, body: jsonEncode(data ?? {})).timeout(const Duration(seconds: 12));
          break;
        case 'PUT':
          res = await http.put(uri, headers: headers, body: jsonEncode(data ?? {})).timeout(const Duration(seconds: 12));
          break;
        default:
          res = await http.get(uri, headers: headers).timeout(const Duration(seconds: 12));
      }
    } catch (e) {
      throw Exception('Network failure: $e');
    }
    final parsed = jsonDecode(utf8.decode(res.bodyBytes));
    if (parsed is! Map<String, dynamic>) throw Exception('Invalid server response');
    if (res.statusCode < 200 || res.statusCode >= 300) throw Exception(parsed['error'] ?? 'HTTP ${res.statusCode}');
    return parsed;
  }
}

class MessengerApp extends StatelessWidget {
  const MessengerApp({super.key});
  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'Global Messenger',
        theme: ThemeData(colorSchemeSeed: Colors.green, useMaterial3: true),
        home: const Home(),
      );
}

class Home extends StatefulWidget {
  const Home({super.key});
  @override
  State<Home> createState() => _HomeState();
}

class _HomeState extends State<Home> {
  final api = Api();
  final email = TextEditingController(),
      password = TextEditingController(),
      name = TextEditingController(),
      search = TextEditingController(),
      message = TextEditingController(),
      qrChallenge = TextEditingController(),
      qrToken = TextEditingController(),
      groupTitle = TextEditingController();
  bool register = false, busy = false;
  String error = '', notice = '', selfId = '', selectedId = '', selectedTitle = '', replyToId = '';
  List<dynamic> chats = [], friends = [], results = [], messages = [];
  Map<String, dynamic>? activeCall;
  WebSocketChannel? socket;
  StreamSubscription? wsSubscription;

  @override
  void initState() {
    super.initState();
    _restore();
  }

  Future<void> _restore() async {
    final saved = await storage.read(key: 'access_token');
    if (saved == null) return;
    api.token = saved;
    try {
      final me = await api.call('/api/v1/me');
      if (!mounted) return;
      setState(() => selfId = me['id']);
      await _reload();
      await _connect();
    } catch (_) {
      await storage.delete(key: 'access_token');
      api.token = null;
    }
  }

  Future<void> _do(Future<void> Function() action) async {
    if (busy) return;
    setState(() {
      busy = true;
      error = '';
      notice = '';
    });
    try {
      await action();
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> _login() async {
    _do(() async {
      if (register) {
        await api.call('/api/v1/auth/register', method: 'POST', data: {
          'email': email.text,
          'password': password.text,
          'display_name': name.text,
        });
        setState(() {
          register = false;
          notice = '계정을 생성했습니다. 로그인하세요.';
        });
        return;
      }
      final result = await api.call('/api/v1/auth/login', method: 'POST', data: {
        'email': email.text,
        'password': password.text,
        'platform': 'android',
        'device_name': 'Mobile Client',
      });
      api.token = result['access_token'];
      await storage.write(key: 'access_token', value: api.token);
      final me = await api.call('/api/v1/me');
      if (mounted) setState(() => selfId = me['id']);
      password.clear();
      await _reload();
      await _connect();
    });
  }

  Future<void> _approveDesktopQR() async {
    _do(() async {
      await api.call('/api/v1/auth/qr/approve', method: 'POST', data: {
        'challenge_id': qrChallenge.text.trim(),
        'qr_token': qrToken.text.trim(),
      });
      qrChallenge.clear();
      qrToken.clear();
      setState(() => notice = 'PC QR 로그인을 승인했습니다.');
    });
  }

  Future<void> _reload() async {
    final x = await api.call('/api/v1/conversations');
    final f = await api.call('/api/v1/friends');
    if (mounted) {
      setState(() {
        chats = x['conversations'];
        friends = f['friends'];
      });
    }
  }

  Future<void> _connect() async {
    await wsSubscription?.cancel();
    await socket?.sink.close();
    final ticket = await api.call('/api/v1/ws-ticket', method: 'POST');
    final wsBase = apiBase.startsWith('https:') ? apiBase.replaceFirst('https:', 'wss:') : apiBase.replaceFirst('http:', 'ws:');
    socket = WebSocketChannel.connect(Uri.parse('$wsBase/api/v1/ws?ticket=${Uri.encodeQueryComponent(ticket['ticket'])}'));
    wsSubscription = socket!.stream.listen((event) {
      try {
        final data = jsonDecode(event);
        if (data['type'] == 'message.created') {
          final item = data['message'];
          if (mounted) {
            setState(() {
              if (item['conversation_id'] == selectedId && !messages.any((m) => m['id'] == item['id'])) {
                messages.add(item);
              }
            });
          }
          _reload();
        } else if (data['type'] == 'conversation.read') {
          _reload();
        } else if (data['type'] == 'call.ringing') {
          if (mounted) setState(() => activeCall = data);
        } else if (data['type'] == 'call.state' && data['state'] == 'ended') {
          if (mounted) setState(() => activeCall = null);
        }
      } catch (_) {}
    }, onError: (_) {
      if (mounted) setState(() => notice = '실시간 연결 오류. 다시 로그인하면 재접속됩니다.');
    });
  }

  Future<void> _find() async {
    _do(() async {
      final x = await api.call('/api/v1/users?search=${Uri.encodeQueryComponent(search.text)}');
      if (mounted) setState(() => results = x['users']);
    });
  }

  Future<void> _friend(String id) async {
    _do(() async {
      await api.call('/api/v1/friends', method: 'POST', data: {'friend_id': id});
      await _reload();
    });
  }

  Future<void> _openChat(String id, String title) async {
    _do(() async {
      selectedId = id;
      selectedTitle = title;
      replyToId = '';
      final x = await api.call('/api/v1/conversations/$id/messages');
      if (mounted) setState(() => messages = x['messages']);
      if (messages.isNotEmpty) {
        await api.call('/api/v1/conversations/$id/read', method: 'POST', data: {'sequence': messages.last['sequence']});
        await _reload();
      }
    });
  }

  Future<void> _syncOffline() async {
    if (selectedId.isEmpty) return;
    _do(() async {
      final lastSeq = messages.isNotEmpty ? messages.last['sequence'] : 0;
      final x = await api.call('/api/v1/conversations/$selectedId/sync?after_sequence=$lastSeq');
      final list = x['messages'] as List<dynamic>;
      if (mounted) {
        setState(() {
          for (final m in list) {
            if (!messages.any((old) => old['id'] == m['id'])) messages.add(m);
          }
          notice = '오프라인 메시지 ${list.length}건 동기화 완료';
        });
      }
      await _reload();
    });
  }

  Future<void> _startChat(String id, String title) async {
    _do(() async {
      final c = await api.call('/api/v1/conversations', method: 'POST', data: {
        'kind': 'direct',
        'member_ids': [id],
        'title': title,
      });
      selectedId = c['id'];
      selectedTitle = title;
      final x = await api.call('/api/v1/conversations/$selectedId/messages');
      if (mounted) setState(() => messages = x['messages']);
      await _reload();
    });
  }

  Future<void> _startCall(String kind) async {
    if (selectedId.isEmpty) return;
    _do(() async {
      final res = await api.call('/api/v1/calls', method: 'POST', data: {
        'conversation_id': selectedId,
        'kind': kind,
        'sdp_offer': 'v=0\r\n',
      });
      setState(() => activeCall = res);
    });
  }

  Future<void> _endCall() async {
    if (activeCall == null) return;
    _do(() async {
      final callId = activeCall!['id'] ?? activeCall!['call_id'];
      await api.call('/api/v1/calls/$callId/state', method: 'POST', data: {'state': 'ended', 'end_reason': 'hangup'});
      setState(() => activeCall = null);
    });
  }

  String _uuid() {
    final r = Random.secure();
    final bytes = List<int>.generate(16, (_) => r.nextInt(256));
    return bytes.map((x) => x.toRadixString(16).padLeft(2, '0')).join();
  }

  Future<void> _send() async {
    if (selectedId.isEmpty || message.text.trim().isEmpty) return;
    final text = message.text;
    final reply = replyToId;
    message.clear();
    replyToId = '';
    await _do(() async {
      final x = await api.call('/api/v1/conversations/$selectedId/messages', method: 'POST', data: {
        'client_id': _uuid(),
        'kind': 'text',
        'body': text,
        if (reply.isNotEmpty) 'reply_to': reply,
      });
      if (mounted) {
        setState(() {
          if (!messages.any((m) => m['id'] == x['message']['id'])) messages.add(x['message']);
        });
        await _reload();
      }
    });
    if (error.isNotEmpty) message.text = text;
  }

  Future<void> _logout() async {
    await wsSubscription?.cancel();
    await socket?.sink.close();
    await storage.delete(key: 'access_token');
    api.token = null;
    if (mounted) {
      setState(() {
        selfId = '';
        chats = [];
        friends = [];
        messages = [];
        selectedId = '';
        activeCall = null;
      });
    }
  }

  @override
  void dispose() {
    email.dispose();
    password.dispose();
    name.dispose();
    search.dispose();
    message.dispose();
    qrChallenge.dispose();
    qrToken.dispose();
    groupTitle.dispose();
    wsSubscription?.cancel();
    socket?.sink.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Global Messenger'),
        actions: [
          if (selfId.isNotEmpty) IconButton(onPressed: _logout, icon: const Icon(Icons.logout)),
        ],
      ),
      body: Column(children: [
        if (error.isNotEmpty)
          Container(
            color: Colors.red.shade100,
            padding: const EdgeInsets.all(8),
            width: double.infinity,
            child: Text(error, style: const TextStyle(color: Colors.red)),
          ),
        if (notice.isNotEmpty)
          Container(
            color: Colors.green.shade100,
            padding: const EdgeInsets.all(8),
            width: double.infinity,
            child: Text(notice, style: const TextStyle(color: Colors.green)),
          ),
        if (activeCall != null)
          Container(
            color: Colors.amber.shade100,
            padding: const EdgeInsets.all(8),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('통화 연결 중 (${activeCall!['kind'] ?? 'voice'})'),
                ElevatedButton(onPressed: _endCall, child: const Text('종료')),
              ],
            ),
          ),
        if (selfId.isEmpty)
          Expanded(
            child: Center(
              child: SingleChildScrollView(
                child: SizedBox(
                  width: 350,
                  child: Card(
                    child: Padding(
                      padding: const EdgeInsets.all(18),
                      child: Column(mainAxisSize: MainAxisSize.min, children: [
                        Text(register ? '회원가입' : '로그인', style: Theme.of(context).textTheme.titleLarge),
                        TextField(controller: email, decoration: const InputDecoration(labelText: '이메일')),
                        if (register) TextField(controller: name, decoration: const InputDecoration(labelText: '표시 이름')),
                        TextField(
                          controller: password,
                          obscureText: true,
                          decoration: const InputDecoration(labelText: '비밀번호 (가입 시 12자 이상)'),
                        ),
                        const SizedBox(height: 12),
                        FilledButton(onPressed: busy ? null : _login, child: Text(register ? '가입' : '로그인')),
                        TextButton(
                          onPressed: () => setState(() => register = !register),
                          child: Text(register ? '로그인으로' : '새 계정 만들기'),
                        ),
                      ]),
                    ),
                  ),
                ),
              ),
            ),
          ),
        if (selfId.isNotEmpty && selectedId.isEmpty)
          Expanded(
            child: ListView(children: [
              Padding(
                padding: const EdgeInsets.all(10),
                child: Card(
                  child: Padding(
                    padding: const EdgeInsets.all(10),
                    child: Column(children: [
                      const Text('PC QR 로그인 승인 (모바일 인증)', style: TextStyle(fontWeight: FontWeight.bold)),
                      TextField(controller: qrChallenge, decoration: const InputDecoration(labelText: 'QR Challenge ID')),
                      TextField(controller: qrToken, decoration: const InputDecoration(labelText: 'QR Token')),
                      const SizedBox(height: 6),
                      FilledButton(onPressed: _approveDesktopQR, child: const Text('PC 로그인 승인')),
                    ]),
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 12),
                child: Row(children: [
                  Expanded(child: TextField(controller: search, decoration: const InputDecoration(labelText: '사용자 검색'))),
                  IconButton(onPressed: _find, icon: const Icon(Icons.search)),
                ]),
              ),
              ...results.map((u) => ListTile(
                    title: Text('${u['display_name']}'),
                    trailing: TextButton(onPressed: () => _friend(u['id']), child: const Text('친구 추가')),
                  )),
              const Divider(),
              const ListTile(title: Text('친구 목록', style: TextStyle(fontWeight: FontWeight.bold))),
              ...friends.map((u) => ListTile(
                    title: Text('${u['display_name']}'),
                    trailing: const Icon(Icons.chat),
                    onTap: () => _startChat(u['id'], u['display_name']),
                  )),
              const Divider(),
              const ListTile(title: Text('대화 목록', style: TextStyle(fontWeight: FontWeight.bold))),
              ...chats.map((c) => ListTile(
                    title: Text('${c['title']}'.isEmpty ? '대화' : c['title']),
                    subtitle: Text('미읽음 ${c['unread_count']}건'),
                    onTap: () => _openChat(c['id'], c['title'] ?? '대화'),
                  )),
            ]),
          ),
        if (selfId.isNotEmpty && selectedId.isNotEmpty)
          Expanded(
            child: Column(children: [
              ListTile(
                leading: IconButton(icon: const Icon(Icons.arrow_back), onPressed: () => setState(() => selectedId = '')),
                title: Text(selectedTitle),
                trailing: Row(mainAxisSize: MainAxisSize.min, children: [
                  IconButton(icon: const Icon(Icons.phone), onPressed: () => _startCall('voice')),
                  IconButton(icon: const Icon(Icons.videocam), onPressed: () => _startCall('video')),
                  IconButton(icon: const Icon(Icons.sync), onPressed: _syncOffline),
                ]),
              ),
              Expanded(
                child: ListView.builder(
                  padding: const EdgeInsets.all(10),
                  itemCount: messages.length,
                  itemBuilder: (context, i) {
                    final m = messages[i];
                    final mine = m['sender_id'] == selfId;
                    return Align(
                      alignment: mine ? Alignment.centerRight : Alignment.centerLeft,
                      child: Card(
                        color: mine ? Colors.green.shade100 : null,
                        child: InkWell(
                          onLongPress: () => setState(() => replyToId = m['id'] ?? ''),
                          child: Padding(
                            padding: const EdgeInsets.all(10),
                            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                              if (m['reply_to'] != null && '${m['reply_to']}'.isNotEmpty)
                                const Text('↩ 답장', style: TextStyle(fontSize: 10, color: Colors.grey)),
                              Text('${m['body']}'),
                              Text('#${m['sequence']}', style: const TextStyle(fontSize: 9, color: Colors.grey)),
                            ]),
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ),
              if (replyToId.isNotEmpty)
                Container(
                  color: Colors.grey.shade200,
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      const Text('메시지 답장 중...'),
                      IconButton(icon: const Icon(Icons.close, size: 16), onPressed: () => setState(() => replyToId = '')),
                    ],
                  ),
                ),
              Padding(
                padding: const EdgeInsets.all(8),
                child: Row(children: [
                  Expanded(child: TextField(controller: message, decoration: const InputDecoration(hintText: '메시지 입력'))),
                  IconButton(onPressed: busy ? null : _send, icon: const Icon(Icons.send)),
                ]),
              ),
            ]),
          ),
      ]),
    );
  }
}
