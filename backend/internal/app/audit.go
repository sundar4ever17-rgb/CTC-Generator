package app

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

func auditTx(ctx context.Context, tx pgx.Tx, actor, action, kind, id string, details any) error {
	b, e := json.Marshal(details)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "INSERT INTO audit_events(id,actor_id,action,entity_type,entity_id,details) VALUES($1,$2,$3,$4,$5,$6)", ID(), nullable(actor), action, kind, id, b)
	return e
}
