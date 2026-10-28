package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestPollsParityAndAuthorization(t *testing.T) {
	cfg := Config{
		DSN:     "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "polls.db")),
		Secret:  "0123456789abcdef0123456789abcdef",
		Service: "gateway",
	}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer func() { srv.Close(); app.Close() }()

	login := func(email, password string) *apiClient {
		c := &apiClient{t: t, srv: srv}
		if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": email, "password": password, "display_name": email}); st != 201 {
			t.Fatalf("register: %d %v", st, out)
		}
		status, result := c.req("POST", "/api/v1/auth/login", map[string]any{"email": email, "password": password, "platform": "web", "device_name": "test"})
		if status != 200 {
			t.Fatalf("login: %d %v", status, result)
		}
		c.token = result["access_token"].(string)
		c.userID = result["user_id"].(string)
		return c
	}

	alice := login("alice-poll@example.test", "AliceStrongPass#2026!")
	bob := login("bob-poll@example.test", "BobStrongPass#2026!")
	eve := login("eve-poll@example.test", "EveStrongPass#2026!")

	// Alice creates group conversation with Bob
	st, conv := alice.req("POST", "/api/v1/conversations", map[string]any{
		"kind":       "group",
		"title":      "Lunch Team",
		"member_ids": []string{bob.userID},
	})
	if st != 201 {
		t.Fatalf("create conv: %d %v", st, conv)
	}
	cid := conv["id"].(string)

	aliceWS := alice.connectWS()
	defer aliceWS.Close()
	bobWS := bob.connectWS()
	defer bobWS.Close()

	// Eve (outsider) cannot list polls
	if st, _ := eve.req("GET", "/api/v1/conversations/"+cid+"/polls", nil); st != 403 {
		t.Fatalf("outsider list polls: expected 403, got %d", st)
	}

	// Alice creates single-choice poll (multiple: false)
	st, created := alice.req("POST", "/api/v1/conversations/"+cid+"/polls", map[string]any{
		"question": "Lunch location?",
		"options":  []string{"Gangnam", "Pangyo", "Hongdae"},
		"multiple": false,
	})
	if st != 201 {
		t.Fatalf("create poll: %d %v", st, created)
	}
	pollID := created["id"].(string)
	if created["question"] != "Lunch location?" || created["closed"] != false || created["multiple"] != false {
		t.Fatalf("poll fields: %v", created)
	}
	opts := created["options"].([]any)
	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	opt1 := opts[0].(map[string]any)["id"].(string)
	opt2 := opts[1].(map[string]any)["id"].(string)

	// Bob received poll.updated on WebSocket
	readWSEvent(t, bobWS, "poll.updated")

	// Verify inline message of kind 'poll' was created
	st, msgsResp := alice.req("GET", "/api/v1/conversations/"+cid+"/messages", nil)
	if st != 200 {
		t.Fatalf("list msgs: %d %v", st, msgsResp)
	}
	msgs := msgsResp["messages"].([]any)
	if len(msgs) == 0 || msgs[len(msgs)-1].(map[string]any)["kind"] != "poll" {
		t.Fatalf("expected inline poll message in conversation: %v", msgs)
	}

	// Eve cannot vote
	if st, _ := eve.req("POST", "/api/v1/polls/"+pollID+"/vote", map[string]any{"option_id": opt1}); st != 403 {
		t.Fatalf("outsider vote: expected 403, got %d", st)
	}

	// Alice votes for opt1
	st, v1 := alice.req("POST", "/api/v1/polls/"+pollID+"/vote", map[string]any{"option_id": opt1})
	if st != 200 {
		t.Fatalf("alice vote: %d %v", st, v1)
	}
	readWSEvent(t, bobWS, "poll.updated")
	pv := v1["poll"].(map[string]any)
	optsV := pv["options"].([]any)
	if optsV[0].(map[string]any)["votes"].(float64) != 1 || optsV[0].(map[string]any)["voted"] != true {
		t.Fatalf("vote state: %v", optsV[0])
	}

	// Alice switches vote to opt2 (single choice) -> opt1 should have 0 votes, opt2 should have 1 vote
	st, v2 := alice.req("POST", "/api/v1/polls/"+pollID+"/vote", map[string]any{"option_id": opt2})
	if st != 200 {
		t.Fatalf("alice switch vote: %d %v", st, v2)
	}
	pv2 := v2["poll"].(map[string]any)["options"].([]any)
	if pv2[0].(map[string]any)["votes"].(float64) != 0 || pv2[0].(map[string]any)["voted"] != false {
		t.Fatalf("opt1 after switch: %v", pv2[0])
	}
	if pv2[1].(map[string]any)["votes"].(float64) != 1 || pv2[1].(map[string]any)["voted"] != true {
		t.Fatalf("opt2 after switch: %v", pv2[1])
	}

	// Bob votes for opt2 as well -> opt2 should have 2 votes
	st, v3 := bob.req("POST", "/api/v1/polls/"+pollID+"/vote", map[string]any{"option_id": opt2})
	if st != 200 {
		t.Fatalf("bob vote: %d %v", st, v3)
	}
	pv3 := v3["poll"].(map[string]any)["options"].([]any)
	if pv3[1].(map[string]any)["votes"].(float64) != 2 || pv3[1].(map[string]any)["voted"] != true {
		t.Fatalf("opt2 bob vote: %v", pv3[1])
	}

	// Alice un-votes opt2 (toggle off) -> opt2 should have 1 vote (Bob's)
	st, v4 := alice.req("POST", "/api/v1/polls/"+pollID+"/vote", map[string]any{"option_id": opt2})
	if st != 200 {
		t.Fatalf("alice un-vote: %d %v", st, v4)
	}
	pv4 := v4["poll"].(map[string]any)["options"].([]any)
	if pv4[1].(map[string]any)["votes"].(float64) != 1 || pv4[1].(map[string]any)["voted"] != false {
		t.Fatalf("opt2 after alice unvote: %v", pv4[1])
	}

	// Bob attempts to close poll -> 403 (only creator Alice can close)
	if st, _ := bob.req("POST", "/api/v1/polls/"+pollID+"/close", nil); st != 403 {
		t.Fatalf("bob close poll: expected 403, got %d", st)
	}

	// Alice closes poll
	st, closedResp := alice.req("POST", "/api/v1/polls/"+pollID+"/close", nil)
	if st != 200 || closedResp["poll"].(map[string]any)["closed"] != true {
		t.Fatalf("alice close poll: %d %v", st, closedResp)
	}
	readWSEvent(t, bobWS, "poll.updated")

	// Voting on closed poll is rejected
	if st, _ := bob.req("POST", "/api/v1/polls/"+pollID+"/vote", map[string]any{"option_id": opt1}); st != 400 {
		t.Fatalf("vote on closed poll: expected 400, got %d", st)
	}

	// Multiple choice poll test
	st, multiPoll := alice.req("POST", "/api/v1/conversations/"+cid+"/polls", map[string]any{
		"question": "Preferred days?",
		"options":  []string{"Monday", "Wednesday", "Friday"},
		"multiple": true,
	})
	if st != 201 {
		t.Fatalf("create multi poll: %d %v", st, multiPoll)
	}
	mPollID := multiPoll["id"].(string)
	mOpts := multiPoll["options"].([]any)
	mOpt1 := mOpts[0].(map[string]any)["id"].(string)
	mOpt2 := mOpts[1].(map[string]any)["id"].(string)

	// Alice votes for both opt1 and opt2
	alice.req("POST", "/api/v1/polls/"+mPollID+"/vote", map[string]any{"option_id": mOpt1})
	st, mV2 := alice.req("POST", "/api/v1/polls/"+mPollID+"/vote", map[string]any{"option_id": mOpt2})
	if st != 200 {
		t.Fatalf("vote multi: %d %v", st, mV2)
	}
	mOptsV := mV2["poll"].(map[string]any)["options"].([]any)
	if mOptsV[0].(map[string]any)["voted"] != true || mOptsV[1].(map[string]any)["voted"] != true {
		t.Fatalf("multi vote state: %v", mOptsV)
	}

	// Alice toggles opt1 off
	st, mV3 := alice.req("POST", "/api/v1/polls/"+mPollID+"/vote", map[string]any{"option_id": mOpt1})
	if st != 200 {
		t.Fatalf("toggle off multi: %d %v", st, mV3)
	}
	mOptsV3 := mV3["poll"].(map[string]any)["options"].([]any)
	if mOptsV3[0].(map[string]any)["voted"] != false || mOptsV3[1].(map[string]any)["voted"] != true {
		t.Fatalf("multi after toggle: %v", mOptsV3)
	}

	// Test persistence across restart
	aliceWS.Close()
	bobWS.Close()
	srv.Close()
	app.Close()

	app, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv = httptest.NewServer(app.Handler())
	bob.srv = srv

	st, listed := bob.req("GET", "/api/v1/conversations/"+cid+"/polls", nil)
	if st != 200 {
		t.Fatalf("list polls after restart: %d %v", st, listed)
	}
	pollList := listed["polls"].([]any)
	if len(pollList) != 2 {
		t.Fatalf("expected 2 persisted polls, got %d", len(pollList))
	}
}

