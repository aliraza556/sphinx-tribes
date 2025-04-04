INSERT INTO workspaces (
    id,
    uuid,
    name,
    owner_pub_key,
    img,
    created,
    updated,
    show,
    deleted,
    bounty_count,
    budget,
    website,
    github,
    description,
    mission,
    tactics,
    schematic_url,
    schematic_img
) VALUES
-- Workspace 1: Focused on open source developer tooling
(
    1,
    'workspace-uuid-1234',
    'Code Spaces',
    '0430a9b0f2a0bad383b1b3a1989571b90f7486a86629e040c603f6f9ecec857505fd2b1279ccce579dbe59cc88d8d49b7543bd62051b1417cafa6bb2e4fd011d30',
    'https://placehold.co/100x100',
    NOW(),
    NOW(),
    true,
    false,
    0,
    500000000,
    'https://devtoolshub.io',
    'https://github.com/stakwork/sphinx-tribes-frontend',
    'A community driven workspace building developer first open source tools.',
    'Empowering developers through better tools',
    'Open governance, fast feedback loops, reward based contributions',
    'https://example.com/schematic.pdf',
    'https://placehold.co/600x400'
),
(
    2,
    'workspace-uuid-123',
    'Owner123 Workspace',
    'owner123',
    'https://placehold.co/100x100',
    NOW(),
    NOW(),
    true,
    false,
    0,
    500000000,
    'https://devtoolshub.io',
    'https://github.com/stakwork/sphinx-tribes-frontend',
    'A community driven workspace building developer first open source tools.',
    'Empowering developers through better tools',
    'Open governance, fast feedback loops, reward based contributions',
    'https://example.com/schematic.pdf',
    'https://placehold.co/600x400'
),
(
    3,
    'workspace-uuid-456',
    'owner456 Workspace',
    'owner456',
    'https://placehold.co/100x100',
    NOW(),
    NOW(),
    true,
    false,
    0,
    500000000,
    'https://devtoolshub.io',
    'https://github.com/stakwork/sphinx-tribes-frontend',
    'A community driven workspace building developer first open source tools.',
    'Empowering developers through better tools',
    'Open governance, fast feedback loops, reward based contributions',
    'https://example.com/schematic.pdf',
    'https://placehold.co/600x400'
),

(
    3,
    'workspace-uuid-789',
    'owner789 Workspace',
    'owner789',
    'https://placehold.co/100x100',
    NOW(),
    NOW(),
    true,
    false,
    0,
    500000000,
    'https://devtoolshub.io',
    'https://github.com/stakwork/sphinx-tribes-frontend',
    'A community driven workspace building developer first open source tools.',
    'Empowering developers through better tools',
    'Open governance, fast feedback loops, reward based contributions',
    'https://example.com/schematic.pdf',
    'https://placehold.co/600x400'
);
