CREATE VIEW things_by_category WITH (security_invoker = true) AS SELECT * FROM possession;
COMMENT ON VIEW things_by_category IS 'Your things, grouped by category
layout: board
by: category
icon: tags';
