CREATE VIEW trips WITH (security_invoker = true) AS SELECT * FROM trip;
COMMENT ON VIEW trips IS 'layout: calendar
by: starts
icon: luggage';
