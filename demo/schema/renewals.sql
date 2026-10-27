CREATE VIEW renewals WITH (security_invoker = true) AS SELECT * FROM subscription;
COMMENT ON VIEW renewals IS 'Subscriptions by when they renew
layout: calendar
by: renews
icon: calendar-sync';
