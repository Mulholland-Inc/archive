<script>
  import PostLayout from '#lib/PostLayout.svelte';
  import Art from '#lib/Art.svelte';
</script>

<PostLayout
  slug="own-your-data"
  cta="Give your knowledge a home."
  lead="Free for your first workspace. Or keep reading about how it works."
  related={['company-memory-for-ai', 'messy-in-clean-out']}
>
    <p>We wrote one line at the top of our home page and kept coming back to it: <strong>Own your data infrastructure.</strong> Database. File storage. Agent access. A foundation for your digital world, built to run on your terms.</p>
    <p>This post is about why that line matters, and what it asks of a product that wants to deserve it.</p>

    <h2>What your company knows is scattered</h2>
    <p>Picture a twelve-person design agency. Client briefs arrive by email. Contracts live in a shared drive. The decision to move a launch date was made in a chat thread on a Tuesday afternoon. Invoices sit in the accounting tool, leads in the CRM, and the reasons behind half of it in someone's notebook.</p>
    <p>Each tool holds a piece, and each piece is useful on its own. But ask a simple question, like "what did we promise this client in March, and did we bill for it?", and the answer is spread across five places with five search boxes, five sets of permissions and five ideas of what a client even is.</p>
    <p>That was already a problem for people. It is a bigger one now that every team also works with an AI assistant. ChatGPT, Claude, Gemini and Grok are remarkably capable, and they know nothing about your business unless you hand it to them, one pasted paragraph at a time.</p>

    <p class="pull">The most valuable thing your company owns is what it knows. Too often, it is only renting a place to keep it.</p>

    <h2>One place, shaped like your business</h2>
    <p>Archive gives that knowledge a single home. It brings three things together that usually live apart:</p>
    <ul>
      <li><strong>Your records</strong>: your own lists, shaped the way your business works. Clients, projects, properties, invoices, whatever you track, each with the fields you need and links between them.</li>
      <li><strong>Your files</strong>: documents, spreadsheets, slides and PDFs, stored next to the records they belong to. Your AI can edit Word, Excel and PowerPoint files in place, and pages let people and AI write together live.</li>
      <li><strong>Agent access</strong>: a way for the AI you already use to search, read and file information in your workspace, under the same rules as everyone else.</li>
    </ul>
    <p>You do not have to give up the tools your team likes. Read-only connectors bring in copies of your mail, files, chats and CRM data and keep them current, without ever changing anything at the source. The agency keeps its inbox and its chat. It just stops losing what happens in them.</p>

    <h2>Every person, and every AI you choose</h2>
    <p>Having everything in one place only helps if the right people can use it, and only the right people. Think of a small property manager. Owners should see their own buildings and nothing else. The maintenance contractor needs work orders, not rent ledgers. Finance needs the ledgers, not tenant chats.</p>
    <p>In Archive, every record belongs to a space, and you decide who can see and change what in each one. Those rules hold everywhere: in the app, on a phone, and for every AI assistant you connect. An assistant working for the contractor sees exactly what the contractor sees.</p>
    <p>Which assistant is up to you. Archive works with ChatGPT, Claude, Gemini and Grok, and it does not lock you into one of them. If your team prefers a different assistant next year, the knowledge does not move. Only the assistant changes. And when an assistant makes changes, they are all-or-nothing, so nothing ends up half done.</p>

    <h2>Yours to take with you</h2>
    <p>Ownership is easy to claim and easy to test: can you leave?</p>
    <p>Archive keeps your records in a standard, widely used database rather than a format only we can read. Your files stay ordinary files. The way your workspace is organized can be written out as plain text and applied again somewhere else. Nothing about your knowledge depends on staying with us.</p>

    <figure>
      <Art name="blog-own-your-data-2" alt="A hand truck wheeling a strapped records drum and cabinet out through an open door, past a stack of moving boxes" />
      <figcaption>If you decide to move, everything comes along: records, files and the way they are organized.</figcaption>
    </figure>

    <p>We think that makes us a better product, not a riskier one for us. A tool you can leave is a tool that has to keep earning its place.</p>

    <h2>Control means decisions, not settings</h2>
    <p>"Control" often ends up meaning a toggle buried in someone else's admin screen. We mean something plainer. With Archive, the decisions that matter are yours:</p>
    <ul>
      <li><strong>What comes in.</strong> You choose which mailboxes, drives and tools to connect, and you can disconnect them.</li>
      <li><strong>How it is organized.</strong> You shape your records to fit your business, and change that shape whenever the business changes.</li>
      <li><strong>Who sees it.</strong> People and AI assistants get exactly the access you give them.</li>
      <li><strong>Which AI works with it.</strong> Bring the assistant your team already trusts, or several.</li>
      <li><strong>Where it goes.</strong> Take everything with you if you ever want to.</li>
    </ul>
    <p>Those are the terms. They are yours.</p>

    <figure>
      <Art name="security" alt="A secure records unit with locked compartments" />
      <figcaption>Every record belongs to a space, and only the right people, and the right AI, see it.</figcaption>
    </figure>

    <h2>A foundation, not another tool</h2>
    <p>Most software adds one more place to look. We are trying to build the opposite: the place underneath, where your company's knowledge collects and stays useful. Your team works in it directly. Your AI assistants work in it on your behalf. And over time, everything your team files becomes something more: the examples you can use to train a small model that sorts new work the way you would.</p>
    <p>The design agency does not need a new habit to get there. It connects its inbox and drive, lets its assistant file the first few hundred records into clients and projects, and invites the team. A week later, the question about March has one answer, in one place, and anyone allowed to ask it can.</p>
    <p>That is what owning your data infrastructure looks like in practice. Not another app to check. A foundation you control.</p>
</PostLayout>
