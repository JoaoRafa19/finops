-- name: GetUserSettings :one
SELECT user_id, home_mode, updated_at
FROM user_settings
WHERE user_id = $1;

-- name: UpsertUserHomeMode :exec
INSERT INTO user_settings (user_id, home_mode, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (user_id) DO UPDATE SET
    home_mode = EXCLUDED.home_mode,
    updated_at = now();
