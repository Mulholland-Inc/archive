<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';

  let dark = $state(false);

  function paint() {
    const root = document.documentElement;
    if (dark) root.dataset.theme = 'dark';
    else delete root.dataset.theme;
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.content = getComputedStyle(root).getPropertyValue('--bg').trim();
  }

  function toggle() {
    dark = !dark;
    paint();
    try {
      localStorage.theme = dark ? 'dark' : 'light';
    } catch {}
  }

  onMount(() => {
    dark = document.documentElement.dataset.theme === 'dark';
    paint();
  });
</script>

<button class="icon-btn" type="button" aria-label={dark ? 'Switch to light mode' : 'Switch to dark mode'} onclick={toggle}><Icon name={dark ? 'sun' : 'moon'} /></button>
