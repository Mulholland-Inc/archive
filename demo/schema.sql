CREATE TYPE account_kind AS ENUM ('checking', 'savings', 'credit', 'investment', 'retirement');
CREATE TABLE bank_account (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    institution text,
    kind account_kind,
    routing archive.us_routing,
    last4 text,
    balance archive.money,
    opened date,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, institution)) STORED
);
CREATE INDEX bank_account_search ON bank_account USING gin (search);
ALTER TABLE bank_account ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_account FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON bank_account USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE bank_account IS 'A bank, savings, credit or investment account
icon: landmark';
COMMENT ON COLUMN bank_account.name IS 'Your name for the account';
COMMENT ON COLUMN bank_account.institution IS 'Bank or broker';
COMMENT ON COLUMN bank_account.kind IS 'Type of account';
COMMENT ON COLUMN bank_account.routing IS 'ABA routing number of the bank';
COMMENT ON COLUMN bank_account.last4 IS 'Last four digits of the account number';
COMMENT ON COLUMN bank_account.balance IS 'Last known balance';
COMMENT ON COLUMN bank_account.opened IS 'Date opened';

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

CREATE TABLE journal (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    day date NOT NULL,
    mood smallint CHECK (mood BETWEEN 1 AND 5),
    entry text NOT NULL,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(entry)) STORED
);
CREATE INDEX journal_search ON journal USING gin (search);
ALTER TABLE journal ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON journal USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE journal IS 'A diary entry for a day
icon: notebook-pen';
COMMENT ON COLUMN journal.day IS 'The day';
COMMENT ON COLUMN journal.mood IS 'Mood, 1 (bad) to 5 (great)';
COMMENT ON COLUMN journal.entry IS 'What happened and how it felt';

CREATE TYPE measurement_kind AS ENUM ('weight', 'sleep', 'steps', 'resting_heart_rate', 'blood_pressure_systolic', 'blood_pressure_diastolic', 'run_distance');
CREATE TABLE measurement (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    taken date NOT NULL,
    kind measurement_kind NOT NULL,
    value double precision NOT NULL,
    unit text NOT NULL,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(notes)) STORED
);
CREATE INDEX measurement_search ON measurement USING gin (search);
ALTER TABLE measurement ENABLE ROW LEVEL SECURITY;
ALTER TABLE measurement FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON measurement USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE measurement IS 'A health or fitness measurement
icon: activity';
COMMENT ON COLUMN measurement.taken IS 'Day of the measurement';
COMMENT ON COLUMN measurement.kind IS 'What was measured';
COMMENT ON COLUMN measurement.value IS 'The measured value';
COMMENT ON COLUMN measurement.unit IS 'Unit, like lb, h, steps, bpm, mmHg or mi';
COMMENT ON COLUMN measurement.notes IS 'Context';

CREATE TABLE note (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    body archive.richtext,
    tags text[],
    pinned boolean NOT NULL DEFAULT false,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, body)) STORED
);
CREATE INDEX note_search ON note USING gin (search);
ALTER TABLE note ENABLE ROW LEVEL SECURITY;
ALTER TABLE note FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON note USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE note IS 'A note, idea or piece of reference
icon: sticky-note';
COMMENT ON COLUMN note.title IS 'Title';
COMMENT ON COLUMN note.body IS 'The note, in Markdown';
COMMENT ON COLUMN note.tags IS 'Tags';
COMMENT ON COLUMN note.pinned IS 'Pinned to the top';

CREATE TYPE person_relation AS ENUM ('partner', 'family', 'friend', 'colleague', 'neighbor', 'doctor', 'service', 'other');
CREATE TABLE person (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    relation person_relation,
    birthday date,
    email archive.email,
    phone archive.phone,
    address archive.address,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, notes)) STORED
);
CREATE INDEX person_search ON person USING gin (search);
ALTER TABLE person ENABLE ROW LEVEL SECURITY;
ALTER TABLE person FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON person USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE person IS 'Someone in your life: family, friends, colleagues, doctors
icon: user';
COMMENT ON COLUMN person.name IS 'Full name';
COMMENT ON COLUMN person.relation IS 'How you know them';
COMMENT ON COLUMN person.birthday IS 'Date of birth';
COMMENT ON COLUMN person.email IS 'Personal email';
COMMENT ON COLUMN person.phone IS 'Mobile number';
COMMENT ON COLUMN person.address IS 'Home address';
COMMENT ON COLUMN person.notes IS 'Things to remember about them';

