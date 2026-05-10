-- name: CreateImportRun :one
INSERT INTO import_runs (user_id, account_id, provider, source_ref, metadata)
VALUES (@user_id, @account_id, @provider, @source_ref, @metadata)
RETURNING *;

-- name: UpdateImportRunCounts :exec
UPDATE import_runs
SET rows_parsed    = @rows_parsed,
    rows_inserted  = @rows_inserted,
    rows_duplicate = @rows_duplicate,
    rows_failed    = @rows_failed
WHERE id = @id;

-- name: FinishImportRun :exec
UPDATE import_runs
SET status       = @status,
    finished_at  = NOW(),
    error_detail = @error_detail
WHERE id = @id;

-- name: ListImportRuns :many
SELECT * FROM import_runs WHERE user_id = @user_id ORDER BY started_at DESC LIMIT 20;
