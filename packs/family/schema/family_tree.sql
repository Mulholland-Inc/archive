CREATE VIEW family_tree WITH (security_invoker = true) AS
SELECT p.*,
    p.died IS NULL AND p.died_year IS NULL AND (p.born_year IS NULL OR p.born_year > extract(year FROM now()) - 110) AS living,
    ARRAY(SELECT l.partnership FROM parent_link l WHERE l.child = p.id ORDER BY l.created_at) AS parents
FROM person p;
COMMENT ON VIEW family_tree IS 'The family, oldest at the top
layout: tree
icon: network';
