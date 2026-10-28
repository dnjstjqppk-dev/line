# Phase 3-4: E2EE & Offline Queue

## Phase 3: E2EE (End-to-End Encryption)

Priority: P0 | Effort: 4-6 weeks

### 3.1 Signal Protocol Implementation (Client)
- Implement X3DH (Extended Triple Diffie-Hellman) key agreement
- Implement Double Ratchet algorithm (sending/receiving chains)
- Implement pre-key bundle upload/download
- Implement session establishment and management
- Add message key derivation and ratcheting
- Implement skipped message key storage (out-of-order delivery)
- Add forward secrecy (delete old keys)

### 3.2 Device Key Management
- Generate identity key pair per device (already has uploadDeviceKeys)
- Generate signed pre-keys (rotate weekly)
- Generate one-time pre-keys (batch of 100, replenish)
- Implement key verification (QR code / safety number)
- Add device verification UI (scan QR, compare fingerprints)

### 3.3 Group E2EE (Sender Keys)
- Implement sender key distribution
- Add group key rotation on member add/remove
- Implement group message encryption/decryption
- Handle out-of-order group messages

### 3.4 Encrypted Backup
- Implement encrypted backup with user passphrase
- Add key derivation (Argon2id or PBKDF2)
- Store encrypted backup in cloud storage
- Implement backup restore flow
- Add backup verification (checksum)

### 3.5 Server-Side Changes for E2EE
- Ensure server never stores plaintext when ciphertext provided
- Add encryption_mode validation (reject none for E2EE conversations)
- Implement key transparency / audit log for key changes
- Add crypto.key_change_events handling
- Remove server-readable body from search

### 3.6 Key Verification UI
- QR code generation for identity key
- QR code scanning for verification
- Safety number display and comparison
- Verified contact badge

## Phase 4: Offline Queue & Sync

Priority: P1 | Effort: 2-3 weeks

### 4.1 Client-Side Offline Queue
- IndexedDB message queue (already scaffolded in messenger-utils.ts)
- Queue outgoing messages when offline
- Queue message status updates (read receipts)
- Queue reactions and poll votes
- Implement retry with exponential backoff
- Add queue size limit (prevent unbounded growth)
- Implement queue persistence across browser restarts

### 4.2 Reconnection & Catch-Up
- Implement WebSocket reconnection with exponential backoff
- Add syncMessages call on reconnect (after_sequence cursor)
- Implement conversation list refresh on reconnect
- Add read cursor sync (last_read_sequence)
- Implement presence re-establishment
- Add typing indicator recovery

### 4.3 Conflict Resolution
- Handle duplicate message detection (client_id idempotency)
- Implement last-write-wins for concurrent edits
- Add merge strategy for offline edits
- Handle message deletion conflicts

### 4.4 Offline-First Architecture
- Cache conversation list in IndexedDB
- Cache recent messages per conversation
- Implement stale-while-revalidate for message list
- Add offline indicator in UI
- Implement optimistic UI updates with rollback