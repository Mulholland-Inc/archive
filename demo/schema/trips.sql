CREATE VIEW trips WITH (security_invoker = true) AS SELECT * FROM trip;
COMMENT ON VIEW trips IS 'Trips by when they start
layout: calendar
by: starts
icon: luggage';
