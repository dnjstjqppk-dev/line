# Phase 5-6: UXUI & Hardening

## Phase 5: UXUI LINE-Level Design

Priority: P1 | Effort: 4-6 weeks

### 5.1 Design System
- Create LINE-accurate color palette (green #06C755, grays, backgrounds)
- Implement LINE typography (Apple SD Gothic Neo, Malgun Gothic)
- Add LINE icon set (SVG icons matching LINE design)
- Implement spacing/sizing tokens (4px grid)
- Add elevation/shadow system
- Implement dark mode with LINE dark theme

### 5.2 Chat Interface
- Redesign message bubbles (rounded corners, tail, grouping)
- Add message grouping by sender and time
- Implement date separators between messages
- Add read receipt indicators (single/double check)
- Implement typing indicator animation
- Add message context menu (long-press/right-click)
- Implement reply preview in message bubble
- Add reaction picker (emoji grid)
- Implement message forwarding UI
- Add message search within conversation

### 5.3 Navigation & Layout
- Redesign sidebar with LINE tab bar (Friends, Chats, VOOM, OpenChat, Wallet)
- Implement bottom navigation for mobile
- Add swipe gestures for tab switching
- Implement pull-to-refresh
- Add floating action button (FAB) for new chat
- Implement conversation list with swipe actions (pin, mute, delete)

### 5.4 Rich Media
- Image preview with zoom/pan
- Video player with controls
- Audio message player with waveform
- File download progress
- Location map preview
- Contact card display
- Sticker rendering with animation

### 5.5 Animations & Transitions
- Page transition animations
- Message send/receive animations
- Modal open/close animations
- List item animations (add/remove)
- Skeleton loading states
- Pull-to-refresh animation

### 5.6 Responsive Design
- Mobile-first responsive layout
- Tablet layout (split view)
- Desktop layout (multi-column)
- Touch-friendly tap targets (44px minimum)
- Safe area handling (notch, home indicator)

### 5.7 Accessibility
- ARIA labels for all interactive elements
- Keyboard navigation (tab, enter, escape)
- Screen reader support
- Focus management in modals
- Color contrast compliance (WCAG AA)
- Reduced motion support

## Phase 6: Backend Hardening & Operations

Priority: P1 | Effort: 3-4 weeks

### 6.1 Database & Migrations
- Implement migration version tracking (golang-migrate or custom)
- Add up/down migration support
- Create migration CI check (verify all migrations apply cleanly)
- Add PostgreSQL integration tests (currently only SQLite)
- Implement connection pooling tuning (pgx pool config)
- Add query performance monitoring (pg_stat_statements)

### 6.2 Observability
- Add structured logging (zerolog or slog)
- Implement distributed tracing (OpenTelemetry)
- Add metrics (Prometheus: request latency, error rate, active connections)
- Implement health check with dependency status
- Add alerting rules (error rate, latency, connection count)
- Create Grafana dashboards

### 6.3 Rate Limiting & Security
- Implement distributed rate limiting (Redis-backed)
- Add per-user rate limits (not just per-IP)
- Implement account lockout after failed attempts
- Add CAPTCHA for registration/login
- Implement IP reputation checking
- Add request signing for API clients

### 6.4 Data Retention & Privacy
- Implement message retention policies (auto-delete after N days)
- Add data export (GDPR/PIPA compliance)
- Implement account deletion cascade (delete all user data)
- Add audit log retention (rotate after 90 days)
- Implement soft-delete with recovery window

### 6.5 Load Testing
- Create k6 or Locust load test scripts
- Test WebSocket connection scaling (10K concurrent)
- Test message throughput (1K msg/sec)
- Test database connection pooling under load
- Implement horizontal scaling test (2+ instances)