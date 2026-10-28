package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type apiClient struct {
	t      *testing.T
	srv    *httptest.Server
	token  string
	userID string
	devID  string
}

func (c *apiClient) req(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatalf("encode body: %v", err)
		}
	}
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, c.srv.URL+path, &buf)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, c.srv.URL+path, nil)
	}
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("http do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (c *apiClient) connectWS() *websocket.Conn {
	c.t.Helper()
	st, ticketResp := c.req("POST", "/api/v1/ws-ticket", nil)
	if st != 200 {
		c.t.Fatalf("ws-ticket expected 200, got %d: %v", st, ticketResp)
	}
	ticket := ticketResp["ticket"].(string)
	wsURL := "ws" + strings.TrimPrefix(c.srv.URL, "http") + "/api/v1/ws?ticket=" + ticket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		c.t.Fatalf("websocket dial failed: %v", err)
	}
	return conn
}

func readWSEvent(t *testing.T, conn *websocket.Conn, expectedType string) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("expected ws event %q, got error: %v", expectedType, err)
		}
		var ev map[string]any
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		if ev["type"] == expectedType {
			return ev
		}
	}
}

func TestFullMessenger12CriticalE2EScenariosAndAllDomains(t *testing.T) {
	cfg := Config{
		DSN:     "sqlite://file:e2e_full_suite?mode=memory&cache=shared",
		Secret:  "0123456789abcdef0123456789abcdef-super-secret-jwt-key",
		Port:    "0",
		Service: "gateway",
		Origins: []string{"*"},
	}
	app, err := New(cfg)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	defer app.Close()

	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	// -------------------------------------------------------------------------
	// E2E Scenario 1: Android 앱에서 회원가입 및 로그인한다.
	// -------------------------------------------------------------------------
	androidA := &apiClient{t: t, srv: srv}
	st, regA := androidA.req("POST", "/api/v1/auth/register", map[string]any{
		"email":        "alice@line-test.local",
		"password":     "AliceStrongPass#2026",
		"display_name": "Alice Kim",
		"phone_e164":   "+821011112222",
		"handle":       "alice_kim",
	})
	if st != 201 {
		t.Fatalf("Scenario 1 register Alice failed: %d %v", st, regA)
	}
	androidA.userID = regA["id"].(string)

	st, loginA := androidA.req("POST", "/api/v1/auth/login", map[string]any{
		"email":       "alice@line-test.local",
		"password":    "AliceStrongPass#2026",
		"device_name": "Galaxy S26 Ultra",
		"platform":    "android",
	})
	if st != 200 {
		t.Fatalf("Scenario 1 login Alice failed: %d %v", st, loginA)
	}
	androidA.token = loginA["access_token"].(string)
	androidA.devID = loginA["device_id"].(string)

	// Upload E2EE Signal/Letter-Sealing Prekeys for Alice's Android device
	st, keyResp := androidA.req("PUT", "/api/v1/devices/"+androidA.devID+"/keys", map[string]any{
		"identity_public_key":     "YWxpY2UtaWRlbnRpdHktcHVibGljLWtleQ==",
		"signed_prekey":           "YWxpY2Utc2lnbmVkLXByZWtleQ==",
		"signed_prekey_signature": "YWxpY2Utc2lnbmF0dXJl",
		"one_time_prekeys": []map[string]any{
			{"key_id": 1, "public_key": "YWxpY2Utb3RwLTE="},
		},
	})
	if st != 200 {
		t.Fatalf("upload device keys failed: %d %v", st, keyResp)
	}

	// -------------------------------------------------------------------------
	// E2E Scenario 2: Windows PC 앱에서 같은 계정으로 QR 로그인한다.
	// -------------------------------------------------------------------------
	pcA := &apiClient{t: t, srv: srv}
	st, qrChall := pcA.req("POST", "/api/v1/auth/qr/challenge", map[string]any{
		"device_name": "Alice-Windows11-Desktop",
		"platform":    "windows",
	})
	if st != 201 {
		t.Fatalf("Scenario 2 QR challenge failed: %d %v", st, qrChall)
	}
	challengeID := qrChall["challenge_id"].(string)
	qrToken := qrChall["qr_token"].(string)

	// Poll before approval -> 202 pending
	st, pollPending := pcA.req("POST", "/api/v1/auth/qr/poll", map[string]any{
		"challenge_id": challengeID,
		"qr_token":     qrToken,
	})
	if st != 202 || pollPending["status"] != "pending" {
		t.Fatalf("Scenario 2 expected 202 pending before approval, got %d %v", st, pollPending)
	}

	// Alice approves QR login from her authenticated Android app
	st, qrApp := androidA.req("POST", "/api/v1/auth/qr/approve", map[string]any{
		"challenge_id": challengeID,
		"qr_token":     qrToken,
	})
	if st != 200 {
		t.Fatalf("Scenario 2 QR approve failed: %d %v", st, qrApp)
	}

	// Windows PC polls again -> receives full authenticated session tokens for Alice's account
	st, pollDone := pcA.req("POST", "/api/v1/auth/qr/poll", map[string]any{
		"challenge_id": challengeID,
		"qr_token":     qrToken,
	})
	if st != 200 || pollDone["user_id"] != androidA.userID {
		t.Fatalf("Scenario 2 QR poll authentication failed: %d %v", st, pollDone)
	}
	pcA.token = pollDone["access_token"].(string)
	pcA.userID = pollDone["user_id"].(string)
	pcA.devID = pollDone["device_id"].(string)

	// -------------------------------------------------------------------------
	// E2E Scenario 3: 다른 사용자(iOS)를 가입시키고 친구로 추가한다.
	// -------------------------------------------------------------------------
	iosB := &apiClient{t: t, srv: srv}
	st, regB := iosB.req("POST", "/api/v1/auth/register", map[string]any{
		"email":        "bob@line-test.local",
		"password":     "BobStrongPass#2026",
		"display_name": "Bob Lee",
		"phone_e164":   "+821033334444",
		"handle":       "bob_lee",
	})
	if st != 201 {
		t.Fatalf("register Bob failed: %d %v", st, regB)
	}
	iosB.userID = regB["id"].(string)

	st, loginB := iosB.req("POST", "/api/v1/auth/login", map[string]any{
		"email":       "bob@line-test.local",
		"password":    "BobStrongPass#2026",
		"device_name": "iPhone 17 Pro",
		"platform":    "ios",
	})
	if st != 200 {
		t.Fatalf("login Bob failed: %d %v", st, loginB)
	}
	iosB.token = loginB["access_token"].(string)
	iosB.devID = loginB["device_id"].(string)

	// Alice searches for Bob and adds him as a friend
	st, searchResp := androidA.req("GET", "/api/v1/users?search=Bob", nil)
	if st != 200 || len(searchResp["users"].([]any)) == 0 {
		t.Fatalf("Scenario 3 search Bob failed: %d %v", st, searchResp)
	}
	st, friendAdd := androidA.req("POST", "/api/v1/friends", map[string]any{
		"friend_id": iosB.userID,
	})
	if st != 201 {
		t.Fatalf("Scenario 3 add friend failed: %d %v", st, friendAdd)
	}

	// Connect real-time WebSockets for Alice (Android), Alice (Windows PC), and Bob (iOS)
	wsAndroidA := androidA.connectWS()
	defer wsAndroidA.Close()
	wsPCA := pcA.connectWS()
	defer wsPCA.Close()
	wsIOSB := iosB.connectWS()
	defer wsIOSB.Close()

	// -------------------------------------------------------------------------
	// E2E Scenario 4, 5, 7: Android에서 메시지를 보내면 상대방 iOS 앱 및 동일 계정의 PC 앱에 실시간 도착한다.
	// -------------------------------------------------------------------------
	st, convResp := androidA.req("POST", "/api/v1/conversations", map[string]any{
		"kind":       "direct",
		"title":      "Alice & Bob Chat",
		"member_ids": []string{iosB.userID},
	})
	if st != 201 {
		t.Fatalf("Scenario 4 create conversation failed: %d %v", st, convResp)
	}
	convID := convResp["id"].(string)

	st, sendResp1 := androidA.req("POST", "/api/v1/conversations/"+convID+"/messages", map[string]any{
		"client_id": "msg-android-0001",
		"kind":      "text",
		"body":      "안녕하세요 밥! 안드로이드에서 보낸 메시지입니다.",
	})
	if st != 201 {
		t.Fatalf("Scenario 4 send message from Android failed: %d %v", st, sendResp1)
	}
	msg1ID := sendResp1["message"].(map[string]any)["id"].(string)

	// Scenario 5: Verify real-time arrival on Bob's iOS WebSocket
	evIOS := readWSEvent(t, wsIOSB, "message.created")
	if evIOS["message"].(map[string]any)["id"] != msg1ID {
		t.Fatalf("Scenario 5 iOS did not receive expected message: %v", evIOS)
	}

	// Scenario 7: Verify real-time arrival on Alice's Windows PC WebSocket
	evPC := readWSEvent(t, wsPCA, "message.created")
	if evPC["message"].(map[string]any)["id"] != msg1ID {
		t.Fatalf("Scenario 7 PC did not receive synced message: %v", evPC)
	}
	// Consume Android's own echo event
	_ = readWSEvent(t, wsAndroidA, "message.created")

	// -------------------------------------------------------------------------
	// E2E Scenario 6: 상대방(iOS)이 읽으면 읽음 상태가 실시간 동기화된다.
	// -------------------------------------------------------------------------
	st, readResp := iosB.req("POST", "/api/v1/conversations/"+convID+"/read", map[string]any{
		"sequence": 1,
	})
	if st != 200 {
		t.Fatalf("Scenario 6 markRead failed: %d %v", st, readResp)
	}
	evReadAndroid := readWSEvent(t, wsAndroidA, "conversation.read")
	if evReadAndroid["user_id"] != iosB.userID || int64(evReadAndroid["sequence"].(float64)) != 1 {
		t.Fatalf("Scenario 6 Android did not receive read receipt sync: %v", evReadAndroid)
	}
	evReadPC := readWSEvent(t, wsPCA, "conversation.read")
	if evReadPC["user_id"] != iosB.userID {
		t.Fatalf("Scenario 6 PC did not receive read receipt sync: %v", evReadPC)
	}
	_ = readWSEvent(t, wsIOSB, "conversation.read")

	// -------------------------------------------------------------------------
	// E2E Scenario 8: PC에서 메시지를 답장(Reply)한다.
	// -------------------------------------------------------------------------
	st, replyResp := pcA.req("POST", "/api/v1/conversations/"+convID+"/messages", map[string]any{
		"client_id": "msg-pc-reply-0002",
		"kind":      "text",
		"body":      "PC 앱에서 이어서 답장합니다!",
		"reply_to":  msg1ID,
	})
	if st != 201 {
		t.Fatalf("Scenario 8 PC reply failed: %d %v", st, replyResp)
	}
	evReplyIOS := readWSEvent(t, wsIOSB, "message.created")
	if evReplyIOS["message"].(map[string]any)["reply_to"] != msg1ID {
		t.Fatalf("Scenario 8 reply_to missing on iOS event: %v", evReplyIOS)
	}
	_ = readWSEvent(t, wsAndroidA, "message.created")
	_ = readWSEvent(t, wsPCA, "message.created")

	// -------------------------------------------------------------------------
	// E2E Scenario 9: PC에서 이미지와 파일을 보낸다.
	// -------------------------------------------------------------------------
	st, attResp := pcA.req("POST", "/api/v1/attachments", map[string]any{
		"original_name":  "architecture_diagram.png",
		"mime_type":      "image/png",
		"size_bytes":     68,
		"content_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
	})
	if st != 201 {
		t.Fatalf("Scenario 9 PC attachment upload failed: %d %v", st, attResp)
	}
	attID := attResp["id"].(string)

	st, imgMsgResp := pcA.req("POST", "/api/v1/conversations/"+convID+"/messages", map[string]any{
		"client_id": "msg-pc-image-0003",
		"kind":      "image",
		"body":      "architecture_diagram.png",
		"metadata": map[string]any{
			"attachment_id": attID,
			"mime_type":     "image/png",
			"size_bytes":    40960,
		},
	})
	if st != 201 {
		t.Fatalf("Scenario 9 PC send image message failed: %d %v", st, imgMsgResp)
	}
	evImgIOS := readWSEvent(t, wsIOSB, "message.created")
	if evImgIOS["message"].(map[string]any)["kind"] != "image" {
		t.Fatalf("Scenario 9 expected kind=image on iOS: %v", evImgIOS)
	}
	_ = readWSEvent(t, wsAndroidA, "message.created")
	_ = readWSEvent(t, wsPCA, "message.created")

	// -------------------------------------------------------------------------
	// E2E Scenario 10: PC와 모바일 사이에서 음성·영상통화를 한다.
	// -------------------------------------------------------------------------
	st, callResp := pcA.req("POST", "/api/v1/calls", map[string]any{
		"conversation_id": convID,
		"kind":            "video",
		"sdp_offer":       "v=0\r\no=- 461173 2 IN IP4 127.0.0.1\r\ns=LINE-PC-Video",
	})
	if st != 201 {
		t.Fatalf("Scenario 10 start video call from PC failed: %d %v", st, callResp)
	}
	callID := callResp["id"].(string)

	// Bob's iOS receives call.ringing over WebSocket
	evRing := readWSEvent(t, wsIOSB, "call.ringing")
	if evRing["call_id"] != callID || evRing["kind"] != "video" {
		t.Fatalf("Scenario 10 iOS did not receive call.ringing: %v", evRing)
	}

	// Bob sends WebRTC SDP Answer and accepts the call
	st, sigResp := iosB.req("POST", "/api/v1/calls/"+callID+"/signal", map[string]any{
		"signal_type": "answer",
		"sdp":         "v=0\r\no=- 998877 2 IN IP4 127.0.0.1\r\ns=LINE-iOS-Answer",
	})
	if st != 200 {
		t.Fatalf("Scenario 10 call signal answer failed: %d %v", st, sigResp)
	}
	evSigPC := readWSEvent(t, wsPCA, "call.signal")
	if evSigPC["signal_type"] != "answer" {
		t.Fatalf("Scenario 10 PC did not receive call.signal answer: %v", evSigPC)
	}

	st, stateConn := iosB.req("POST", "/api/v1/calls/"+callID+"/state", map[string]any{
		"state": "connected",
	})
	if st != 200 {
		t.Fatalf("Scenario 10 call state connected failed: %d %v", st, stateConn)
	}
	evStatePC := readWSEvent(t, wsPCA, "call.state")
	if evStatePC["state"] != "connected" {
		t.Fatalf("Scenario 10 PC did not receive call.state connected: %v", evStatePC)
	}

	// Record WebRTC call quality sample & end call
	st, _ = pcA.req("POST", "/api/v1/calls/"+callID+"/quality", map[string]any{
		"rtt_ms": 24, "jitter_ms": 4, "packet_loss_ratio": 0.001,
	})
	if st != 201 {
		t.Fatalf("Scenario 10 call quality sample failed: %d", st)
	}
	st, _ = pcA.req("POST", "/api/v1/calls/"+callID+"/state", map[string]any{
		"state": "ended", "end_reason": "normal_hangup",
	})
	if st != 200 {
		t.Fatalf("Scenario 10 call end failed: %d", st)
	}

	// -------------------------------------------------------------------------
	// E2E Scenario 11: 네트워크를 끊었다가 연결해도 메시지가 복구된다.
	// -------------------------------------------------------------------------
	_ = wsIOSB.Close() // Simulate Bob's iOS losing network connection (last seen sequence = 3)

	for i := 4; i <= 6; i++ {
		st, _ := androidA.req("POST", "/api/v1/conversations/"+convID+"/messages", map[string]any{
			"client_id": fmt.Sprintf("offline-msg-%d", i),
			"kind":      "text",
			"body":      fmt.Sprintf("오프라인 동안 전송된 메시지 #%d", i),
		})
		if st != 201 {
			t.Fatalf("Scenario 11 send message #%d while offline failed: %d", i, st)
		}
	}

	// Bob reconnects and requests delta sync after sequence 3
	st, syncResp := iosB.req("GET", "/api/v1/conversations/"+convID+"/sync?after_sequence=3", nil)
	if st != 200 {
		t.Fatalf("Scenario 11 offline sync failed: %d %v", st, syncResp)
	}
	syncedMsgs := syncResp["messages"].([]any)
	if len(syncedMsgs) != 3 {
		t.Fatalf("Scenario 11 expected 3 missed messages, got %d", len(syncedMsgs))
	}
	if int64(syncedMsgs[0].(map[string]any)["sequence"].(float64)) != 4 || int64(syncedMsgs[2].(map[string]any)["sequence"].(float64)) != 6 {
		t.Fatalf("Scenario 11 sequence order mismatch: %v", syncedMsgs)
	}

	// -------------------------------------------------------------------------
	// E2E Scenario 12: 사용자 권한으로 관리자·타 파트너 API에 접근할 수 없음을 검증한다.
	// -------------------------------------------------------------------------
	st, _ = androidA.req("GET", "/api/v1/admin/stats", nil)
	if st != 403 {
		t.Fatalf("Scenario 12 expected 403 on /admin/stats for normal user, got %d", st)
	}
	st, _ = androidA.req("GET", "/api/v1/admin/users", nil)
	if st != 403 {
		t.Fatalf("Scenario 12 expected 403 on /admin/users for normal user, got %d", st)
	}
	st, _ = androidA.req("POST", "/api/v1/admin/moderation/actions", map[string]any{
		"subject_user_id": iosB.userID,
		"action":          "suspend",
	})
	if st != 403 {
		t.Fatalf("Scenario 12 expected 403 on /admin/moderation/actions for normal user, got %d", st)
	}

	// Alice creates a Business Organization; Bob (non-member) is forbidden from accessing it
	st, orgResp := androidA.req("POST", "/api/v1/partner/organizations", map[string]any{
		"name":       "Alice Corp",
		"legal_name": "Alice Corporation Ltd.",
		"country":    "KR",
	})
	if st != 201 {
		t.Fatalf("create partner org failed: %d %v", st, orgResp)
	}
	orgID := orgResp["id"].(string)

	st, _ = iosB.req("GET", "/api/v1/partner/organizations/"+orgID+"/accounts", nil)
	if st != 403 {
		t.Fatalf("Scenario 12 expected 403 when Bob accesses Alice's partner org, got %d", st)
	}

	// -------------------------------------------------------------------------
	// Additional Phase 3~8 Domain Feature Verifications (Polls, Store, Community, Partner, Dev, Creator, Ads, Admin)
	// -------------------------------------------------------------------------
	// Reactions & Pins & Polls & Search
	st, _ = iosB.req("POST", "/api/v1/conversations/"+convID+"/messages/"+msg1ID+"/reactions", map[string]any{"reaction": "heart"})
	if st != 201 {
		t.Fatalf("message reaction failed: %d", st)
	}
	st, _ = androidA.req("POST", "/api/v1/conversations/"+convID+"/pins", map[string]any{"message_id": msg1ID})
	if st != 201 {
		t.Fatalf("pin message failed: %d", st)
	}
	st, pollCreate := androidA.req("POST", "/api/v1/conversations/"+convID+"/polls", map[string]any{
		"question": "팀 회식 장소 투표",
		"options":  []string{"강남", "판교"},
	})
	if st != 201 {
		t.Fatalf("create poll failed: %d %v", st, pollCreate)
	}
	st, searchAll := androidA.req("GET", "/api/v1/search?q=안드로이드", nil)
	if st != 200 || len(searchAll["messages"].([]any)) == 0 {
		t.Fatalf("unified search failed: %d %v", st, searchAll)
	}

	// Creator Sticker Pack + Store Order (Double-Entry Ledger) + OpenChat Community
	st, _ = androidA.req("POST", "/api/v1/creator/profile", map[string]any{"display_name": "Alice Studio"})
	if st != 200 {
		t.Fatalf("creator profile failed: %d", st)
	}
	st, packResp := androidA.req("POST", "/api/v1/creator/packs", map[string]any{"title": "Cute Bear Stickers", "price_minor": 2500, "currency": "KRW"})
	if st != 201 {
		t.Fatalf("creator pack failed: %d %v", st, packResp)
	}
	packID := packResp["id"].(string)

	st, orderResp := iosB.req("POST", "/api/v1/store/orders", map[string]any{
		"product_type":    "sticker_pack",
		"resource_id":     packID,
		"name":            "Cute Bear Stickers",
		"price_minor":     2500,
		"currency":        "KRW",
		"idempotency_key": "order-idem-0001",
	})
	if st != 503 {
		t.Fatalf("unverified payment provider must fail closed: %d %v", st, orderResp)
	}
	var paymentCount int
	if err := app.db.QueryRow("SELECT count(*) FROM commerce.payments").Scan(&paymentCount); err != nil || paymentCount != 0 {
		t.Fatalf("blocked checkout wrote payments: %d %v", paymentCount, err)
	}

	st, roomResp := androidA.req("POST", "/api/v1/community/rooms", map[string]any{
		"name":        "Go & Vue Developers Korea",
		"description": "OpenChat Community",
		"nickname":    "AliceHost",
	})
	if st != 201 {
		t.Fatalf("community room failed: %d %v", st, roomResp)
	}

	// Partner Official Account, Developer App, Ads Campaign
	st, oaResp := androidA.req("POST", "/api/v1/partner/organizations/"+orgID+"/accounts", map[string]any{
		"name":        "Alice Official Brand",
		"description": "Official Account",
	})
	if st != 201 {
		t.Fatalf("official account failed: %d %v", st, oaResp)
	}
	st, devOrg := androidA.req("POST", "/api/v1/developer/organizations", map[string]any{"name": "Alice Dev Labs"})
	if st != 201 {
		t.Fatalf("dev org failed: %d %v", st, devOrg)
	}
	st, devApp := androidA.req("POST", "/api/v1/developer/organizations/"+devOrg["id"].(string)+"/apps", map[string]any{"name": "Alice Bot App"})
	if st != 201 || devApp["client_secret_once"] == "" {
		t.Fatalf("dev app failed: %d %v", st, devApp)
	}
	st, advResp := androidA.req("POST", "/api/v1/ads/advertisers", map[string]any{"organization_id": orgID, "name": "Alice Ads"})
	if st != 201 {
		t.Fatalf("advertiser failed: %d %v", st, advResp)
	}
	st, campResp := androidA.req("POST", "/api/v1/ads/campaigns", map[string]any{
		"advertiser_id":      advResp["id"].(string),
		"name":               "Autumn Launch Campaign",
		"objective":          "conversions",
		"daily_budget_minor": 100000,
		"currency":           "KRW",
	})
	if st != 201 {
		t.Fatalf("ads campaign failed: %d %v", st, campResp)
	}

	// Elevate Alice to platform_admin in DB to verify Super Admin Console & Audit Logs
	_, err = app.db.Exec("UPDATE identity.users SET role='platform_admin' WHERE id=$1", androidA.userID)
	if err != nil {
		t.Fatalf("grant platform_admin failed: %v", err)
	}
	st, statsResp := androidA.req("GET", "/api/v1/admin/stats", nil)
	if st != 200 || statsResp["metrics"] == nil {
		t.Fatalf("admin stats failed: %d %v", st, statsResp)
	}
}
