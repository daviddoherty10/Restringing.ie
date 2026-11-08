-- +goose Up
CREATE TABLE compliance_cases (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),reporter_id UUID REFERENCES users(id),stringer_id UUID REFERENCES stringers(id),order_id UUID REFERENCES orders(id),
 kind TEXT NOT NULL CHECK(kind IN ('listing','order','unlawful_content')),location TEXT NOT NULL,details TEXT NOT NULL,contact TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'submitted' CHECK(status IN ('submitted','in_review','resolved','dismissed','appealed')),
 outcome TEXT NOT NULL DEFAULT '',appeal TEXT NOT NULL DEFAULT '',appealed BOOLEAN NOT NULL DEFAULT false,
 version INTEGER NOT NULL DEFAULT 1,reviewer_id UUID REFERENCES users(id),initial_reviewer_id UUID REFERENCES users(id),created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 request_key TEXT NOT NULL,request_hash TEXT NOT NULL
);
CREATE UNIQUE INDEX case_request_key ON compliance_cases(COALESCE(reporter_id::text,'public'),request_key);
CREATE TABLE compliance_case_events(id BIGSERIAL PRIMARY KEY,case_id UUID NOT NULL REFERENCES compliance_cases(id),actor_id UUID REFERENCES users(id),action TEXT NOT NULL,reason TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE compliance_notifications(id UUID PRIMARY KEY DEFAULT gen_random_uuid(),user_id UUID NOT NULL REFERENCES users(id),case_id UUID NOT NULL REFERENCES compliance_cases(id),title TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),read_at TIMESTAMPTZ);
CREATE INDEX compliance_inbox ON compliance_notifications(user_id,created_at DESC);
-- +goose Down
DROP TABLE compliance_notifications,compliance_case_events,compliance_cases;
