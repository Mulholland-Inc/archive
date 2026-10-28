-- name: DatabaseExists :one
SELECT EXISTS (SELECT FROM pg_catalog.pg_database WHERE datname = 'archive');

-- name: CreateDatabase :exec
CREATE DATABASE archive;
