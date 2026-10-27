CREATE TYPE book_status AS ENUM ('want', 'reading', 'read', 'abandoned');
CREATE TABLE book (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    author text,
    isbn archive.isbn,
    status book_status,
    rating smallint CHECK (rating BETWEEN 1 AND 5),
    finished date,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, author, notes)) STORED
);
CREATE INDEX book_search ON book USING gin (search);
ALTER TABLE book ENABLE ROW LEVEL SECURITY;
ALTER TABLE book FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON book FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON book USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE book IS 'icon: book-open';
