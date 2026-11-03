-- The personal store: each person's own connections, like their Gmail, and what they sync, kept once in the
-- main database for every workspace they are in (personal.go). Hub.migrate applies it with main.sql, by diff, at
-- startup. Its tables are archive.sql's of the same names, column for column, so the syncs, list, get and
-- archive.query read and write them as they do a workspace's, but for whose a row is: a connection's member is
-- the id of the person (archive."user") it is theirs, not of a workspace's member, and nothing has a group.
--
-- A person reads them as {p}_personal, the only role they are granted to, which {p}_authenticator becomes for
-- them, with archive.person saying who they are, signed with personal_key: a row shows only to whose it is
-- (archive.me). A read can set archive.person, or the role, to anything, but cannot sign as anyone else, and
-- the authenticator is no other role. No workspace's roles are granted anything here, so no member, owner or
-- admin of any workspace reads another person's. Go writes them as the main database's owner, which
-- row-level security lets through, as it does a workspace's.
--
-- A workspace keeps its own tables of these names for its integrations; its mail_message, mail_party,
-- mail_attachment and doc_page stay empty, since only personal connectors sync into them.

-- personal_key is the secret archive.person is signed with, from the server's key (Hub.openSelf). Only Go and
-- archive.me read it.
CREATE TABLE archive.personal_key (
    id  boolean PRIMARY KEY DEFAULT true CONSTRAINT personal_key_one CHECK (id),
    key text NOT NULL
);

CREATE DOMAIN archive.email AS text CHECK (VALUE ~ '^[^@\s]+@[^@\s]+$');
COMMENT ON DOMAIN archive.email IS 'Email address; the domain is lowercase and punycode';

CREATE DOMAIN archive.url AS text;
COMMENT ON DOMAIN archive.url IS 'Web address, WHATWG-normalized';

