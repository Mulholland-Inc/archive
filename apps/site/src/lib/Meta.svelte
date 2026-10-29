<script>
  import { page } from '$app/state';
  import { SITE } from './site.js';

  let { title, description } = $props();

  const url = $derived(SITE + page.url.pathname);
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={description} />
  <link rel="canonical" href={url} />
  <meta property="og:url" content={url} />
  <meta property="og:title" content={title} />
  <meta property="og:description" content={description} />
  <meta name="twitter:title" content={title} />
  <meta name="twitter:description" content={description} />
</svelte:head>