CREATE TYPE place_kind AS ENUM ('home', 'restaurant', 'cafe', 'bar', 'shop', 'doctor', 'gym', 'park', 'museum', 'office', 'school', 'hotel', 'other');
CREATE TABLE place (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    kind place_kind,
    address archive.address,
    location archive.geo,
    website archive.url,
    rating smallint CHECK (rating BETWEEN 1 AND 5),
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, notes)) STORED
);
CREATE INDEX place_search ON place USING gin (search);
ALTER TABLE place ENABLE ROW LEVEL SECURITY;
ALTER TABLE place FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON place USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE place IS 'A place you go to or want to remember
icon: map-pin';
COMMENT ON COLUMN place.name IS 'Name of the place';
COMMENT ON COLUMN place.kind IS 'What sort of place';
COMMENT ON COLUMN place.address IS 'Street address';
COMMENT ON COLUMN place.location IS 'Coordinates';
COMMENT ON COLUMN place.website IS 'Website';
COMMENT ON COLUMN place.rating IS 'Your rating, 1 to 5';
COMMENT ON COLUMN place.notes IS 'What to order, who to ask for, opening quirks';

CREATE TABLE recipe (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    cuisine text,
    minutes integer,
    servings integer,
    ingredients text[],
    steps text,
    source archive.url,
    tags text[],
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, cuisine, steps)) STORED
);
CREATE INDEX recipe_search ON recipe USING gin (search);
ALTER TABLE recipe ENABLE ROW LEVEL SECURITY;
ALTER TABLE recipe FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON recipe USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE recipe IS 'A recipe you cook
icon: chef-hat';
COMMENT ON COLUMN recipe.name IS 'Dish';
COMMENT ON COLUMN recipe.cuisine IS 'Cuisine';
COMMENT ON COLUMN recipe.minutes IS 'Total time in minutes';
COMMENT ON COLUMN recipe.servings IS 'Servings';
COMMENT ON COLUMN recipe.ingredients IS 'Ingredients with amounts, one per item';
COMMENT ON COLUMN recipe.steps IS 'Method';
COMMENT ON COLUMN recipe.source IS 'Where it came from';
COMMENT ON COLUMN recipe.tags IS 'Tags like vegetarian or quick';

CREATE TABLE trip (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    destination text,
    country archive.country,
    starts date,
    ends date,
    budget archive.money,
    people uuid[],
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, destination, notes)) STORED
);
CREATE INDEX trip_search ON trip USING gin (search);
ALTER TABLE trip ENABLE ROW LEVEL SECURITY;
ALTER TABLE trip FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON trip USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE trip IS 'A trip or vacation, past or planned
icon: plane';
COMMENT ON COLUMN trip.name IS 'Name of the trip';
COMMENT ON COLUMN trip.destination IS 'City or region';
COMMENT ON COLUMN trip.country IS 'Country';
COMMENT ON COLUMN trip.starts IS 'First day';
COMMENT ON COLUMN trip.ends IS 'Last day';
COMMENT ON COLUMN trip.budget IS 'Budget';
COMMENT ON COLUMN trip.people IS 'references person';
COMMENT ON COLUMN trip.notes IS 'Plans, bookings, what to pack';

CREATE TYPE insurance_kind AS ENUM ('health', 'home', 'contents', 'liability', 'travel', 'car', 'life', 'legal');
CREATE TABLE insurance (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    provider text,
    kind insurance_kind,
    policy_number text,
    premium archive.money,
    deductible archive.money,
    renews date,
    policy uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, provider, policy_number)) STORED
);
CREATE INDEX insurance_search ON insurance USING gin (search);
ALTER TABLE insurance ENABLE ROW LEVEL SECURITY;
ALTER TABLE insurance FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON insurance USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE insurance IS 'An insurance policy
icon: shield-check';
COMMENT ON COLUMN insurance.name IS 'Your name for the policy';
COMMENT ON COLUMN insurance.provider IS 'Insurer';
COMMENT ON COLUMN insurance.kind IS 'What it covers';
COMMENT ON COLUMN insurance.policy_number IS 'Policy number';
COMMENT ON COLUMN insurance.premium IS 'Monthly premium';
COMMENT ON COLUMN insurance.deductible IS 'Own risk per year or claim';
COMMENT ON COLUMN insurance.renews IS 'Renewal or cancellation date';
COMMENT ON COLUMN insurance.policy IS 'Policy document';