-- object is archive.sql's: what every row is. No role reads it.
CREATE TABLE archive.object (
    id         uuid NOT NULL DEFAULT uuidv7(),
    name       text NOT NULL DEFAULT '',
    access     uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE archive.object ENABLE ROW LEVEL SECURITY;
ALTER TABLE archive.object FORCE ROW LEVEL SECURITY;
COMMENT ON TABLE archive.object IS 'What every row is: its id, name, access and when it was made and changed. Every table inherits it; it is read through them, never itself';
COMMENT ON COLUMN archive.object.name IS 'The row''s title, which every screen shows it and its links by';

-- A connection is one of a person's own accounts, as archive.sql has it: member is the person's id, integration
-- is never true, and a person has one connection to each account, however many workspaces they are in.
CREATE TABLE archive.connection (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    member     uuid NOT NULL,
    provider   text NOT NULL,
    account    text NOT NULL,
    token      bytea NOT NULL,
    scopes     text[] NOT NULL DEFAULT '{}',
    error      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    meta       jsonb NOT NULL DEFAULT '{}',
    sync_at    timestamptz DEFAULT now(),
    synced_at  timestamptz,
    cursor     jsonb NOT NULL DEFAULT '{}',
    asked_at   timestamptz NOT NULL DEFAULT now(),
    lease_at   timestamptz,
    revoked    boolean NOT NULL DEFAULT false,
    integration boolean NOT NULL DEFAULT false CONSTRAINT connection_personal CHECK (NOT integration),
    name       text NOT NULL DEFAULT '',
    type       archive.workspace_type NOT NULL,
    UNIQUE (member, provider, account)
) INHERITS (archive.object);
CREATE INDEX connection_sync_idx ON archive.connection (sync_at) WHERE sync_at IS NOT NULL;
COMMENT ON TABLE archive.connection IS 'One of your own accounts at another service that Archive reads for you, like your Gmail; archive.action uses it. Read only; connect and disconnect them in Settings, Connectors
icon: plug';
COMMENT ON COLUMN archive.connection.provider IS 'Which service, a connector: gmail, notion, quickbooks, ...';
COMMENT ON COLUMN archive.connection.meta IS 'What the service said about the account when it connected, like QuickBooks'' realmId';
COMMENT ON COLUMN archive.connection.account IS 'The account there, like the mailbox address';
COMMENT ON COLUMN archive.connection.scopes IS 'What Archive may do there, as the service granted it';
COMMENT ON COLUMN archive.connection.error IS 'Why Archive last failed to reach it, like a revoked grant; empty while it works';
COMMENT ON COLUMN archive.connection.sync_at IS 'When Archive next reads what changed there; null once it has it all, until the service says something changed, you ask, or you read what it syncs';
COMMENT ON COLUMN archive.connection.synced_at IS 'When Archive last had everything there';
COMMENT ON COLUMN archive.connection.type IS 'Which archives it is in: only those of this type, personal, family or business, read what it syncs and act with it. A connection takes the type of the archive it was connected in, and its person changes it in Settings, Connectors';
COMMENT ON COLUMN archive.connection.integration IS 'It is the archive''s, like its QuickBooks, which an owner connects for everyone, not one person''s own';

CREATE TABLE archive.known_person (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      archive.email NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    name       text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
) INHERITS (archive.object);
COMMENT ON TABLE archive.known_person IS 'A person your connections know, by their email address: one row however many connections know them, like a Gmail sender who is also a Gusto employee. Read only; Archive syncs them, and their names are where they were seen, like mail_party and payroll_worker
icon: contact';
COMMENT ON COLUMN archive.known_person.email IS 'Their address, lowercase: what makes them one person everywhere';

CREATE TABLE archive.mail_message (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    connection  uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    stream      text NOT NULL,
    source_id   text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    name        text NOT NULL DEFAULT '',
    seen_at     timestamptz NOT NULL DEFAULT now(),
    thread      text,
    subject     text,
    snippet     text,
    text        text,
    sent_at     timestamptz,
    received_at timestamptz,
    labels      text[],
    UNIQUE (connection, stream, source_id)
) INHERITS (archive.object);
COMMENT ON TABLE archive.mail_message IS 'A message of a mailbox you connected, like your Gmail; who it is from and to is in mail_party, and its files in mail_attachment. Read only; Archive syncs them
icon: mail';
COMMENT ON COLUMN archive.mail_message.thread IS 'The conversation it is part of, its id there';
COMMENT ON COLUMN archive.mail_message.snippet IS 'The start of its text, as the mailbox shows it';
COMMENT ON COLUMN archive.mail_message.text IS 'Its text, or the words of its HTML when it has none, up to 64 KB';
COMMENT ON COLUMN archive.mail_message.sent_at IS 'When it was sent, as its Date says, or else when it arrived';
COMMENT ON COLUMN archive.mail_message.received_at IS 'When the mailbox had it';
COMMENT ON COLUMN archive.mail_message.labels IS 'Its labels or folders there, like INBOX';
COMMENT ON COLUMN archive.mail_message.connection IS 'references connection; the account it is from';
COMMENT ON COLUMN archive.mail_message.stream IS 'What its connector syncs it as, like messages';
COMMENT ON COLUMN archive.mail_message.source_id IS 'Its id there';
COMMENT ON COLUMN archive.mail_message.updated_at IS 'When Archive last saw it change';

CREATE TABLE archive.mail_party (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    connection uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    message    uuid NOT NULL REFERENCES archive.mail_message ON DELETE CASCADE,
    person     uuid NOT NULL REFERENCES archive.known_person ON DELETE CASCADE,
    role       text NOT NULL CONSTRAINT mail_party_role CHECK (role IN ('from', 'to', 'cc', 'bcc', 'reply_to')),
    name       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
) INHERITS (archive.object);
CREATE INDEX mail_party_message_idx ON archive.mail_party (message);
CREATE INDEX mail_party_person_idx ON archive.mail_party (person);
CREATE INDEX mail_party_connection_idx ON archive.mail_party (connection);
COMMENT ON TABLE archive.mail_party IS 'Someone a message is from or to. Read only; Archive syncs them
icon: at-sign';
COMMENT ON COLUMN archive.mail_party.message IS 'references mail_message';
COMMENT ON COLUMN archive.mail_party.person IS 'references known_person; who, by their address';
COMMENT ON COLUMN archive.mail_party.role IS 'from, to, cc, bcc or reply_to';
COMMENT ON COLUMN archive.mail_party.name IS 'The name the message gives them';

CREATE TABLE archive.mail_attachment (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    connection uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    name       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    message    uuid NOT NULL REFERENCES archive.mail_message ON DELETE CASCADE,
    filename   text,
    mime       text,
    size       bigint
) INHERITS (archive.object);
CREATE INDEX mail_attachment_message_idx ON archive.mail_attachment (message);
CREATE INDEX mail_attachment_connection_idx ON archive.mail_attachment (connection);
COMMENT ON TABLE archive.mail_attachment IS 'A file attached to a message; its content stays in the mailbox. Read only; Archive syncs them
icon: paperclip';
COMMENT ON COLUMN archive.mail_attachment.message IS 'references mail_message';
COMMENT ON COLUMN archive.mail_attachment.mime IS 'Its media type, like application/pdf';
COMMENT ON COLUMN archive.mail_attachment.size IS 'Its size in bytes';

CREATE TABLE archive.doc_page (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    connection uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    stream     text NOT NULL,
    source_id  text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name       text NOT NULL DEFAULT '',
    seen_at    timestamptz NOT NULL DEFAULT now(),
    kind       text,
    title      text,
    url        archive.url,
    icon       text,
    edited_at  timestamptz,
    UNIQUE (connection, stream, source_id)
) INHERITS (archive.object);
COMMENT ON TABLE archive.doc_page IS 'A page of docs or a wiki you connected, like Notion; its content stays there. Read only; Archive syncs them
icon: file-text';
COMMENT ON COLUMN archive.doc_page.kind IS 'What it is there, like page or database';
COMMENT ON COLUMN archive.doc_page.icon IS 'Its emoji, or its image''s URL';
COMMENT ON COLUMN archive.doc_page.edited_at IS 'When it was last edited, when the service says';
COMMENT ON COLUMN archive.doc_page.connection IS 'references connection; the account it is from';
COMMENT ON COLUMN archive.doc_page.stream IS 'What its connector syncs it as, like messages';
COMMENT ON COLUMN archive.doc_page.source_id IS 'Its id there';
COMMENT ON COLUMN archive.doc_page.updated_at IS 'When Archive last saw it change';

-- me is the person a read of the store is for, and my_type the type of the archive they read it in: archive.person,
-- <their id>.<the type>.<its signature>, when it is signed with personal_key, and null otherwise.
CREATE FUNCTION archive.me() RETURNS uuid LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT s.who::uuid FROM (SELECT split_part(current_setting('archive.person', true), '.', 1) AS who,
        split_part(current_setting('archive.person', true), '.', 2) AS typ,
        split_part(current_setting('archive.person', true), '.', 3) AS sig) s, archive.personal_key k
    WHERE s.sig = encode(sha256(convert_to(k.key || s.who || '.' || s.typ, 'UTF8')), 'hex')
$$;

CREATE FUNCTION archive.my_type() RETURNS archive.workspace_type LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT s.typ::archive.workspace_type FROM (SELECT split_part(current_setting('archive.person', true), '.', 1) AS who,
        split_part(current_setting('archive.person', true), '.', 2) AS typ,
        split_part(current_setting('archive.person', true), '.', 3) AS sig) s, archive.personal_key k
    WHERE s.sig = encode(sha256(convert_to(k.key || s.who || '.' || s.typ, 'UTF8')), 'hex')
$$;

-- object_name is archive.sql's, for the tables here.
CREATE FUNCTION archive.object_name() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    r jsonb := to_jsonb(NEW);
    own text := nullif(btrim(NEW.name), '');
BEGIN
    NEW.name := coalesce(nullif(btrim(CASE TG_TABLE_NAME
        WHEN 'connection' THEN concat_ws(' · ', nullif(r->>'account', ''), r->>'provider')
        WHEN 'known_person' THEN r->>'email'
        WHEN 'mail_message' THEN coalesce(nullif(btrim(r->>'subject'), ''), nullif(left(btrim(r->>'snippet'), 120), ''), 'No subject')
        WHEN 'mail_party' THEN coalesce(own, (SELECT p.email FROM archive.known_person p WHERE p.id = (r->>'person')::uuid))
        WHEN 'mail_attachment' THEN r->>'filename'
        WHEN 'doc_page' THEN r->>'title'
        ELSE own END), ''), '');
    RETURN NEW;
END $$;

CREATE TRIGGER object_name BEFORE INSERT OR UPDATE ON archive.connection FOR EACH ROW EXECUTE FUNCTION archive.object_name();
CREATE TRIGGER object_name BEFORE INSERT OR UPDATE ON archive.known_person FOR EACH ROW EXECUTE FUNCTION archive.object_name();
CREATE TRIGGER object_name BEFORE INSERT OR UPDATE ON archive.mail_message FOR EACH ROW EXECUTE FUNCTION archive.object_name();
CREATE TRIGGER object_name BEFORE INSERT OR UPDATE ON archive.mail_party FOR EACH ROW EXECUTE FUNCTION archive.object_name();
CREATE TRIGGER object_name BEFORE INSERT OR UPDATE ON archive.mail_attachment FOR EACH ROW EXECUTE FUNCTION archive.object_name();
CREATE TRIGGER object_name BEFORE INSERT OR UPDATE ON archive.doc_page FOR EACH ROW EXECUTE FUNCTION archive.object_name();

GRANT USAGE ON SCHEMA archive TO "{p}_personal";
GRANT SELECT ON archive.connection, archive.known_person, archive.mail_message, archive.mail_party, archive.mail_attachment, archive.doc_page TO "{p}_personal";

-- A row is seen by whose connection it is from, and a person with a row that names them. personal_key has
-- row-level security and no policy, so a read past its grants still sees nothing.
ALTER TABLE archive.personal_key ENABLE ROW LEVEL SECURITY;
ALTER TABLE archive.connection ENABLE ROW LEVEL SECURITY;
-- A connection, and so everything it synced, shows only in archives of its type.
CREATE POLICY mine ON archive.connection FOR SELECT USING (member = archive.me() AND type = archive.my_type());
ALTER TABLE archive.mail_message ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.mail_message FOR SELECT USING (EXISTS (SELECT FROM archive.connection c WHERE c.id = mail_message.connection));
ALTER TABLE archive.mail_party ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.mail_party FOR SELECT USING (EXISTS (SELECT FROM archive.connection c WHERE c.id = mail_party.connection));
ALTER TABLE archive.mail_attachment ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.mail_attachment FOR SELECT USING (EXISTS (SELECT FROM archive.connection c WHERE c.id = mail_attachment.connection));
ALTER TABLE archive.doc_page ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.doc_page FOR SELECT USING (EXISTS (SELECT FROM archive.connection c WHERE c.id = doc_page.connection));
ALTER TABLE archive.known_person ENABLE ROW LEVEL SECURITY;
CREATE POLICY seen ON archive.known_person FOR SELECT USING (EXISTS (SELECT FROM archive.mail_party p WHERE p.person = known_person.id));
