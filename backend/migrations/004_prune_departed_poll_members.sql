-- +goose Up
DELETE FROM poll_availability pa USING poll_occurrences o, schedule_polls p
WHERE pa.occurrence_id=o.id AND o.poll_id=p.id AND p.closed_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM team_members tm
      WHERE tm.team_id=p.team_id AND tm.user_id=pa.member_id
  );

DELETE FROM poll_submissions ps USING schedule_polls p
WHERE ps.poll_id=p.id AND p.closed_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM team_members tm
      WHERE tm.team_id=p.team_id AND tm.user_id=ps.member_id
  );

DELETE FROM poll_members pm USING schedule_polls p
WHERE pm.poll_id=p.id AND p.closed_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM team_members tm
      WHERE tm.team_id=p.team_id AND tm.user_id=pm.member_id
  );

UPDATE poll_occurrences o SET status=CASE
    WHEN (SELECT count(*) FROM poll_members pm WHERE pm.poll_id=o.poll_id) =
         (SELECT count(*) FROM poll_availability pa WHERE pa.occurrence_id=o.id AND pa.available)
    THEN 'confirmed' ELSE 'attention_required' END
FROM schedule_polls p
WHERE o.poll_id=p.id AND p.closed_at IS NULL
  AND o.status IN ('confirmed', 'attention_required');

-- +goose Down
-- Removed poll membership cannot be reconstructed safely.
