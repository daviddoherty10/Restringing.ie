-- +goose Up
-- Tracking changes are append-only snapshots. Business/order locks serialize
-- their version checks with fulfilment; money and inventory stay in orders.
CREATE TABLE order_tracking_updates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 100),
    kind TEXT NOT NULL CHECK (kind IN ('collection_request','stringer_update')),
    request JSONB NOT NULL,
    state JSONB NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    note TEXT NOT NULL CHECK (length(note) BETWEEN 1 AND 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(order_id,actor_id,idempotency_key),
    UNIQUE(order_id,version)
);
CREATE INDEX order_tracking_updates_actor_idx ON order_tracking_updates(actor_id,created_at);

CREATE TABLE order_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position BIGSERIAL NOT NULL UNIQUE,
    order_id UUID NOT NULL REFERENCES orders(id),
    sender_id UUID NOT NULL REFERENCES users(id),
    sender_role TEXT NOT NULL CHECK (sender_role IN ('customer','stringer','support')),
    body TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 2000),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(order_id,sender_id,idempotency_key)
);
CREATE INDEX order_messages_order_idx ON order_messages(order_id,position DESC);
CREATE INDEX order_messages_sender_idx ON order_messages(sender_id,created_at);
CREATE TABLE order_message_reads (
    order_id UUID NOT NULL REFERENCES orders(id),
    user_id UUID NOT NULL REFERENCES users(id),
    position BIGINT NOT NULL CHECK (position > 0),
    PRIMARY KEY(order_id,user_id)
);

-- In-app delivery is atomic with the originating change. Outbound SMS/email
-- delivery is deliberately not enabled by this migration.
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    order_id UUID NOT NULL REFERENCES orders(id),
    event_key TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('new_order','status','message','tracking')),
    title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    read_at TIMESTAMPTZ,
    UNIQUE(user_id,event_key)
);
CREATE INDEX notifications_user_idx ON notifications(user_id,created_at DESC,id);
CREATE INDEX notifications_unread_idx ON notifications(user_id) WHERE read_at IS NULL;

-- +goose Down
DROP TABLE notifications, order_message_reads, order_messages, order_tracking_updates;
