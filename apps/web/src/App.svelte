<script lang="ts">
  import { Dock } from './lib';
  import Dashboard from './pages/Dashboard.svelte';
  import Kit from './pages/Kit.svelte';

  const routes = [
    { value: 'dashboard', label: 'Dashboard' },
    { value: 'specimen', label: 'Specimen' }
  ];

  const read = () => {
    const r = location.hash.replace('#/', '');
    return routes.some((x) => x.value === r) ? r : 'dashboard';
  };

  let route = $state(read());
  let theme = $state<'light' | 'dark'>((localStorage.getItem('theme') as 'light' | 'dark') ?? 'light');

  $effect(() => {
    if (location.hash !== `#/${route}`) history.replaceState(null, '', `#/${route}`);
  });

  $effect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('theme', theme);
  });

  function toggleTheme() {
    const next = theme === 'dark' ? 'light' : 'dark';
    const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown };
    if (doc.startViewTransition && !matchMedia('(prefers-reduced-motion: reduce)').matches) {
      doc.startViewTransition(() => {
        theme = next;
        document.documentElement.dataset.theme = next;
      });
    } else theme = next;
  }
</script>

<svelte:window onhashchange={() => (route = read())} />

{#if route === 'dashboard'}
  <Dashboard />
{:else}
  <Kit />
{/if}

<Dock bind:route {routes} {theme} ontheme={toggleTheme} />
