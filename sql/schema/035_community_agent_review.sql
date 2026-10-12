-- +goose Up
-- Existing posts keep their visibility without pretending they were AI-reviewed.
ALTER TABLE community_posts
 ADD COLUMN review_status text NOT NULL DEFAULT 'legacy' CHECK(review_status IN ('legacy','pending','reviewing','approved','rejected','unavailable')),
 ADD COLUMN review_version int NOT NULL DEFAULT 1 CHECK(review_version>0),
 ADD COLUMN review_note text NOT NULL DEFAULT '',
 ADD COLUMN review_token uuid,
 ADD COLUMN review_until timestamptz,
 ADD COLUMN reviewed_at timestamptz,
 ADD COLUMN cover_reviewed boolean NOT NULL DEFAULT false;
CREATE INDEX community_review_queue ON community_posts(created_at,id) WHERE deleted_at IS NULL AND review_status='pending';
CREATE TABLE community_review_calls (
 post_id uuid NOT NULL REFERENCES community_posts(id), version int NOT NULL,
 author_id uuid NOT NULL REFERENCES users(id), kind text NOT NULL CHECK(kind IN ('text','image')),
 month date NOT NULL, ceiling_milli bigint NOT NULL CHECK(ceiling_milli>0),
 state text NOT NULL CHECK(state IN ('pending','completed','uncertain')),
 created_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 PRIMARY KEY(post_id,version)
);

-- +goose Down
-- Never turn non-public submissions into public posts through a downgrade.
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM community_posts WHERE review_status NOT IN ('legacy','approved')) THEN
  RAISE EXCEPTION 'Unpublished community submissions exist; preserve them before downgrade';
 END IF;
END $$;
-- +goose StatementEnd
DROP TABLE community_review_calls;
DROP INDEX community_review_queue;
ALTER TABLE community_posts DROP COLUMN review_status, DROP COLUMN review_version,
 DROP COLUMN review_note, DROP COLUMN review_token, DROP COLUMN review_until,
 DROP COLUMN reviewed_at, DROP COLUMN cover_reviewed;
