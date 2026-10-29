<script>
  import BlogCard from '#lib/BlogCard.svelte';
  import Hero from '#lib/Hero.svelte';
  import { POSTS } from '#lib/posts.js';

  const [lead, ...rest] = POSTS;
</script>

<svelte:head>
  <title>Blog — Archive</title>
  <meta name="description" content="Notes on Archive: how it compares, how teams use it with their AI, and why we built it." />
</svelte:head>

<Hero kind="blog" title="Blog" lead="How Archive compares, how teams use it with their AI, and why we built it the way we did." />

<section>
  <div class="wrap"><BlogCard post={lead} lead /></div>
</section>

<section>
  <div class="wrap blog-grid">
    {#each rest as post}<BlogCard {post} />{/each}
  </div>
</section>
