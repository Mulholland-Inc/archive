<script>
  import PostLayout from '#lib/PostLayout.svelte';
  import Art from '#lib/Art.svelte';
  import Compare from '#lib/Compare.svelte';
</script>

<PostLayout
  slug="archive-vs-notion"
  cta="Put your AI to work."
  lead="Free for your first workspace. Connect ChatGPT or Claude in a minute."
  related={['archive-vs-airtable', 'archive-vs-google-drive']}
>
  <p>Notion is one of the best-loved work tools of the last decade, and it earned that. It is polished, quick to learn and flexible enough that teams run everything in it, from meeting notes to product roadmaps. Pages and databases sit side by side, the templates are excellent, and it has its own AI built in.</p>
  <p>Archive also gives your team pages and lists. So why look at it at all? Because Archive is built for the moment your AI assistant stops being a chat window on the side and starts doing real work in your business.</p>

  <h2>Same idea, different foundation</h2>
  <p>In Archive, every list is made of real records in a real database. Each field knows what it holds, such as an amount of money, a date or an email address. Records can point to each other, and the rules about who can see and change what are enforced underneath, not only in the screens you click through.</p>
  <p>Picture a design agency of eight people that tracks clients, projects and invoices. In Archive those are three lists that know about each other: a project belongs to a client, and an invoice belongs to a project. Ask Claude which clients have unpaid invoices on projects that wrapped up last month, and it reads the actual records to answer. Ask it to draft the reminders and mark them as sent, and it does that too, in one go.</p>

  <h2>Your AI, working directly in your records</h2>
  <p>Archive works with the AI you already use: ChatGPT, Claude, Gemini and Grok. Add Archive once, and your assistant can read and change your lists, pages and files directly, as you. It sees exactly what you are allowed to see, and nothing more.</p>
  <p>Rather than pressing a long chain of small buttons, the assistant writes a short piece of code and Archive runs it right next to your data. One request can search, update fifty records and write a summary page. Changes are all-or-nothing: if something goes wrong halfway, nothing is saved, so you never end up with half an update to clean up.</p>
  <p>Each assistant gets its own access key. You can narrow what it may touch, and switch one off without affecting the others.</p>
  <p class="pull">The question is not whether your AI can see your workspace. It is whether it can do real work there, safely.</p>
  <p>Notion connects to outside tools and assistants too, and Notion AI is genuinely useful for writing and summarizing. The difference is in what sits underneath: in Archive, every record your assistant touches lives in a database that checks every change and keeps everything in step.</p>

  <h2>Pages people and AI write together</h2>
  <p>Archive pages are live documents. Your team types in the browser, an assistant edits the same page at the same moment, and every change merges. Nobody's typing gets overwritten. While the assistant works, it shows up in the list of who is on the page, just like a teammate.</p>
  <p>Say a property manager keeps a page per building, with a checklist for the next inspection, notes from tenants and open repairs. While she walks the building ticking boxes on her phone, Claude adds the repair requests that came in by email that morning to the same page. Neither of them waits for the other.</p>
  <figure>
    <Art name="blog-archive-vs-notion-2" alt="A laptop and an AI chat window both wired into the same page, with two editing markers on it" />
    <figcaption>One page, two editors: a person on a laptop and an AI assistant, both writing at once.</figcaption>
  </figure>
  <p>To be fair: Notion's editor is richer today, with embeds, layouts and years of polish. Archive pages are blocks of text, such as headings, lists, to-dos, quotes and code. Inline formatting and nested blocks are on the way.</p>

  <h2>Files that stay files</h2>
  <p>Most teams keep contracts, budgets and decks close to their notes. In Archive, Word, Excel and PowerPoint files stay real files that open anywhere, and your assistant can edit them in place: change a clause, fill in a cell, update the text on a slide. Everything it was not asked to touch, including styles, images and comments, stays exactly as it was.</p>

  <h2>A model that knows your business</h2>
  <p>Every time someone files an email, a receipt or a contract into the right list, that is a lesson. Archive can turn those lessons into a small model of your own: one that sorts new work into the right list, or one that reads a long document and fills in the record for you. It learns from your own records only, and your assistants can call it whenever they need it.</p>

  <h2>At a glance</h2>
  <Compare
    them="Notion"
    rows={[
      ['Pages', 'Live pages people and AI edit together, with every change merged', 'Polished, flexible pages with a rich editor'],
      ['Lists', 'Real records with typed fields and links between lists', 'Databases that live inside pages'],
      ['Ways to see a list', 'Lists, boards and calendars', 'Grids, boards, calendars, timelines, galleries and more'],
      ['AI', 'ChatGPT, Claude, Gemini and Grok, acting as you', 'Notion AI built in, plus integrations'],
      ['Big changes', 'A whole task runs in one go, and it all lands or none of it does', 'Edits through Notion AI and connected tools'],
      ['Office files', 'Edited in place, formatting left intact', 'Stored as attachments or embeds'],
      ['Who sees what', 'Enforced underneath, for people and AI alike', 'Workspace, teamspace and page sharing'],
      ['Your own models', 'Train small models on your own records', 'Built-in AI features'],
      ['Bringing data in', 'Connectors for mail, files, chat and more, Notion included', 'Imports and integrations']
    ]}
  />

  <h2>Which one fits</h2>
  <p>If your team mostly writes, with wikis, specs and meeting notes, and your lists are light, Notion is a wonderful home and its editor is hard to beat.</p>
  <p>If your lists are the heart of the business, such as clients, orders, properties or invoices, and you want ChatGPT or Claude to do real work in them rather than only answer questions, Archive was built for that. Every change is checked, every record stays consistent, and your AI works under the same rules as everyone else.</p>
  <p>It does not have to be either/or. Archive's Notion connector brings your pages and databases in, read-only, so you can try Archive with real content without moving anything.</p>
</PostLayout>