CREATE TABLE possession (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    brand text,
    category text,
    purchased date,
    price archive.money,
    warranty_until date,
    serial text,
    receipt uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, brand, category, serial)) STORED
);
CREATE INDEX possession_search ON possession USING gin (search);
ALTER TABLE possession ENABLE ROW LEVEL SECURITY;
ALTER TABLE possession FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON possession USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE possession IS 'Something you own that is worth keeping track of
icon: package';
COMMENT ON COLUMN possession.name IS 'What it is';
COMMENT ON COLUMN possession.brand IS 'Brand and model';
COMMENT ON COLUMN possession.category IS 'Like electronics, furniture or bike';
COMMENT ON COLUMN possession.purchased IS 'Purchase date';
COMMENT ON COLUMN possession.price IS 'What you paid';
COMMENT ON COLUMN possession.warranty_until IS 'End of the warranty';
COMMENT ON COLUMN possession.serial IS 'Serial number';
COMMENT ON COLUMN possession.receipt IS 'Receipt or invoice';

CREATE TABLE event (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    starts timestamptz NOT NULL,
    ends timestamptz,
    place uuid REFERENCES place,
    people uuid[],
    recurrence archive.recurrence,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, notes)) STORED
);
CREATE INDEX event_search ON event USING gin (search);
ALTER TABLE event ENABLE ROW LEVEL SECURITY;
ALTER TABLE event FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON event USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE event IS 'Something on your calendar
icon: calendar-days';
COMMENT ON COLUMN event.title IS 'What it is';
COMMENT ON COLUMN event.starts IS 'Start time';
COMMENT ON COLUMN event.ends IS 'End time';
COMMENT ON COLUMN event.place IS 'Where';
COMMENT ON COLUMN event.people IS 'references person';
COMMENT ON COLUMN event.recurrence IS 'Repeat rule, if it repeats';
COMMENT ON COLUMN event.notes IS 'Agenda or things to bring';

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

CREATE TYPE document_kind AS ENUM ('passport', 'id_card', 'drivers_license', 'residence_permit', 'health_card');
CREATE TABLE identity_document (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
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
CREATE POLICY access ON identity_document USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE identity_document IS 'A passport, ID card, driver''s license or permit
icon: id-card';
COMMENT ON COLUMN identity_document.kind IS 'Type of document';
COMMENT ON COLUMN identity_document.number IS 'Document number';
COMMENT ON COLUMN identity_document.holder IS 'Whose document it is';
COMMENT ON COLUMN identity_document.country IS 'Issuing country';
COMMENT ON COLUMN identity_document.issued IS 'Issue date';
COMMENT ON COLUMN identity_document.expires IS 'Expiry date';
COMMENT ON COLUMN identity_document.scan IS 'Scan of the document';

CREATE TYPE payment_category AS ENUM ('income', 'rent', 'groceries', 'eating_out', 'transport', 'utilities', 'insurance', 'subscriptions', 'shopping', 'health', 'travel', 'gifts', 'savings', 'taxes', 'other');
CREATE TABLE payment (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    booked date NOT NULL,
    amount archive.money NOT NULL,
    counterparty text,
    description text,
    category payment_category,
    account uuid REFERENCES bank_account,
    receipt uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(counterparty, description)) STORED
);
CREATE INDEX payment_search ON payment USING gin (search);
ALTER TABLE payment ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON payment USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE payment IS 'Money in or out of an account; negative amounts are spending
icon: arrow-left-right';
COMMENT ON COLUMN payment.booked IS 'Booking date';
COMMENT ON COLUMN payment.amount IS 'Amount, negative when you paid';
COMMENT ON COLUMN payment.counterparty IS 'Who paid you or whom you paid';
COMMENT ON COLUMN payment.description IS 'Description from the bank';
COMMENT ON COLUMN payment.category IS 'Spending category';
COMMENT ON COLUMN payment.account IS 'Account it was booked on';
COMMENT ON COLUMN payment.receipt IS 'Receipt or invoice file';

