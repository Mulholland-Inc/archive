<script>
  import '../tokens.css';
  import '../site.css';
  import { beforeNavigate, afterNavigate, onNavigate } from '$app/navigation';
  import Nav from '#lib/Nav.svelte';
  import Footer from '#lib/Footer.svelte';

  let { children } = $props();

  beforeNavigate(({ from, to }) => {
    if (from?.url.pathname !== to?.url.pathname) document.documentElement.classList.add('jump');
  });

  onNavigate((nav) => {
    if (!document.startViewTransition || nav.from?.url.pathname === nav.to?.url.pathname) return;
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    return new Promise((resolve) => {
      document.startViewTransition(async () => {
        resolve();
        await nav.complete;
      });
    });
  });

  afterNavigate(() => {
    document.documentElement.classList.remove('jump');
  });
</script>

<Nav />
<main>{@render children()}</main>
<Footer />
