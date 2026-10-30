export const POSTS = [
  {
    slug: "archive-vs-notion",
    title: "Archive vs Notion",
    short: "Archive vs Notion",
    lead: "Both keep your team's lists and documents. The difference shows up when your AI starts doing the work.",
    summary: "Both keep your team's lists and documents. The difference shows up when your AI starts doing the work.",
    description: "Notion and Archive both keep your team's lists and documents. Here is what changes when your AI works directly in your records.",
    alt: "A tidy paper notebook on a desk beside a platform where a document, a records cabinet and an AI chat window are wired together",
    size: [2563, 1039]
  },
  {
    slug: "company-memory-for-ai",
    title: "Give your AI your company's memory",
    short: "Give your AI your company's memory",
    lead: "ChatGPT, Claude, Gemini and Grok are brilliant, and they know nothing about your business. Connect Archive once and they answer from your own records.",
    summary: "ChatGPT and Claude are brilliant and know nothing about your business. Here is how to fix that.",
    description: "ChatGPT, Claude, Gemini and Grok know nothing about your business. Connect Archive once and they answer from your own records, within what you are allowed to see.",
    alt: "Four chat assistants plugged into one large card catalog, with one drawer pulled open and a few drawers locked",
    size: [2256, 1256]
  },
  {
    slug: "own-your-data",
    title: "Own your data infrastructure",
    short: "Own your data infrastructure",
    lead: "Your company's knowledge is its most valuable asset. It should live somewhere you control.",
    summary: "Your company's knowledge is its most valuable asset. It should live somewhere you control.",
    description: "Your company's knowledge is its most valuable asset. It should live in one place you control, that every person and every AI you choose can use, and that you can take with you.",
    alt: "A sturdy base with a carry handle holding a records drum, a filing cabinet with an open drawer and an antenna plugged into three small assistant robots, with a key turned in its lock",
    size: [1759, 1317]
  },
  {
    slug: "archive-vs-airtable",
    title: "Archive vs Airtable",
    short: "Archive vs Airtable",
    lead: "Flexible lists are great until you outgrow them. What changes when there is a real database underneath.",
    summary: "Flexible lists are great until you outgrow them. What changes when there is a real database underneath.",
    description: "Airtable made databases friendly. Here is what changes with a real database underneath, plus files and your AI on top.",
    alt: "A flat spreadsheet board on legs with a few loose tiles beside a pair of sturdy record cabinets, where a robot arm files a card",
    size: [2090, 1284]
  },
  {
    slug: "one-tool-for-agents",
    title: "Why your AI gets one tool, not a hundred",
    short: "Why your AI gets one tool, not a hundred",
    lead: "Fewer buttons, better answers. Instead of a wall of narrow tools, your assistant writes a short script that does the whole job in one go.",
    summary: "Fewer buttons, better answers: how letting your assistant write a little code changes what it can do.",
    description: "Instead of dozens of narrow buttons, your assistant writes a short script that looks things up, loops and makes changes in one all-or-nothing go.",
    alt: "A pegboard crowded with small single-purpose gadgets behind one robot arm that follows a recipe on a clipboard and sorts cards into trays",
    size: [2089, 1473]
  },
  {
    slug: "messy-in-clean-out",
    title: "From messy inbox to clean records",
    short: "From messy inbox to clean records",
    lead: "Mail, files and chats arrive the way they are. A tour of how they become something you can use.",
    summary: "Mail, files and chats arrive the way they are. A tour of how they become something you can use.",
    description: "Mail, files, chats and CRM data arrive the way they are. Your AI reads them and files clean records, so the same phone number or amount is always written the same way. A walk through one day at a small company.",
    alt: "A mailbox, a chat sign and a bin of loose envelopes and notes feed a belt into a sorting machine, which sends out identical record cards to a filing cabinet",
    size: [1869, 1373]
  },
  {
    slug: "archive-vs-google-drive",
    title: "Archive vs Google Drive and SharePoint",
    short: "Archive vs Google Drive",
    lead: "Folders hold files. Archive also knows what is in them, and lets your AI edit them in place.",
    summary: "Folders hold files. Archive also knows what is in them, and lets your AI edit them in place.",
    description: "Folders hold files. Archive also knows what is in them, searches inside, and lets your AI edit Word, Excel and PowerPoint in place.",
    alt: "A crate of plain folders wired to a platform where a document, a spreadsheet and a slide stand open under a magnifying glass, beside a filing cabinet",
    size: [1923, 921]
  },
  {
    slug: "train-your-own-model",
    title: "Train a model that knows your business",
    short: "Train a model that knows your business",
    lead: "Everything your team files is a lesson. Turn it into a small model that sorts new work the way you would.",
    summary: "Everything your team files is a lesson. Turn it into a small model that sorts new work the way you would.",
    description: "Everything your team files is a lesson. Turn it into a small model that sorts incoming documents and fills in records the way you would.",
    alt: "Stacks of filed cards beside a training machine that has produced a blue chip, which powers a sorter dropping incoming pages into three bins",
    size: [2197, 1298]
  },
  {
    slug: "change-the-shape-anytime",
    title: "Change how your data is organized, anytime",
    short: "Change how your data is organized",
    lead: "Businesses change shape. Your workspace should bend with them without losing a thing.",
    summary: "Businesses change shape. Your workspace should bend with them without losing a thing.",
    description: "Businesses change shape. Add a field, rename something or add a new kind of record, see exactly what will change before it happens, and lose nothing along the way.",
    alt: "A crane lowers a new blue module into the one empty slot of a grid of record blocks, cabled to a stand of board, calendar and list panels, with a toolbox and wrench nearby",
    size: [1643, 1399]
  }
];

export const post = (slug) => POSTS.find((p) => p.slug === slug);
