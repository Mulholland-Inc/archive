<script>
  import Cta from './Cta.svelte';
  import Hero from './Hero.svelte';
  import Meta from './Meta.svelte';
  import { START } from './site.js';
  import { post as find } from './posts.js';

  let { slug, cta, lead, related, children } = $props();

  const post = $derived(find(slug));
  const links = $derived([START, ...related.map((other) => [`/blog/${other}`, find(other).short])]);
</script>

<Meta title="{post.title} — Archive" description={post.description} />

<Hero kind="post" title={post.title} lead={post.lead} art="blog-{post.slug}" alt={post.alt} />

<article class="post">
  {@render children()}
</article>

<Cta title={cta} {lead} {links} />
