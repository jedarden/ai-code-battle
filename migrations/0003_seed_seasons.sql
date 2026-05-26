-- Seed the first season for AI Code Battle
-- This creates Season 1: The Founding, the initial competitive season

INSERT INTO seasons (id, name, theme, rules_version, status, champion_id, starts_at, ends_at, created_at) VALUES
(1, 'Season 1: The Founding', 'The dawn of automated competition — bots clash for the first time on symmetrical battlefields.', '1.0', 'active', NULL, NOW() - INTERVAL '7 days', NULL, NOW() - INTERVAL '7 days');
