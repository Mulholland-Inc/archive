<script>
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import Cta from './Cta.svelte';
  import { APP } from './site.js';
  import { post as find } from './posts.js';

  let { slug, cta, lead, related, children } = $props();

  const post = $derived(find(slug));
</script>

<svelte:head>
  <title>{post.title} — Archive</title>
  <meta name="description" content={post.description} />
  <meta name="theme-color" content="#dce8f7" />
</svelte:head>

<section class="post-hero">
  <h1>{post.title}</h1>
  <p class="lead">{post.lead}</p>
</section>

<div class="post-art"><Art name="blog-{post.slug}" alt={post.alt} /></div>

<article class="post">
  {@render children()}
</article>

<Cta title={cta} {lead}>
  {#snippet actions()}
    <Button href={APP}>Start building free</Button>
    {#each related as other}<Button href="/blog/{other}" ghost>{find(other).short}</Button>{/each}
  {/snippet}
</Cta>
