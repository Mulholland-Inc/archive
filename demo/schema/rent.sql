CREATE VIEW rent WITH (security_invoker = true) AS SELECT * FROM payment WHERE category = 'rent';
COMMENT ON VIEW rent IS 'Rent you paid
icon: house';
