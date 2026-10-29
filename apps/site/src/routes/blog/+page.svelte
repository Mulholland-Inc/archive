<script>
  import BlogCard from '#lib/BlogCard.svelte';
  import { POSTS } from '#lib/posts.js';

  const [lead, ...rest] = POSTS;
</script>

<svelte:head>
  <title>Blog — Archive</title>
  <meta name="description" content="Notes on Archive: how it compares, how teams use it with their AI, and why we built it." />
  <meta name="theme-color" content="#0b0d12" />
</svelte:head>

<section class="blog-head">
  <div class="wrap">
    <h1>Blog</h1>
    <p>How Archive compares, how teams use it with their AI, and why we built it the way we did.</p>
  </div>
</section>

<section class="blog-top">
  <div class="wrap"><BlogCard post={lead} lead /></div>
</section>

<section class="blog-rest">
  <div class="wrap blog-grid">
    {#each rest as post}<BlogCard {post} />{/each}
  </div>
</section>
