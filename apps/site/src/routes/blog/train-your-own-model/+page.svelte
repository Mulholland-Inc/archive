<script>
  import PostLayout from '#lib/PostLayout.svelte';
  import Figure from '#lib/Figure.svelte';
  import Pull from '#lib/Pull.svelte';
</script>

<PostLayout
  slug="train-your-own-model"
  cta="Teach a model your way of working."
  lead="Start filing today. Your first set of examples is already taking shape."
  related={['change-the-shape-anytime']}
>
    <p>Every business has a way of sorting things that nobody wrote down. The office manager knows a "statement" from that one supplier is really an invoice. The leasing team knows which emails are maintenance requests and which are just tenants saying thanks. The finance team knows which receipts belong to which project at a glance.</p>
    <p>That knowledge lives in people's heads, and it shows up every time they file something. Archive lets you capture it and put it to work, in a small model trained on your own records.</p>

    <h2>Filing is teaching</h2>
    <p>Think about what happens when someone on your team files a document in Archive. A PDF arrives, they decide it is a contract, and they fill in the contract's details: the client, the start date, the value, the renewal terms. That single act holds two lessons. The first is "this kind of document is a contract." The second is "this is how a contract's text turns into a filled-in record."</p>
    <p>Do that a few hundred times, and you have a collection of worked examples, written by the people who know your business best. Nobody had to sit down and label anything. The examples came from doing the job.</p>
    <p>The same is true when your AI assistant does the filing and a person checks or corrects it. Every correction is a lesson too.</p>

    <Pull>You don't need a labeling project. Your team has been building one every time they filed something.</Pull>

    <h2>Two kinds of model</h2>
    <p>Archive turns those examples into one of two small models, each built for a specific job.</p>
    <h3>A sorter that picks the right kind of record</h3>
    <p>The first reads an incoming document, email or message and decides what kind of record it should become: an invoice, a contract, a resume, a maintenance request, or any other kind of record you keep. It learns your categories, not a generic list someone else invented, so it sorts the way your team already does.</p>
    <p>For a property manager, that might mean a steady stream of tenant emails split into repair requests, rent questions and lease changes. For a recruiting agency, it might mean resumes, client briefs and signed offers, each landing in the right place.</p>
    <h3>A filer that fills in the details</h3>
    <p>The second reads long text and turns it into filled-in records, field by field, in the same shape your team uses. Hand it a long contract, a full email thread or a whole report in one go, and it returns the client, the dates, the amounts and the terms, ready to save.</p>
    <p>For a finance team, that could be supplier invoices turned into records with the vendor, invoice number, due date and total already filled in. For an agency, a project brief becomes a project with its deadline, budget and contacts.</p>

    <Figure art="blog-train-your-own-model-2" alt="A long paper roll of text feeds into a machine, which prints neat record cards with filled-in blue fields into a tray" caption="The filer reads long text and hands back records with the fields already filled in." />

    <h2>Checked before it starts work</h2>
    <p>A model is only useful if it is right often enough to trust. Before a model sorts or files anything for real, Archive tests it on examples from your own records that it never saw during training, and shows you how it did. You see where it does well and where it still gets confused, using your own documents as the yardstick, not a public benchmark.</p>
    <p>If the results aren't good enough yet, the answer is usually more examples. Keep filing as you normally would, and train again when the collection has grown.</p>

    <h2>Served next to your data, at full precision</h2>
    <p>Once a model is ready, it runs right alongside your information in Archive. Your assistant sends it a new document, gets back a category or a filled-in record, and saves the result, working as the person who asked and within the same access rules as everything else.</p>
    <p>We also make a deliberate choice about quality. Many services save money by running a shrunken, compressed copy of a model, which is cheaper to run but tends to slip on exactly the unusual cases where filing goes wrong. Your models always run at full precision. They are never cut down to a smaller, lossier version.</p>

    <h2>Only what you can see, only what you choose</h2>
    <p>The examples a model learns from come from your own records, gathered with the same access rules as everything else. If you can't see a record, it can't end up in your training set. You decide which kinds of records to learn from, and the model learns from those alone.</p>
    <p>When your business changes shape, say you add a field to your contracts or start tracking a new kind of record, the filer won't know about it until you train it again. Gather a fresh set of examples and retrain, and it catches up with how you work now.</p>

    <h2>Start teaching today</h2>
    <p>You don't have to decide anything about models to get started. Bring your work into Archive and file it the way your team already does. Every record you save is quietly becoming a lesson, and when you are ready, your first set of examples is already waiting.</p>
    <p>Curious how documents get into Archive in the first place? Read <a href="/blog/messy-in-clean-out">from messy inbox to clean records</a>.</p>
</PostLayout>
