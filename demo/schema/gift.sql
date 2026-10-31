CREATE TYPE gift_status AS ENUM ('idea', 'bought', 'given', 'received');
CREATE TABLE gift (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    idea text NOT NULL,
    person uuid REFERENCES person,
    occasion text,
    price archive.money,
    status gift_status,
    link archive.url,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(idea, occasion)) STORED
);
CREATE INDEX gift_search ON gift USING gin (search);
ALTER TABLE gift ENABLE ROW LEVEL SECURITY;
ALTER TABLE gift FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON gift USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE gift IS 'A gift idea or a gift given or received
icon: gift';
COMMENT ON COLUMN gift.idea IS 'The gift';
COMMENT ON COLUMN gift.person IS 'For or from whom';
COMMENT ON COLUMN gift.occasion IS 'Birthday, Sinterklaas, ...';
COMMENT ON COLUMN gift.price IS 'Price';
COMMENT ON COLUMN gift.status IS 'Where it stands';
COMMENT ON COLUMN gift.link IS 'Where to buy it';
