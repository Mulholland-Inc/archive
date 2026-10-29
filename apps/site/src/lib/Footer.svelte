<script>
  import Word from '#kit/Word.svelte';
  import { GITHUB, PRODUCT } from './site.js';

  const link = (p) => [p.href, p.title];
  const COLUMNS = [
    ['Product', PRODUCT.slice(0, 4).map(link)],
    ['Platform', PRODUCT.slice(4).map(link)],
    ['Resources', [['/docs', 'Docs'], ['/blog', 'Blog'], ['/pricing', 'Pricing'], [GITHUB, 'GitHub']]],
    ['Company', [['https://mulholland.ai', 'Mulholland'], ['mailto:hello@mulholland.ai', 'Contact'], ['/security', 'Trust'], [GITHUB + '/issues', 'Support']]]
  ];
</script>

<footer class="foot">
  <div class="wrap">
    <div class="foot-grid">
      <div class="stack">
        <a class="brand" href="/" aria-label="Archive home"><Word /></a>
        <p class="small">The workspace with a real database underneath. Made by Mulholland.</p>
      </div>
      {#each COLUMNS as [title, links]}
        <div>
          <h4>{title}</h4>
          <ul>
            {#each links as [href, label]}<li><a {href}>{label}</a></li>{/each}
          </ul>
        </div>
      {/each}
    </div>
    <div class="big" aria-hidden="true"><Word class="" /></div>
    <div class="fine"><span>© {new Date().getFullYear()} Mulholland, Inc.</span><span>Archive</span></div>
  </div>
</footer>
