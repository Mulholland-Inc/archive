<script>
  import PostLayout from '#lib/PostLayout.svelte';
  import Art from '#lib/Art.svelte';
  import Compare from '#lib/Compare.svelte';
</script>

<PostLayout
  slug="archive-vs-airtable"
  cta="Give your lists a real foundation."
  lead="Free for your first workspace. No card, no sales call."
  related={['archive-vs-notion', 'archive-vs-google-drive']}
>
  <p>Airtable deserves a lot of credit. It showed millions of people that a database could feel as friendly as a spreadsheet. Grids, boards, calendars, forms, automations, a huge library of templates and, more recently, AI features of its own. For many teams it was the first time their work felt truly organized.</p>
  <p>Archive starts from the same wish: flexible lists, shaped the way your business works. The difference is what sits underneath, and what you can build on top of it.</p>

  <h2>When the questions change</h2>
  <p>A base usually starts small. Then it grows, more people join, and the questions change. Why does this phone number look different from that one? Can a contractor see the projects but not the rates? Can an AI update forty records without leaving a mess if it stops halfway? Who decides what the assistant is allowed to touch?</p>
  <p>These are database questions. Archive answers them with a real database underneath every list.</p>

  <h2>A real database underneath</h2>
  <p>Take the finance team at a company of forty people. They track vendors, purchase orders and invoices. In Archive each of those is a list of real records, and the lists know about each other: an invoice belongs to a purchase order, and a purchase order belongs to a vendor.</p>
  <p>Fields know what they hold. An amount is stored as an amount with its currency, so "$1,234.50" typed by hand becomes a proper number in dollars, not a scrap of text. A phone number typed three different ways is stored one way. Email addresses are tidied as they arrive. The cleanup happens on the way in, whichever person, file or assistant the information came from.</p>
  <p>You can change how your data is organized anytime. Add a field, rename one and keep everything in it, or link two lists together. Archive shows you what a change will do before it happens, and it will not quietly throw away information: removing something that still holds data has to be confirmed.</p>

  <h2>It keeps up as you grow</h2>
  <p>Archive runs on the same kind of database that larger businesses rely on, not on a layer that imitates one. Many people and many assistants can work in the same lists at once. When two of them change the same file, they take turns, and no edit is lost.</p>

  <h2>The same rules everywhere</h2>
  <p>In Archive, who can see and change what is decided by the database itself. Your team in the browser, ChatGPT acting on your behalf, a script your developer wrote, a connected tool: all of them get exactly the access they were given, and nothing more. A mistake in one place cannot expose someone else's records.</p>
  <p>Work is grouped into spaces. Everyone gets a private space, and you can make shared ones for a team or a client. The contractor sees the project space and nothing else.</p>
  <figure>
    <Art name="blog-archive-vs-airtable-2" alt="A laptop, a tablet and an AI chat window each wired through the same small gate into one records cabinet" />
    <figcaption>Every way in, whether a laptop, a phone or an AI assistant, goes through the same gate and follows the same rules.</figcaption>
  </figure>
  <p class="pull">When the rules live in the database, it does not matter who is asking. A person, a script or an AI gets exactly what it is allowed, and nothing more.</p>

  <h2>Changes that land all at once, or not at all</h2>
  <p>Airtable's automations are great at "when this happens, do that". Archive takes a different route for bigger jobs: your AI assistant writes a short piece of code for the whole task, and Archive runs it in one go.</p>
  <p>Say the finance team asks Claude to match sixty invoices against their purchase orders, mark the ones that agree and flag the ones that do not. If something goes wrong at invoice forty-one, nothing is saved. You never end up with forty matched invoices, twenty untouched ones and no way to tell which is which. Fix the problem, run it again, and it all lands together.</p>

  <h2>Pages, files and AI on top</h2>
  <p>Lists are only part of the work, so Archive keeps the rest in the same place:</p>
  <ul>
    <li><strong>Pages</strong> your team and your AI write together, live, with every change merged.</li>
    <li><strong>Files</strong> that stay real Word, Excel and PowerPoint files, which your assistant can edit in place without disturbing the formatting.</li>
    <li><strong>Your AI assistants</strong>, whether ChatGPT, Claude, Gemini or Grok, working directly in your records, as you.</li>
    <li><strong>Your own models</strong>, trained on the records you already filed, to sort new invoices or fill in records from long documents the way your team would.</li>
  </ul>

  <h2>At a glance</h2>
  <Compare
    them="Airtable"
    rows={[
      ['Feels like', 'Lists, boards and calendars over a real database', 'A friendly, spreadsheet-like database'],
      ['Fields', 'Typed fields that clean up messy input, like money, phone and email', 'Many field types, with formulas and lookups'],
      ['Changing structure', 'Reshape anytime; renames keep their data and nothing is dropped without asking', 'Add and edit fields right in the grid'],
      ['Who sees what', 'Enforced underneath, for people and AI alike', 'Workspace and base sharing, plus interfaces'],
      ['Big jobs', 'Your AI runs the whole task in one go, all or nothing', 'Built-in automations with triggers and actions'],
      ['AI', 'ChatGPT, Claude, Gemini and Grok, acting as you', "Airtable's own AI features"],
      ['Documents', 'Live pages and Office files edited in place', 'Long text fields and attachments'],
      ['Bringing data in', 'Connectors for mail, files, chat, CRM and finance', 'Imports, syncs and integrations']
    ]}
  />

  <h2>Which one fits</h2>
  <p>If you need a quick tracker, your team loves the grid and your automations are simple, Airtable is a great choice and a pleasure to use.</p>
  <p>If your lists have become the backbone of the business, you need every record to stay consistent, and you want your AI to do real work in them under the same rules as everyone else, that is what Archive is for. And because pages and files live alongside your records, you have fewer tools to keep in sync.</p>
</PostLayout>
