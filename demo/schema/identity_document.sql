CREATE TYPE document_kind AS ENUM ('passport', 'id_card', 'drivers_license', 'residence_permit', 'health_card');
CREATE TABLE identity_document (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    kind document_kind NOT NULL,
    number archive.passport,
    holder uuid REFERENCES person,
    country archive.country,
    issued date,
    expires date,
    scan uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(country)) STORED
);
CREATE INDEX identity_document_search ON identity_document USING gin (search);
ALTER TABLE identity_document ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_document FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON identity_document FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON identity_document USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE identity_document IS 'A passport, ID card, driver''s license or permit
icon: id-card';
COMMENT ON COLUMN identity_document.kind IS 'Type of document';
COMMENT ON COLUMN identity_document.number IS 'Document number';
COMMENT ON COLUMN identity_document.holder IS 'Whose document it is';
COMMENT ON COLUMN identity_document.country IS 'Issuing country';
COMMENT ON COLUMN identity_document.issued IS 'Issue date';
COMMENT ON COLUMN identity_document.expires IS 'Expiry date';
COMMENT ON COLUMN identity_document.scan IS 'Scan of the document';
