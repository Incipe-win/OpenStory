-- +goose Up

-- ── Admin user (password: changeme123) ──────────────
INSERT INTO users (id, email, username, password_hash, display_name, role)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'admin@openstory.local',
    'admin',
    crypt('changeme123', gen_salt('bf', 10)),
    'Administrator',
    'admin'
);

-- ── Demo user (password: demo123456) ────────────────
INSERT INTO users (id, email, username, password_hash, display_name, role)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    'demo@openstory.local',
    'demo',
    crypt('demo123456', gen_salt('bf', 10)),
    'Demo User',
    'user'
);

-- ── Credit accounts with initial balance ────────────
INSERT INTO credit_accounts (user_id, balance, total_earned)
VALUES
    ('00000000-0000-0000-0000-000000000001', 10000, 10000),
    ('00000000-0000-0000-0000-000000000002', 500, 500);

-- ── Initial credit ledger entries ───────────────────
INSERT INTO credit_ledger (account_id, user_id, type, amount, balance_after, reference_type, description)
VALUES
    ((SELECT id FROM credit_accounts WHERE user_id = '00000000-0000-0000-0000-000000000001'),
     '00000000-0000-0000-0000-000000000001', 'bonus', 10000, 10000, 'admin', 'Initial admin credits'),
    ((SELECT id FROM credit_accounts WHERE user_id = '00000000-0000-0000-0000-000000000002'),
     '00000000-0000-0000-0000-000000000002', 'bonus', 500, 500, 'admin', 'Welcome bonus');

-- ── Sample project ──────────────────────────────────
INSERT INTO projects (id, user_id, name, description, status)
VALUES (
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000002',
    'My First Story',
    'A sample project to explore OpenStory features.',
    'draft'
);

-- ── Audit log for seed ──────────────────────────────
INSERT INTO audit_logs (user_id, action, resource_type, resource_id, ip_address)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'seed', 'system', NULL, '127.0.0.1');


-- +goose Down
DELETE FROM audit_logs   WHERE action = 'seed';
DELETE FROM credit_ledger WHERE description IN ('Initial admin credits', 'Welcome bonus');
DELETE FROM credit_accounts WHERE user_id IN (
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000002'
);
DELETE FROM projects WHERE id = '00000000-0000-0000-0000-000000000010';
DELETE FROM users WHERE id IN (
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000002'
);
