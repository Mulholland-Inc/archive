CREATE VIEW doctors WITH (security_invoker = true) AS SELECT * FROM person WHERE relation = 'doctor';
COMMENT ON VIEW doctors IS 'Your doctors, dentists and other carers
icon: stethoscope';
