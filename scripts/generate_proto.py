from pathlib import Path
p=Path(__file__).resolve().parents[1]/'proto/messenger/v1'
p.mkdir(parents=True,exist_ok=True)
header='syntax = "proto3";\npackage messenger.v1;\noption go_package = "github.com/example/global-messenger/backend/gen/messenger/v1;messengerv1";\n\n'
files={
'common.proto':'''message Empty {}
message PageRequest { int32 limit = 1; string cursor = 2; }
message PageInfo { string next_cursor = 1; bool has_more = 2; }
message EntityRef { string id = 1; string kind = 2; }
message ApiError { string code = 1; string message = 2; string request_id = 3; }
message Money { string currency = 1; int64 amount_minor = 2; }
''',
'identity.proto':'''import "messenger/v1/common.proto";
service IdentityService {
 rpc Register(RegisterRequest) returns (IdentityUser);
 rpc Login(LoginRequest) returns (TokenPair);
 rpc ResolveSession(ResolveSessionRequest) returns (IdentityUser);
 rpc RevokeDevice(RevokeDeviceRequest) returns (Empty);
}
message RegisterRequest { string email = 1; string password = 2; string display_name = 3; string locale = 4; }
message LoginRequest { string email = 1; string password = 2; string device_id = 3; }
message TokenPair { string access_token = 1; string refresh_token = 2; int64 expires_in = 3; }
message ResolveSessionRequest { string access_token = 1; }
message RevokeDeviceRequest { string actor_id = 1; string device_id = 2; }
message IdentityUser { string id = 1; string email = 2; string display_name = 3; string role = 4; string state = 5; }
''',
'social.proto':'''import "messenger/v1/common.proto";
service SocialService {
 rpc SearchUsers(SearchUsersRequest) returns (SearchUsersResponse);
 rpc AddFriend(AddFriendRequest) returns (Friendship);
 rpc BlockUser(BlockUserRequest) returns (Empty);
 rpc ListFriends(ListFriendsRequest) returns (SearchUsersResponse);
}
message SearchUsersRequest { string user_id = 1; string query = 2; PageRequest page = 3; }
message SearchUsersResponse { repeated UserProfile users = 1; PageInfo page = 2; }
message UserProfile { string id = 1; string display_name = 2; string handle = 3; string avatar_url = 4; }
message AddFriendRequest { string actor_id = 1; string friend_id = 2; }
message Friendship { string user_id = 1; string friend_id = 2; }
message BlockUserRequest { string actor_id = 1; string blocked_user_id = 2; }
message ListFriendsRequest { string user_id = 1; PageRequest page = 2; }
''',
'chat.proto':'''import "messenger/v1/common.proto";
service ChatService {
 rpc CreateConversation(CreateConversationRequest) returns (Conversation);
 rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse);
 rpc SendMessage(SendMessageRequest) returns (ChatMessage);
 rpc ListMessages(ListMessagesRequest) returns (ListMessagesResponse);
 rpc MarkRead(MarkReadRequest) returns (Empty);
 rpc AddMember(AddConversationMemberRequest) returns (Empty);
}
message Conversation { string id = 1; string kind = 2; string title = 3; repeated string member_ids = 4; int64 last_sequence = 5; }
message ChatMessage { string id = 1; string conversation_id = 2; string sender_id = 3; string client_id = 4; int64 sequence = 5; string kind = 6; string body = 7; bytes ciphertext = 8; int64 created_at_unix_ms = 9; }
message CreateConversationRequest { string actor_id = 1; string kind = 2; string title = 3; repeated string member_ids = 4; }
message ListConversationsRequest { string actor_id = 1; PageRequest page = 2; }
message ListConversationsResponse { repeated Conversation conversations = 1; PageInfo page = 2; }
message SendMessageRequest { string actor_id = 1; string conversation_id = 2; string client_id = 3; string kind = 4; string body = 5; bytes ciphertext = 6; }
message ListMessagesRequest { string actor_id = 1; string conversation_id = 2; int64 before_sequence = 3; int32 limit = 4; }
message ListMessagesResponse { repeated ChatMessage messages = 1; int64 next_before_sequence = 2; }
message MarkReadRequest { string actor_id = 1; string conversation_id = 2; int64 sequence = 3; }
message AddConversationMemberRequest { string actor_id = 1; string conversation_id = 2; string member_id = 3; }
''',
'realtime.proto':'''import "messenger/v1/chat.proto";
service RealtimeService { rpc StreamEvents(StreamEventsRequest) returns (stream ServerEvent); rpc PublishEvent(ServerEvent) returns (PublishEventResponse); }
message StreamEventsRequest { string user_id = 1; string device_id = 2; int64 after_offset = 3; }
message ServerEvent { string event_id = 1; string type = 2; string user_id = 3; ChatMessage message = 4; int64 created_at_unix_ms = 5; bytes payload = 6; }
message PublishEventResponse { bool accepted = 1; }
message DeliveryAck { string device_id = 1; string event_id = 2; int64 delivered_at_unix_ms = 3; }
''',
'calling.proto':'''import "messenger/v1/common.proto";
service CallingService { rpc StartCall(StartCallRequest) returns (Call); rpc Signal(CallSignal) returns (Empty); rpc EndCall(EndCallRequest) returns (Call); }
message StartCallRequest { string actor_id = 1; string conversation_id = 2; string kind = 3; }
message Call { string id = 1; string conversation_id = 2; string kind = 3; string state = 4; repeated string participant_ids = 5; }
message CallSignal { string call_id = 1; string from_user_id = 2; string to_user_id = 3; string kind = 4; string sdp = 5; string ice_candidate_json = 6; }
message EndCallRequest { string call_id = 1; string actor_id = 2; string reason = 3; }
''',
'media.proto':'''import "messenger/v1/common.proto";
service MediaService { rpc CreateUpload(CreateUploadRequest) returns (UploadGrant); rpc CompleteUpload(CompleteUploadRequest) returns (MediaObject); rpc GetMedia(GetMediaRequest) returns (MediaObject); }
message CreateUploadRequest { string actor_id = 1; string conversation_id = 2; string original_name = 3; string mime = 4; int64 size_bytes = 5; string sha256 = 6; }
message UploadGrant { string upload_id = 1; string presigned_url = 2; int64 expires_at_unix = 3; }
message CompleteUploadRequest { string actor_id = 1; string upload_id = 2; }
message GetMediaRequest { string actor_id = 1; string object_id = 2; }
message MediaObject { string id = 1; string object_key = 2; string mime = 3; int64 size_bytes = 4; string status = 5; }
''',
'business.proto':'''import "messenger/v1/common.proto";
service BusinessService { rpc CreateOrganization(CreateBusinessOrgRequest) returns (BusinessOrganization); rpc CreateOfficialAccount(CreateOfficialAccountRequest) returns (OfficialAccount); rpc CreateCampaign(CreateBusinessCampaignRequest) returns (BusinessCampaign); rpc AssignSupportTicket(AssignTicketRequest) returns (Empty); }
message CreateBusinessOrgRequest { string actor_id = 1; string name = 2; string legal_name = 3; string country = 4; }
message BusinessOrganization { string id = 1; string name = 2; string verification_state = 3; }
message CreateOfficialAccountRequest { string actor_id = 1; string organization_id = 2; string name = 3; }
message OfficialAccount { string id = 1; string organization_id = 2; string name = 3; string status = 4; }
message CreateBusinessCampaignRequest { string actor_id = 1; string official_account_id = 2; string name = 3; bytes template_json = 4; int64 schedule_unix_ms = 5; }
message BusinessCampaign { string id = 1; string status = 2; int64 scheduled_at_unix_ms = 3; }
message AssignTicketRequest { string actor_id = 1; string ticket_id = 2; string agent_id = 3; }
''',
'developer.proto':'''import "messenger/v1/common.proto";
service DeveloperService { rpc CreateApplication(CreateApplicationRequest) returns (DeveloperApplication); rpc RegisterWebhook(RegisterWebhookRequest) returns (WebhookConfig); rpc ValidateOAuthGrant(OAuthGrantRequest) returns (OAuthGrantResponse); }
message CreateApplicationRequest { string actor_id = 1; string organization_id = 2; string name = 3; repeated string redirect_uris = 4; }
message DeveloperApplication { string id = 1; string client_id = 2; string name = 3; repeated string scopes = 4; }
message RegisterWebhookRequest { string actor_id = 1; string app_id = 2; string endpoint_url = 3; }
message WebhookConfig { string id = 1; string endpoint_url = 2; bool enabled = 3; }
message OAuthGrantRequest { string client_id = 1; string authorization_code = 2; string code_verifier = 3; }
message OAuthGrantResponse { string access_token = 1; string scope = 2; int64 expires_in = 3; }
''',
'commerce.proto':'''import "messenger/v1/common.proto";
service CommerceService { rpc CreateOrder(CreateOrderRequest) returns (Order); rpc ConfirmPayment(ConfirmPaymentRequest) returns (Payment); rpc Refund(RefundRequest) returns (RefundResponse); }
message CreateOrderRequest { string buyer_id = 1; repeated OrderItem items = 2; string idempotency_key = 3; }
message OrderItem { string product_id = 1; int32 quantity = 2; }
message Order { string id = 1; string state = 2; Money total = 3; }
message ConfirmPaymentRequest { string order_id = 1; string provider = 2; string provider_transaction_id = 3; }
message Payment { string id = 1; string state = 2; Money total = 3; }
message RefundRequest { string payment_id = 1; Money amount = 2; string reason = 3; }
message RefundResponse { string refund_id = 1; string state = 2; }
''',
'creator.proto':'''import "messenger/v1/common.proto";
service CreatorService { rpc SubmitStickerPack(SubmitStickerPackRequest) returns (StickerPack); rpc ReviewStickerPack(ReviewStickerPackRequest) returns (StickerPack); rpc GetSettlement(GetCreatorSettlementRequest) returns (CreatorSettlement); }
message SubmitStickerPackRequest { string creator_id = 1; string title = 2; repeated string object_keys = 3; Money price = 4; }
message StickerPack { string id = 1; string title = 2; string state = 3; }
message ReviewStickerPackRequest { string actor_id = 1; string pack_id = 2; string outcome = 3; string reason = 4; }
message GetCreatorSettlementRequest { string creator_id = 1; string period = 2; }
message CreatorSettlement { string id = 1; Money amount = 2; string state = 3; }
''',
'ads.proto':'''import "messenger/v1/common.proto";
service AdsService { rpc CreateCampaign(CreateAdCampaignRequest) returns (AdCampaign); rpc ReviewCreative(ReviewAdCreativeRequest) returns (AdCreative); }
message CreateAdCampaignRequest { string advertiser_id = 1; string name = 2; string objective = 3; Money daily_budget = 4; }
message AdCampaign { string id = 1; string name = 2; string state = 3; }
message ReviewAdCreativeRequest { string actor_id = 1; string creative_id = 2; string outcome = 3; }
message AdCreative { string id = 1; string state = 2; }
''',
'community.proto':'''import "messenger/v1/common.proto";
service CommunityService { rpc CreateRoom(CreateRoomRequest) returns (CommunityRoom); rpc JoinRoom(JoinRoomRequest) returns (Empty); rpc PublishPost(PublishPostRequest) returns (CommunityPost); }
message CreateRoomRequest { string actor_id = 1; string name = 2; string visibility = 3; }
message CommunityRoom { string id = 1; string name = 2; string visibility = 3; }
message JoinRoomRequest { string actor_id = 1; string room_id = 2; string nickname = 3; }
message PublishPostRequest { string actor_id = 1; string room_id = 2; string body = 3; }
message CommunityPost { string id = 1; string room_id = 2; string body = 3; }
''',
'safety.proto':'''import "messenger/v1/common.proto";
service SafetyService { rpc FileReport(FileReportRequest) returns (SafetyReport); rpc TakeAction(TakeActionRequest) returns (ModerationAction); rpc Appeal(AppealRequest) returns (SafetyAppeal); }
message FileReportRequest { string reporter_id = 1; string subject_user_id = 2; string category = 3; string details = 4; }
message SafetyReport { string id = 1; string state = 2; string category = 3; }
message TakeActionRequest { string actor_id = 1; string report_id = 2; string action = 3; string reason = 4; }
message ModerationAction { string id = 1; string action = 2; }
message AppealRequest { string actor_id = 1; string action_id = 2; string explanation = 3; }
message SafetyAppeal { string id = 1; string state = 2; }
''',
'ops.proto':'''import "messenger/v1/common.proto";
service OperationsService { rpc GetMetrics(GetMetricsRequest) returns (PlatformMetrics); rpc SetFeatureFlag(SetFeatureFlagRequest) returns (Empty); }
message GetMetricsRequest { string requester_id = 1; }
message PlatformMetrics { int64 users = 1; int64 conversations = 2; int64 messages = 3; int64 open_reports = 4; }
message SetFeatureFlagRequest { string actor_id = 1; string key = 2; bool enabled = 3; }
'''
}
for name,body in files.items(): (p/name).write_text(header+body,encoding='utf-8')
print(f'Generated {len(files)} proto files, {sum((p/n).read_text().count(chr(10)) for n in files)} lines')
