export const EXPLAIN = {
  title: 'Archive for Dummies',
  subtitle: "explained like you're 12",
  nudge: 'New to all this?'
};

export const PAGES = [
  {
    art: 'explain-1',
    alt: 'An app screen slid aside on its rail, showing the database that sat behind it',
    title: 'Every app is secretly a database.',
    text: 'Notion, Salesforce, NetSuite, QuickBooks, Google Sheets, even Claude. Behind the screen, every one of them keeps your stuff in a database.'
  },
  {
    art: 'explain-2',
    alt: 'Two tables of rows and columns, with a line arching from a row in one to its matching row in the other',
    title: 'And a database is just tables.',
    text: 'Rows and columns, like a spreadsheet. Customers in one table, orders in another, and a line from each order to its customer. That’s the “relational” part. That’s it.'
  },
  {
    art: 'explain-3',
    alt: 'A big open tank full of data, and a small robot sipping from it through one thin straw',
    title: 'But you only get a straw.',
    text: 'You can’t touch those tables yourself. Your AI has to sip through the app’s API or MCP, one small question at a time. It’s slow, and it only sees what fits through the straw.'
  },
  {
    art: 'explain-4',
    alt: 'Crates stacked inside a bolted frame, pressed up against its fixed lid, with more crates left outside',
    title: 'And it only bends so far.',
    text: 'Want a new kind of record, a new rule, or ten times more data? You get what the app’s makers built, at the size they allow.'
  },
  {
    art: 'explain-5',
    alt: 'A map of typed records on a board: a customer, an order, a product and an invoice, joined by links',
    title: 'Palantir calls the right way an [ontology](/ontology).',
    text: 'Typed tables that describe how your business really works: what a customer is, what an order is, and how they connect. It’s how some of the biggest companies in the world run.'
  },
  {
    art: 'explain-6',
    alt: 'A database on its own fenced plot, with a flag planted on top and a key lying by the gate',
    title: 'Archive gives you your own.',
    text: 'A real Postgres database, typed and shaped like your business, [living where you choose](/security). Not a copy. Not a straw. The real tables.'
  },
  {
    art: 'explain-7',
    alt: 'A screen of code standing right on a table of records, with some records changed and one lifted mid-edit',
    title: 'Your AI writes code, right on the data.',
    text: 'Coding agents like Claude Code don’t click around. They [write code](/developers) that runs next to your tables: change a hundred thousand records, add a new type, build a new view. In seconds.'
  },
  {
    art: 'explain-8',
    alt: 'A wide open chute pouring data from a tank straight into a robot',
    title: 'That’s why it’s 10x faster.',
    text: 'No straw. No waiting for a feature. No limits someone else set. Your AI works on the real thing, directly.'
  },
  {
    art: 'explain-9',
    alt: 'One round base holding five pieces: a table, a database, a code screen, a key and a robot',
    title: 'Five things, in one place.',
    list: [
      ['Direct data', 'The real tables, not a copy'],
      ['A database', 'Postgres, built to grow'],
      ['Code', 'Your AI can change anything'],
      ['[Sovereignty](/security)', 'You own it and can take it anywhere'],
      ['[Agents](/agents)', 'Every assistant works from the same source']
    ]
  }
];
