-- +goose Up
-- Existing households remain shared; provisioning never moves existing stock.
ALTER TABLE households ADD COLUMN kind text NOT NULL DEFAULT 'family' CHECK (kind IN ('personal','family'));
CREATE TABLE personal_kitchens (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    household_id uuid NOT NULL UNIQUE REFERENCES households(id)
);

-- +goose Down
-- Refuse a downgrade that would silently expose private kitchens as families.
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM households WHERE kind='personal') THEN
        RAISE EXCEPTION 'Personal kitchens exist; preserve or migrate them before downgrade';
    END IF;
END $$;
-- +goose StatementEnd
DROP TABLE personal_kitchens;
ALTER TABLE households DROP COLUMN kind;
