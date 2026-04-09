DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
          AND table_name = 'user_bot_leaderboard_settings'
    ) THEN
        BEGIN
            ALTER TABLE user_bot_leaderboard_settings
                ALTER COLUMN user_id TYPE TEXT USING user_id::text;
        EXCEPTION WHEN undefined_column THEN
            -- no-op
        END;
    ELSE
        CREATE TABLE user_bot_leaderboard_settings (
            id BIGSERIAL PRIMARY KEY,
            user_id TEXT NOT NULL,
            bot_id TEXT NOT NULL,
            is_public BOOLEAN NOT NULL DEFAULT FALSE,
            share_strategy BOOLEAN NOT NULL DEFAULT FALSE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (user_id, bot_id)
        );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_user_bot_leaderboard_settings_user
    ON user_bot_leaderboard_settings(user_id);

CREATE INDEX IF NOT EXISTS idx_user_bot_leaderboard_settings_public
    ON user_bot_leaderboard_settings(is_public, updated_at DESC);
