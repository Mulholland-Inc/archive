CREATE TYPE book_status AS ENUM ('want', 'reading', 'read', 'abandoned');
CREATE TABLE book (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
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
CREATE POLICY access ON book USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE book IS 'A book you read, are reading or want to read
icon: book-open';
COMMENT ON COLUMN book.title IS 'Title';
COMMENT ON COLUMN book.author IS 'Author';
COMMENT ON COLUMN book.isbn IS 'ISBN-13';
COMMENT ON COLUMN book.status IS 'Reading status';
COMMENT ON COLUMN book.rating IS 'Your rating, 1 to 5';
COMMENT ON COLUMN book.finished IS 'Date you finished it';
COMMENT ON COLUMN book.notes IS 'Thoughts and quotes';
