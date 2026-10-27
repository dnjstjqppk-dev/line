# 📊 LINE FEATURE PARITY MATRIX (기능 동등성 매트릭스)

**플랫폼:** Global LINE-Grade Messenger Platform  
**기준 버전:** v1.0.0 (Web + Mobile Android + Backend + Edge)  
**분류 기준:** `IMPLEMENTED` / `PARTIAL` / `MISSING` / `BLOCKED` / `VERIFIED`  

---

| 대분류 | 세부 기능 항목 | LINE 공식 기능 설명 | 본 플랫폼 구현 상태 | 검증 수준 (Test Verdict) |
|---|---|---|:---:|:---:|
| **회원 & 계정** | 신규 회원가입 | 이메일/휴대폰 기반 계정 등록 | `IMPLEMENTED` | `VERIFIED` (PBKDF2/JWT) |
| | 로그인/로그아웃 | 다중 기기 토큰 발급 및 세션 종료 | `IMPLEMENTED` | `VERIFIED` (E2E Test) |
| | QR 코드 PC 로그인 | 모바일 승인 기반 PC/웹 챌린지 인증 | `IMPLEMENTED` | `VERIFIED` (QR Challenge/Approve) |
| | 프로필 & 상태메시지 | 닉네임, 상태메시지, 아바타 컬러, 생일 | `IMPLEMENTED` | `VERIFIED` (UI & API) |
| | BGM 뱃지 | 프로필 상단 음악 타이틀 표시 | `IMPLEMENTED` | `VERIFIED` (LINE Home UI) |
| **친구 관리** | 사용자 검색 | ID/핸들/이메일 기반 친구 검색 | `IMPLEMENTED` | `VERIFIED` (API GET /users) |
| | 친구 추가 & 수락 | 친구 관계 맺기 및 목록 동기화 | `IMPLEMENTED` | `VERIFIED` (POST /friends) |
| | 즐겨찾기 | 상단 고정 즐겨찾기 친구 목록 | `IMPLEMENTED` | `VERIFIED` (is_favorite toggle) |
| | 숨김/차단 관리 | 스팸 방지 및 친구 숨김/차단 | `IMPLEMENTED` | `VERIFIED` (Block/Hide routes) |
| | 생일 친구 선물하기 | 오늘 생일인 친구 표시 및 선물 인터랙션 | `IMPLEMENTED` | `VERIFIED` (Birthday banner & gift) |
| **1:1 실시간 대화** | 텍스트 송수신 | 실시간 양방향 메시지 전달 | `IMPLEMENTED` | `VERIFIED` (WebSocket 100% Pass) |
| | 읽음 확인 (Read Receipt) | 발신자 말풍선 옆 '읽음' 표시 | `IMPLEMENTED` | `VERIFIED` (POST /read & WS event) |
| | 메시지 보내기 취소 | 24시간 이내 발송 메시지 전송 취소 | `IMPLEMENTED` | `VERIFIED` (POST /unsend within 24h) |
| | 답장 (Reply) & 인용 | 특정 메시지 타겟 답장 미리보기 | `IMPLEMENTED` | `VERIFIED` (reply_to & preview) |
| | 6종 감정 리액션 | ❤️, 😍, 😂, 😮, 😢, 👍 이모티콘 리액션 | `IMPLEMENTED` | `VERIFIED` (POST /reactions) |
| | 상단 공지 고정 | 방장/참여자의 중요 메시지 핀 배너 | `IMPLEMENTED` | `VERIFIED` (📌 공지 접기/펼치기) |
| **그룹 대화** | 그룹 생성 & 초대 | 복수 멤버 그룹방 생성 및 초대/퇴장 | `IMPLEMENTED` | `VERIFIED` (POST /conversations) |
| | 그룹 투표 (Polls) | 단일/복수 투표, 마감, 실시간 집계 | `IMPLEMENTED` | `VERIFIED` (Edge & Web synchronized) |
| | 릴레이 노트 (Notes) | 대화방 내 긴 글, 회의록 보관 및 댓글 | `IMPLEMENTED` | `VERIFIED` (Drawer notes tab) |
| | 공유 일정 (Events) | 대화방 캘린더, 일정 등록 및 RSVP 투표 | `IMPLEMENTED` | `VERIFIED` (Drawer events tab) |
| | 공유 사진첩 (Albums) | 대화방 미디어 모아보기 및 앨범 생성 | `IMPLEMENTED` | `VERIFIED` (Drawer albums tab) |
| **통화 (Calling)** | 1:1 음성통화 | WebRTC 기반 초저지연 양방향 음성통화 | `IMPLEMENTED` | `VERIFIED` (SDP/ICE Signaling PASS) |
| | 1:1 영상통화 | WebRTC 기반 HD 비디오/오디오 스트림 | `IMPLEMENTED` | `VERIFIED` (Camera/Mic toggle) |
| | 통화 시뮬레이션 UI | 펄스 링, 시간 타이머, 음소거, 스피커 | `IMPLEMENTED` | `VERIFIED` (Floating Overlay) |
| | 그룹 통화 시그널링 | 다자간 통화 세션 개설 및 참여자 브로드캐스트 | `IMPLEMENTED` | `VERIFIED` (calls/:id/signal) |
| | 통화 품질 텔레메트리 | RTT, 지터, 패킷 손실률 보고 | `IMPLEMENTED` | `VERIFIED` (POST /calls/:id/quality) |
| **VOOM & 스토리** | 24시간 스토리 | 24시간 후 만료되는 ephemeral 스토리 캐러셀 | `IMPLEMENTED` | `VERIFIED` (POST /stories, auto-expiry) |
| | 숏폼 비디오 피드 | 숏폼 영상 탐색, 좋아요, 댓글 | `IMPLEMENTED` | `VERIFIED` (VOOM tab & interactions) |
| **오픈채팅** | 공개 커뮤니티 탐색 | 카테고리별 오픈채팅방 탐색 및 검색 | `IMPLEMENTED` | `VERIFIED` (GET /community/rooms) |
| | 익명 참여 및 피드 | 독립 프로필 설정, 게시글 작성 및 공감 | `IMPLEMENTED` | `VERIFIED` (POST /posts, POST /like) |
| **지갑 & 페이** | LINE Pay 잔액 | 계좌 연동 가상 잔액 표시 (`₩50,000`) | `IMPLEMENTED` | `VERIFIED` (GET /wallet) |
| | 잔액 충전 | 원클릭 Pay 잔액 충전 모달 | `IMPLEMENTED` | `VERIFIED` (POST /wallet/charge) |
| | 친구 송금 (Remittance) | 대화방 내 송금 버블 전송 및 잔액 차감 | `IMPLEMENTED` | `VERIFIED` (POST /wallet/send) |
| | 서비스 그리드 | 선물하기, 스티커샵, 테마샵 바로가기 | `IMPLEMENTED` | `VERIFIED` (Wallet UI Grid) |
| **보관함 (Keep)** | 개인 클라우드 보관함 | 텍스트, 링크, 미디어 카테고리별 저장 | `IMPLEMENTED` | `VERIFIED` (GET/POST /saved-messages) |
| **스티커 & 테마** | 스티커 팩 (Brown/Sally) | 기본 캐릭터 및 이모션 팩 전송 트레이 | `IMPLEMENTED` | `VERIFIED` (Composer sticker picker) |
| | 커스텀 테마 5종 | Mint, Ocean, Sunset, Sakura, Midnight | `IMPLEMENTED` | `VERIFIED` (Realtime CSS theme engine) |
| **관리 콘솔** | 슈퍼 관리자 센터 | 플랫폼 KPI, 사용자 조회, 플래그, 제재 | `IMPLEMENTED` | `VERIFIED` (apps/admin-console) |
| | 파트너 콘솔 | 공식 계정, 고객센터 인박스, 캠페인 | `IMPLEMENTED` | `VERIFIED` (apps/partner-console) |
| | 개발자 포털 | 앱 등록, 봇 웹훅, LIFF 미니앱 | `IMPLEMENTED` | `VERIFIED` (apps/developer-console) |
| | 크리에이터 마켓 | 스티커/테마 등록, 정산 원장 | `IMPLEMENTED` | `VERIFIED` (apps/creator-console) |
| | 광고주 센터 | 캠페인, 광고 그룹, 타겟팅 관리 | `IMPLEMENTED` | `VERIFIED` (apps/ads-console) |
| **Android 네이티브** | Flutter Release APK | Android SDK 36 기반 릴리즈 APK 패키징 | `IMPLEMENTED` | `VERIFIED` (Messenger-Android-v1.0.0-release.apk) |
| | 시스템 권한 통합 | 인터넷, 오디오, 카메라, 포그라운드 서비스 | `IMPLEMENTED` | `VERIFIED` (AndroidManifest.xml) |

