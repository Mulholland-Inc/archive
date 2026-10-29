<script>
  import '../tokens.css';
  import '../site.css';
  import { beforeNavigate, afterNavigate } from '$app/navigation';
  import Nav from '#lib/Nav.svelte';
  import Footer from '#lib/Footer.svelte';

  let { children } = $props();

  beforeNavigate(({ from, to }) => {
    if (from?.url.pathname !== to?.url.pathname) document.documentElement.style.scrollBehavior = 'auto';
  });

  afterNavigate(() => {
    document.documentElement.style.scrollBehavior = '';
  });
</script>

<Nav />
<main>{@render children()}</main>
<Footer />
