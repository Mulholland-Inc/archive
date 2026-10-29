<script>
  import '../tokens.css';
  import '../site.css';
  import { beforeNavigate, afterNavigate } from '$app/navigation';
  import Nav from '#lib/Nav.svelte';
  import Footer from '#lib/Footer.svelte';

  let { children } = $props();

  let main;

  beforeNavigate(({ from, to }) => {
    if (from?.url.pathname !== to?.url.pathname) document.documentElement.classList.add('jump');
  });

  afterNavigate(({ from, to }) => {
    document.documentElement.classList.remove('jump');
    if (!from || from.url.pathname === to?.url.pathname) return;
    main.classList.remove('enter');
    void main.offsetWidth;
    main.classList.add('enter');
  });
</script>

<Nav />
<main bind:this={main}>{@render children()}</main>
<Footer />
