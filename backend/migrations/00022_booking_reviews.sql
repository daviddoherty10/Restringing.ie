-- +goose Up
CREATE TABLE booking_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT NOT NULL CHECK (char_length(comment) <= 500),
    withdrawn BOOLEAN NOT NULL DEFAULT false,
    hidden BOOLEAN NOT NULL DEFAULT false,
    report_reason TEXT CHECK (char_length(report_reason) BETWEEN 1 AND 500),
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX booking_reviews_report_idx ON booking_reviews(created_at,id) WHERE report_reason IS NOT NULL AND NOT withdrawn;

-- +goose Down
DROP TABLE booking_reviews;
