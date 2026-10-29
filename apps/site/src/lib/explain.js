export const EXPLAIN = {
  title: "Archive, explained like you're 12",
  nudge: 'New to all this?'
};

export const PAGES = [
  {
    art: 'hidden',
    alt: 'Three app screens in front of a locked wall, with a database peeking over the top',
    title: 'Every app is secretly a database.',
    text: 'Notion, Salesforce, NetSuite, QuickBooks, Google Sheets, even Claude. Behind the screen, every one of them keeps your stuff in a database.'
  },
  {
    art: 'tables',
    alt: 'A board of neat rows and columns, some of them highlighted',
    title: 'And a database is just tables.',
    text: 'Rows and columns, like a spreadsheet. Customers in one table, orders in another, and a line from each order to its customer. That’s the “relational” part. That’s it.'
  },
  {
    art: 'endpoint',
    alt: 'A big machine with a single thin hose running out to a small plug',
    title: 'But you only get a straw.',
    text: 'You can’t touch those tables yourself. Your AI has to sip through the app’s API or MCP, one small question at a time. It’s slow, and it only sees what fits through the straw.'
  },
  {
    art: 'security',
    alt: 'A locked vault with a keypad',
    title: 'And it only bends so far.',
    text: 'Want a new kind of record, a new rule, or ten times more data? You get what the app’s makers built, at the size they allow.'
  },
  {
    art: 'ontology',
    alt: 'Records for people, places and documents joined by lines on a board',
    title: 'Palantir calls the right way an ontology.',
    text: 'Typed tables that describe how your business really works: what a customer is, what an order is, and how they connect. It’s how some of the biggest companies in the world run.'
  },
  {
    art: 'blog-own-your-data',
    alt: 'A sturdy base holding a records drum and a filing cabinet, with a key turned in its lock',
    title: 'Archive gives you your own.',
    text: 'A real Postgres database, typed and shaped like your business, living where you choose. Not a copy. Not a straw. The real tables.'
  },
  {
    art: 'code',
    alt: 'A screen of code beside a big play button',
    title: 'Your AI writes code, right on the data.',
    text: 'Coding agents like Claude Code don’t click around. They write code that runs next to your tables: change a hundred thousand records, add a new type, build a new view. In seconds.'
  },
  {
    art: 'delta',
    alt: 'A conveyor moving sheets along in a straight line',
    title: 'That’s why it’s 10x faster.',
    text: 'No straw. No waiting for a feature. No limits someone else set. Your AI works on the real thing, directly.'
  },
  {
    art: 'story-5',
    alt: 'A factory with new modules plugged in around it',
    title: 'Five things, in one place.',
    list: [
      ['Direct data', 'the real tables, not a copy'],
      ['A database', 'Postgres, built to grow'],
      ['Code', 'your AI can change anything'],
      ['Sovereignty', 'you own it and can take it anywhere'],
      ['Agents', 'every assistant works from the same source']
    ]
  }
];
