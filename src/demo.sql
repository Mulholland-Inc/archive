CREATE TYPE company_industry AS ENUM ('software', 'finance', 'healthcare', 'retail', 'manufacturing', 'logistics', 'energy', 'media', 'education', 'hospitality');
CREATE TYPE company_relationship AS ENUM ('customer', 'prospect', 'partner', 'vendor', 'churned');
CREATE TABLE company (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    industry company_industry,
    relationship company_relationship,
    website archive.url,
    domain archive.domain,
    employees integer,
    vat_id archive.vat_id,
    address archive.address,
    annual_revenue archive.money,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED
);
CREATE INDEX company_search ON company USING gin (search);
CREATE INDEX company_name ON company (name);
ALTER TABLE company ENABLE ROW LEVEL SECURITY;
ALTER TABLE company FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON company FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON company USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE company IS 'A company you sell to, buy from or work with';

CREATE TABLE contact (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    email archive.email,
    phone archive.phone,
    title text,
    company uuid REFERENCES company,
    linkedin archive.url,
    last_contacted date,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, email, title)) STORED
);
CREATE INDEX contact_search ON contact USING gin (search);
CREATE INDEX contact_name ON contact (name);
ALTER TABLE contact ENABLE ROW LEVEL SECURITY;
ALTER TABLE contact FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON contact FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON contact USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE contact IS 'A person at a company';

CREATE TYPE employee_department AS ENUM ('engineering', 'sales', 'marketing', 'finance', 'operations', 'people', 'design', 'support');
CREATE TABLE employee (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    email archive.email,
    phone archive.phone,
    title text,
    department employee_department,
    start_date date,
    location text,
    salary archive.money,
    iban archive.iban,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, email, title, location)) STORED
);
CREATE INDEX employee_search ON employee USING gin (search);
CREATE INDEX employee_name ON employee (name);
ALTER TABLE employee ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON employee FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON employee USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE employee IS 'Someone who works here';

CREATE TYPE deal_stage AS ENUM ('lead', 'qualified', 'proposal', 'negotiation', 'won', 'lost');
CREATE TABLE deal (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    company uuid REFERENCES company,
    contact uuid REFERENCES contact,
    stage deal_stage,
    value archive.money,
    probability archive.percentage,
    expected_close date,
    owner uuid REFERENCES employee,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED
);
CREATE INDEX deal_search ON deal USING gin (search);
ALTER TABLE deal ENABLE ROW LEVEL SECURITY;
ALTER TABLE deal FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON deal FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON deal USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE deal IS 'A sales opportunity';

CREATE TYPE project_status AS ENUM ('planned', 'active', 'on_hold', 'completed', 'cancelled');
CREATE TABLE project (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    client uuid REFERENCES company,
    status project_status,
    budget archive.money,
    start date,
    due date,
    lead uuid REFERENCES employee,
    summary archive.richtext,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, summary)) STORED
);
CREATE INDEX project_search ON project USING gin (search);
ALTER TABLE project ENABLE ROW LEVEL SECURITY;
ALTER TABLE project FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON project FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON project USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE project IS 'Work for a client';

CREATE TYPE invoice_status AS ENUM ('draft', 'sent', 'paid', 'overdue', 'void');
CREATE TABLE invoice (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    number text NOT NULL,
    company uuid REFERENCES company,
    project uuid REFERENCES project,
    total archive.money,
    vat_rate archive.percentage,
    issued date,
    due date,
    status invoice_status,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(number)) STORED
);
CREATE INDEX invoice_search ON invoice USING gin (search);
CREATE INDEX invoice_number ON invoice (number);
ALTER TABLE invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON invoice FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON invoice USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE invoice IS 'An invoice sent to a client';

CREATE TYPE expense_category AS ENUM ('software', 'travel', 'office', 'hardware', 'marketing', 'contractors', 'meals', 'training');
CREATE TABLE expense (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    description text NOT NULL,
    vendor uuid REFERENCES company,
    category expense_category,
    amount archive.money,
    date date,
    reimbursable boolean,
    employee uuid REFERENCES employee,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(description)) STORED
);
CREATE INDEX expense_search ON expense USING gin (search);
ALTER TABLE expense ENABLE ROW LEVEL SECURITY;
ALTER TABLE expense FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON expense FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON expense USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE expense IS 'Money spent';

CREATE TYPE contract_kind AS ENUM ('msa', 'sow', 'nda', 'subscription', 'lease', 'dpa');
CREATE TYPE contract_status AS ENUM ('draft', 'in_review', 'signed', 'expired', 'terminated');
CREATE TABLE contract (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    company uuid REFERENCES company,
    kind contract_kind,
    value archive.money,
    start date,
    "end" date,
    auto_renew boolean,
    status contract_status,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title)) STORED
);
CREATE INDEX contract_search ON contract USING gin (search);
ALTER TABLE contract ENABLE ROW LEVEL SECURITY;
ALTER TABLE contract FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON contract FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON contract USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE contract IS 'An agreement with a company';

CREATE TABLE meeting (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    at archive.timestamp,
    duration archive.duration,
    company uuid REFERENCES company,
    attendees uuid[],
    notes archive.richtext,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, notes)) STORED
);
CREATE INDEX meeting_search ON meeting USING gin (search);
COMMENT ON COLUMN meeting.attendees IS 'references contact';
ALTER TABLE meeting ENABLE ROW LEVEL SECURITY;
ALTER TABLE meeting FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON meeting FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON meeting USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE meeting IS 'A meeting with a company';

CREATE TYPE task_status AS ENUM ('todo', 'in_progress', 'blocked', 'done');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high', 'urgent');
CREATE TABLE task (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    project uuid REFERENCES project,
    assignee uuid REFERENCES employee,
    status task_status,
    priority task_priority,
    due date,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title)) STORED
);
CREATE INDEX task_search ON task USING gin (search);
ALTER TABLE task ENABLE ROW LEVEL SECURITY;
ALTER TABLE task FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON task FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON task USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE task IS 'Something to do in a project';

CREATE TYPE product_billing AS ENUM ('one_off', 'monthly', 'yearly');
CREATE TABLE product (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    sku text,
    price archive.money,
    billing product_billing,
    active boolean,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, sku)) STORED
);
CREATE INDEX product_search ON product USING gin (search);
CREATE INDEX product_sku ON product (sku);
ALTER TABLE product ENABLE ROW LEVEL SECURITY;
ALTER TABLE product FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON product FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON product USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE product IS 'Something we sell';
