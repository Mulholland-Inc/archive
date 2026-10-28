-- name: Revision :one
SELECT coalesce((SELECT id::text || ':' || hash FROM archive.revision ORDER BY created_at DESC, id DESC LIMIT 1), '')::text;

-- name: InsertRevision :exec
INSERT INTO archive.revision (hash) VALUES ($1);

-- name: SetLocal :exec
SELECT set_config(sqlc.arg(name), sqlc.arg(value), true);

-- name: AdvisoryLock :exec
SELECT pg_advisory_lock(sqlc.arg(key));

-- name: AdvisoryUnlock :exec
SELECT pg_advisory_unlock(sqlc.arg(key));

-- name: AdvisoryXactLock :exec
SELECT pg_advisory_xact_lock(sqlc.arg(key));

-- name: Catalog :one
WITH col AS (
    SELECT a.attrelid AS rel, a.attnum AS num, jsonb_build_object(
        'name', a.attname,
        'description', coalesce(col_description(a.attrelid, a.attnum), ''),
        'type', e.typname,
        'schema', e.typnamespace::regnamespace::text,
        'reg', format_type(e.oid, a.atttypmod),
        'list', t.typcategory = 'A' AND t.typelem <> 0,
        'not_null', a.attnotnull,
        'default', a.atthasdef,
        'generated', a.attgenerated <> '',
        'pk', EXISTS (SELECT FROM pg_catalog.pg_constraint k WHERE k.conrelid = a.attrelid AND k.contype = 'p' AND k.conkey = ARRAY[a.attnum]),
        'ref', coalesce((SELECT r.relnamespace::regnamespace::text || '.' || r.relname FROM pg_catalog.pg_constraint k JOIN pg_catalog.pg_class r ON r.oid = k.confrelid
            WHERE k.conrelid = a.attrelid AND k.contype = 'f' AND k.conkey = ARRAY[a.attnum] LIMIT 1), '')) AS c
    FROM pg_catalog.pg_attribute a
    JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
    JOIN pg_catalog.pg_type e ON e.oid = CASE WHEN t.typcategory = 'A' AND t.typelem <> 0 THEN t.typelem ELSE t.oid END
    WHERE a.attnum > 0 AND NOT a.attisdropped
)
SELECT jsonb_build_object(
    'tables', coalesce((SELECT jsonb_object_agg(c.relname, jsonb_build_object(
        'name', c.relname,
        'kind', CASE c.relkind WHEN 'v' THEN 'view' ELSE 'table' END,
        'description', coalesce(obj_description(c.oid, 'pg_class'), ''),
        'rls', c.relrowsecurity,
        'force', c.relforcerowsecurity,
        'security_invoker', coalesce((SELECT o.option_value::bool FROM pg_options_to_table(c.reloptions) o WHERE o.option_name = 'security_invoker'), false),
        'policies', (SELECT count(*) FROM pg_catalog.pg_policy p WHERE p.polrelid = c.oid),
        'columns', (SELECT jsonb_agg(col.c ORDER BY col.num) FROM col WHERE col.rel = c.oid)))
        FROM pg_catalog.pg_class c WHERE c.relnamespace = 'public'::regnamespace AND c.relkind IN ('r', 'p', 'v')), '{}'),
    'types', coalesce((SELECT jsonb_object_agg(t.typnamespace::regnamespace::text || '.' || t.typname, jsonb_build_object(
        'name', t.typname,
        'schema', t.typnamespace::regnamespace::text,
        'kind', t.typtype,
        'description', coalesce(obj_description(t.oid, 'pg_type'), ''),
        'base', coalesce((SELECT b.typname FROM pg_catalog.pg_type b WHERE b.oid = t.typbasetype), ''),
        'base_schema', coalesce((SELECT b.typnamespace::regnamespace::text FROM pg_catalog.pg_type b WHERE b.oid = t.typbasetype), ''),
        'enum', coalesce((SELECT jsonb_agg(l.enumlabel ORDER BY l.enumsortorder) FROM pg_catalog.pg_enum l WHERE l.enumtypid = t.oid), '[]'),
        'attrs', coalesce((SELECT jsonb_agg(col.c ORDER BY col.num) FROM col WHERE col.rel = t.typrelid), '[]')))
        FROM pg_catalog.pg_type t LEFT JOIN pg_catalog.pg_class r ON r.oid = t.typrelid
        WHERE t.typnamespace IN ('archive'::regnamespace, 'public'::regnamespace)
          AND (t.typtype IN ('d', 'e') OR t.typtype = 'c' AND r.relkind = 'c')), '{}'));
