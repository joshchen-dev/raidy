-- +goose Up
INSERT INTO poll_members (poll_id, member_id)
SELECT p.id, tm.user_id
FROM schedule_polls p
JOIN team_members tm ON tm.team_id=p.team_id
WHERE p.closed_at IS NULL
ON CONFLICT DO NOTHING;

UPDATE poll_occurrences o SET status=CASE
    WHEN (SELECT count(*) FROM poll_members pm WHERE pm.poll_id=o.poll_id) =
         (SELECT count(*) FROM poll_availability pa WHERE pa.occurrence_id=o.id AND pa.available)
    THEN 'confirmed' ELSE 'attention_required' END
FROM schedule_polls p
WHERE o.poll_id=p.id AND p.closed_at IS NULL
  AND o.status IN ('confirmed', 'attention_required');

-- +goose Down
-- Added poll membership cannot be distinguished from the original snapshot safely.
