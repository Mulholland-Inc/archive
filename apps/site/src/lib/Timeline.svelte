<script>
  import { reveal } from './reveal.js';

  let { entries } = $props();

  const label = (date) =>
    new Date(date + 'T12:00:00Z').toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' });
</script>

<div class="timeline">
  {#each entries as entry}
    <article class="entry reveal" id={entry.id} use:reveal>
      <time datetime={entry.id.slice(0, 10)}>{label(entry.id.slice(0, 10))}</time>
      <div>
        <h3>{entry.title}</h3>
        <ul>
          {#each entry.items as item}<li>{item}</li>{/each}
        </ul>
      </div>
    </article>
  {/each}
</div>