func TestReactionsParityAndAuthorization(t *testing.T) {
	cfg := Config{
		DSN:     "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "reactions.db")),
		Secret:  "0123456789abcdef0123456789abcdef",
		Service: "gateway",
	}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer func() { srv.Close(); app.Close() }()

	login := func(email, password string) *apiClient {
		c := &apiClient{t: t, srv: srv}
		if st, out := c.req("POST", "/api/v1/auth/register", map[string]any{"email": email, "password": password, "display_name": email}); st != 201 {
			t.Fatalf("register: %d %v", st, out)
		}
		status, result := c.req("POST", "/api/v1/auth/login", map[string]any{"email": email, "password": password, "platform": "web", "device_name": "test"})
		if status != 200 {
			t.Fatalf("login: %d %v", status, result)
		}
		c.token = result["access_token"].(string)
		c.userID = result["user_id"].(string)
		return c
	}

	alice := login("alice-rx@example.test", "AliceStrongPass#2026!")
	bob := login("bob-rx@example.test", "BobStrongPass#2026!")
	eve := login("eve-rx@example.test", "EveStrongPass#2026!")

	st, conv := alice.req("POST", "/api/v1/conversations", map[string]any{
		"kind":       "group",
		"title":      "Reaction Chat",
		"member_ids": []string{bob.userID},
	})
	if st != 201 {
		t.Fatalf("create conv: %d %v", st, conv)
	}
	cid := conv["id"].(string)

	aliceWS := alice.connectWS()
	defer aliceWS.Close()

	// Alice sends message
	st, sent := alice.req("POST", "/api/v1/conversations/"+cid+"/messages", map[string]any{
		"client_id": "msg-rx-1",
		"kind":      "text",
		"body":      "Hello with reactions!",
	})
	if st != 201 {
		t.Fatalf("send msg: %d %v", st, sent)
	}
	mid := sent["message"].(map[string]any)["id"].(string)

	// Eve (outsider) cannot react
	if st, _ := eve.req("POST", "/api/v1/conversations/"+cid+"/messages/"+mid+"/reactions", map[string]any{"reaction": "❤️"}); st != 403 {
		t.Fatalf("outsider react: expected 403, got %d", st)
	}

	// Bob reacts with heart
	st, r1 := bob.req("POST", "/api/v1/conversations/"+cid+"/messages/"+mid+"/reactions", map[string]any{"reaction": "❤️"})
	if st != 201 {
		t.Fatalf("bob react: %d %v", st, r1)
	}
	if r1["status"] != "reacted" {
		t.Fatalf("reaction status: %v", r1)
	}
	rxCounts := r1["reactions"].(map[string]any)
	if rxCounts["❤️"].(float64) != 1 {
		t.Fatalf("heart count: %v", rxCounts)
	}

	// Alice received message.reaction on WebSocket
	event := readWSEvent(t, aliceWS, "message.reaction")
	if event["reactions"].(map[string]any)["❤️"].(float64) != 1 {
		t.Fatalf("ws reaction event: %v", event)
	}

	// Alice also reacts with heart
	st, r2 := alice.req("POST", "/api/v1/conversations/"+cid+"/messages/"+mid+"/reactions", map[string]any{"reaction": "❤️"})
	if st != 201 {
		t.Fatalf("alice react: %d %v", st, r2)
	}
	if r2["reactions"].(map[string]any)["❤️"].(float64) != 2 {
		t.Fatalf("heart count after alice: %v", r2)
	}

	// Bob reacts with thumbs up
	bob.req("POST", "/api/v1/conversations/"+cid+"/messages/"+mid+"/reactions", map[string]any{"reaction": "👍"})

	// Alice fetches messages -> should contain reactions map and my_reactions
	st, msgsResp := alice.req("GET", "/api/v1/conversations/"+cid+"/messages", nil)
	if st != 200 {
		t.Fatalf("list msgs: %d %v", st, msgsResp)
	}
	targetMsg := msgsResp["messages"].([]any)[0].(map[string]any)
	rMap := targetMsg["reactions"].(map[string]any)
	if rMap["❤️"].(float64) != 2 || rMap["👍"].(float64) != 1 {
		t.Fatalf("reactions map in list: %v", rMap)
	}
	myRx := targetMsg["my_reactions"].([]any)
	if len(myRx) != 1 || myRx[0].(string) != "❤️" {
		t.Fatalf("my_reactions for alice: %v", myRx)
	}

	// Bob fetches messages -> my_reactions should contain both ❤️ and 👍
	_, bobMsgs := bob.req("GET", "/api/v1/conversations/"+cid+"/messages", nil)
	bobTarget := bobMsgs["messages"].([]any)[0].(map[string]any)
	bobMyRx := bobTarget["my_reactions"].([]any)
	if len(bobMyRx) != 2 {
		t.Fatalf("my_reactions for bob: %v", bobMyRx)
	}

	// Bob toggles off heart
	st, r3 := bob.req("POST", "/api/v1/conversations/"+cid+"/messages/"+mid+"/reactions", map[string]any{"reaction": "❤️"})
	if st != 200 || r3["status"] != "removed" {
		t.Fatalf("bob un-react: %d %v", st, r3)
	}
	if r3["reactions"].(map[string]any)["❤️"].(float64) != 1 {
		t.Fatalf("heart count after removal: %v", r3)
	}

	// Test persistence across restart
	aliceWS.Close()
	srv.Close()
	app.Close()

	app, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv = httptest.NewServer(app.Handler())
	alice.srv = srv

	st, restartedMsgs := alice.req("GET", "/api/v1/conversations/"+cid+"/messages", nil)
	if st != 200 {
		t.Fatalf("list msgs after restart: %d %v", st, restartedMsgs)
	}
	restartedTarget := restartedMsgs["messages"].([]any)[0].(map[string]any)
	restartedRMap := restartedTarget["reactions"].(map[string]any)
	if restartedRMap["❤️"].(float64) != 1 || restartedRMap["👍"].(float64) != 1 {
		t.Fatalf("persisted reactions: %v", restartedRMap)
	}
}