CREATE TABLE payment_card (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    number archive.card,
    expires text,
    account uuid REFERENCES bank_account,
    holder uuid REFERENCES person,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED
);
CREATE INDEX payment_card_search ON payment_card USING gin (search);
ALTER TABLE payment_card ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_card FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON payment_card USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE payment_card IS 'A debit or credit card
icon: credit-card';
COMMENT ON COLUMN payment_card.name IS 'Your name for the card';
COMMENT ON COLUMN payment_card.number IS 'Card number';
COMMENT ON COLUMN payment_card.expires IS 'Expiry, MM/YY';
COMMENT ON COLUMN payment_card.account IS 'Account the card draws on';
COMMENT ON COLUMN payment_card.holder IS 'Card holder';

CREATE TABLE subscription (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    price archive.money,
    billing archive.recurrence,
    renews date,
    website archive.url,
    account uuid REFERENCES bank_account,
    active boolean NOT NULL DEFAULT true,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED
);
CREATE INDEX subscription_search ON subscription USING gin (search);
ALTER TABLE subscription ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON subscription USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE subscription IS 'Something you pay for on a schedule
icon: repeat';
COMMENT ON COLUMN subscription.name IS 'Service name';
COMMENT ON COLUMN subscription.price IS 'Price per billing period';
COMMENT ON COLUMN subscription.billing IS 'Billing schedule as an RRULE';
COMMENT ON COLUMN subscription.renews IS 'Next renewal date';
COMMENT ON COLUMN subscription.website IS 'Where to manage or cancel it';
COMMENT ON COLUMN subscription.account IS 'Account it is paid from';
COMMENT ON COLUMN subscription.active IS 'Still subscribed';

CREATE TYPE task_status AS ENUM ('todo', 'doing', 'waiting', 'done');
CREATE TYPE task_priority AS ENUM ('low', 'normal', 'high', 'urgent');
CREATE TYPE life_area AS ENUM ('home', 'health', 'money', 'admin', 'work', 'social', 'learning', 'travel');
CREATE TABLE task (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    status task_status NOT NULL DEFAULT 'todo',
    priority task_priority,
    area life_area,
    due date,
    person uuid REFERENCES person,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, notes)) STORED
);
CREATE INDEX task_search ON task USING gin (search);
ALTER TABLE task ENABLE ROW LEVEL SECURITY;
ALTER TABLE task FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON task USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE task IS 'Something you need to do
icon: square-check';
COMMENT ON COLUMN task.title IS 'What to do';
COMMENT ON COLUMN task.status IS 'Progress';
COMMENT ON COLUMN task.priority IS 'How important';
COMMENT ON COLUMN task.area IS 'Part of life it belongs to';
COMMENT ON COLUMN task.due IS 'Due date';
COMMENT ON COLUMN task.person IS 'Person it is about or waiting on';
COMMENT ON COLUMN task.notes IS 'Details';

CREATE VIEW doctors WITH (security_invoker = true) AS SELECT * FROM person WHERE relation = 'doctor';
COMMENT ON VIEW doctors IS 'Your doctors, dentists and other carers
icon: stethoscope';

CREATE VIEW family WITH (security_invoker = true) AS SELECT * FROM person WHERE relation = 'family';
COMMENT ON VIEW family IS 'Your family
icon: users';

CREATE VIEW renewals WITH (security_invoker = true) AS SELECT * FROM subscription;
COMMENT ON VIEW renewals IS 'Subscriptions by when they renew
layout: calendar
by: renews
icon: calendar-sync';

CREATE VIEW task_board WITH (security_invoker = true) AS SELECT * FROM task;
COMMENT ON VIEW task_board IS 'Tasks by progress
layout: board
by: status
icon: kanban';

CREATE VIEW rent WITH (security_invoker = true) AS SELECT * FROM payment WHERE category = 'rent';
COMMENT ON VIEW rent IS 'Rent you paid
icon: house';

CREATE VIEW trips WITH (security_invoker = true) AS SELECT * FROM trip;
COMMENT ON VIEW trips IS 'Trips by when they start
layout: calendar
by: starts
icon: luggage';

CREATE VIEW things_by_category WITH (security_invoker = true) AS SELECT * FROM possession;
COMMENT ON VIEW things_by_category IS 'Your things, grouped by category
layout: board
by: category
icon: tags';

