CREATE VIEW renewals WITH (security_invoker = true) AS SELECT * FROM subscription;
COMMENT ON VIEW renewals IS 'layout: calendar
by: renews
icon: calendar-sync';
