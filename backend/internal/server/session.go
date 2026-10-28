package server

import (
	"context"
	"github.com/example/global-messenger/backend/internal/auth"
)

func (a *App) sessionActive(ctx context.Context, userID, sessionID string) bool {
	if !auth.ValidUUID(sessionID) {
		return false
	}
	var id string
	return a.db.QueryRowContext(ctx, `SELECT s.id FROM identity.sessions s
 JOIN identity.devices d ON d.id=s.device_id AND d.user_id=s.user_id
 JOIN identity.users u ON u.id=s.user_id
 WHERE s.id=$1 AND s.user_id=$2 AND s.revoked_at IS NULL AND s.expires_at > now()
 AND d.revoked_at IS NULL AND u.state='active'`, sessionID, userID).Scan(&id) == nil
}
