-- +goose Up
CREATE TABLE community_posts (
 id uuid PRIMARY KEY, author_id uuid NOT NULL REFERENCES users(id),
 title text NOT NULL CHECK(char_length(title) BETWEEN 2 AND 60),
 story text NOT NULL DEFAULT '' CHECK(char_length(story)<=1000),
 servings int NOT NULL CHECK(servings BETWEEN 1 AND 20),
 minutes int NOT NULL CHECK(minutes BETWEEN 1 AND 240),
 tags text[] NOT NULL, ingredients jsonb NOT NULL, steps text[] NOT NULL,
 fingerprint text NOT NULL, cover bytea, cover_type text NOT NULL DEFAULT '',
 recipe_id uuid UNIQUE REFERENCES recipes(id),
 created_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
 UNIQUE(author_id,fingerprint), CHECK(octet_length(cover)<=2097152)
);
CREATE INDEX community_posts_recent ON community_posts(created_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE TABLE community_reactions (
 post_id uuid NOT NULL REFERENCES community_posts(id), user_id uuid NOT NULL REFERENCES users(id),
 kind text NOT NULL CHECK(kind IN ('like','save')), active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(post_id,user_id,kind)
);
CREATE INDEX community_week_votes ON community_reactions(created_at,post_id) WHERE kind='like' AND active;
CREATE TABLE community_weeks (week_start date PRIMARY KEY, finalized_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE community_candidates (
 id uuid PRIMARY KEY, week_start date NOT NULL REFERENCES community_weeks(week_start),
 post_id uuid NOT NULL REFERENCES community_posts(id), author_id uuid NOT NULL REFERENCES users(id),
 rank int NOT NULL CHECK(rank BETWEEN 1 AND 3), votes int NOT NULL CHECK(votes>0),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','accepted','rejected')),
 review_note text NOT NULL DEFAULT '', reviewed_by uuid REFERENCES users(id), reviewed_at timestamptz,
 UNIQUE(week_start,post_id), UNIQUE(week_start,author_id), UNIQUE(week_start,rank)
);
CREATE TABLE community_points (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id),
 amount int NOT NULL CHECK(amount<>0), reason text NOT NULL, event_key text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(user_id,event_key)
);
CREATE INDEX community_points_account ON community_points(user_id,created_at DESC);
-- +goose StatementBegin
CREATE FUNCTION prevent_community_points_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'community points are append only'; END $$;
-- +goose StatementEnd
CREATE TRIGGER community_points_immutable BEFORE UPDATE OR DELETE ON community_points
FOR EACH ROW EXECUTE FUNCTION prevent_community_points_mutation();
CREATE TABLE community_rewards (
 user_id uuid NOT NULL REFERENCES users(id), reward text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(user_id,reward)
);
CREATE TABLE community_profiles (
 user_id uuid PRIMARY KEY REFERENCES users(id),
 badge text NOT NULL DEFAULT '', frame text NOT NULL DEFAULT '', theme text NOT NULL DEFAULT 'forest'
);
CREATE TABLE community_replays (
 user_id uuid NOT NULL REFERENCES users(id), key text NOT NULL,
 request_hash text NOT NULL, response jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,key)
);
-- +goose Down
DROP TABLE community_replays,community_profiles,community_rewards;
DROP TABLE community_points;
DROP FUNCTION prevent_community_points_mutation;
DROP TABLE community_candidates,community_weeks,community_reactions,community_posts;
