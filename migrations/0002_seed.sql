INSERT INTO teams (id, name) VALUES
    (1, 'Default Team'),
    (2, 'Second Team')
ON CONFLICT (id) DO NOTHING;
