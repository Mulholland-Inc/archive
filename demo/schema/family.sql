CREATE VIEW family WITH (security_invoker = true) AS SELECT * FROM person WHERE relation = 'family';
COMMENT ON VIEW family IS 'icon: users';
