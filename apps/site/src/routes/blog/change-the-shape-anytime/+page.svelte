<script>
  import PostLayout from '#lib/PostLayout.svelte';
  import Figure from '#lib/Figure.svelte';
  import Pull from '#lib/Pull.svelte';
</script>

<PostLayout
  slug="change-the-shape-anytime"
  cta="Shape it your way. Change it anytime."
  lead="Free for your first workspace. No card, no sales call."
  related={['messy-in-clean-out', 'archive-vs-airtable']}
>
    <p>The way you organize information on day one is a guess. A good guess, usually, but still a guess. Then the business moves.</p>
    <p>A design agency that sold projects starts selling monthly retainers. A property manager that handled long-term leases takes on a few short-term rentals. A finance team that tracked one budget splits it across three cost centers. Each change sounds small. Each one quietly breaks the way information was set up.</p>
    <p>In many tools, that is the moment people stop updating the system and start keeping a side spreadsheet. Archive is built so you never have to.</p>

    <h2>Your structure should move with you</h2>
    <p>In Archive, your records are shaped the way your business works: clients, projects, units, invoices, each with its own fields and the links between them. That shape is not fixed. You can change it whenever the business changes, while the team keeps working.</p>
    <p>The changes people make most often are simple:</p>
    <ul>
      <li><strong>Add a field.</strong> The agency adds a renewal date and a monthly amount to its clients.</li>
      <li><strong>Add a new kind of record.</strong> The agency adds retainers, each linked to a client, with its own hours and terms.</li>
      <li><strong>Rename something.</strong> "Leads" become "Prospects". "City" becomes "Town". "Projects" become "Engagements".</li>
      <li><strong>Remove what you no longer use.</strong> An old status nobody picks, a field from a process you retired.</li>
    </ul>
    <p>You can make these changes yourself, or ask your AI assistant to do it. "Add retainers to our workspace, linked to clients, with a start date, a monthly amount and included hours" is a perfectly good instruction. Either way, the same safety rules apply.</p>

    <h2>See exactly what will change, before it happens</h2>
    <p>Changing how information is organized should never be a leap of faith. Before anything is applied, Archive shows you a preview of the change: what will be added, what will be renamed, what will be removed, and which steps carry a risk worth a second look.</p>

    <Figure art="blog-change-the-shape-anytime-2" alt="A plan board beside a control desk with a lever and a guarded button, and a magnifier held over a single record block with a tag on it" caption="Look at the plan first. Apply it when you are happy. The risky button has a guard on it." />

    <p>When you approve it, the change is applied all at once. Either every part of it lands or none of it does, so your workspace never ends up half changed. Everyone connected sees the new shape right away, including every AI assistant, which always works from the current version.</p>

    <Pull>Renaming a field should never cost you the information in it. Removing one should never happen by accident.</Pull>

    <h2>Nothing deleted by accident</h2>
    <p>The scariest change is the one that throws information away. Archive treats it that way. If a change would remove a field or a kind of record that holds information, it is blocked unless you explicitly say that is what you want. A tidy-up that would quietly wipe a year of notes does not go through on a misclick, and it does not go through because an assistant misunderstood an instruction.</p>
    <p>Some removals are harmless, and Archive knows the difference. Taking away a board or a calendar removes a way of looking at your records, not the records themselves, so it goes through without fuss.</p>

    <h2>Renames keep everything</h2>
    <p>A rename is not a delete followed by an add. When "City" becomes "Town", every value that was in City is in Town afterwards. When "Leads" become "Prospects", every prospect still has its history, its files and its links to everything else.</p>
    <p>That sounds obvious. It is worth saying because in many systems a rename is exactly where data goes missing: a field quietly re-created, an import that matched on the old name, an automation still writing to a field that no longer exists. In Archive, you say it is a rename, and it is treated as one.</p>

    <h2>Boards, calendars and lists follow along</h2>
    <p>Most people never look at the raw shape of their information. They look at a board of jobs by status, a calendar of lease renewals, a list of open invoices. In Archive, those are simply different ways of looking at the same records, so they move with your changes.</p>

    <Figure art="views" alt="A stand holding a board with cards in lanes, a calendar grid and a spreadsheet-like list" caption="Boards, calendars and lists are ways of looking at the same records, so they update with them." />

    <p>Add a renewal date to clients, and you can put clients on a calendar by renewal date the same afternoon. Add a "stage" to retainers, and a board grouped by stage is one step away. Rename a field, and every board and list that used it keeps working under the new name. Nobody has to rebuild a dashboard because the business learned something.</p>
    <p>And because every record follows the same access rules, a new kind of record is protected from the start. Only the right people, and the right AI, see it.</p>

    <h2>What this changes day to day</h2>
    <p>The real benefit is not any single feature. It is that changing your setup stops being a project. The property manager adds short-term rentals on a Monday, with nightly rates, cleaning turnovers and a calendar of bookings, and the team uses it on Tuesday. The finance team splits its budget into cost centers without exporting anything to a spreadsheet and back. The agency's retainers show up next to its clients, and its AI assistant starts filing hours against them the same week.</p>
    <p>Your business will keep changing shape. Now your workspace can keep up, and you will not lose a thing along the way.</p>
</PostLayout>
