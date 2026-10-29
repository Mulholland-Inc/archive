<script>
  import { clipboard } from './copy.svelte.js';

  let { tabs, children } = $props();

  let on = $state(0);
  let block;

  const clip = clipboard();
  const panes = () => [...block.querySelectorAll('pre')];

  $effect(() => {
    panes().forEach((pre, i) => (pre.hidden = i !== on));
  });
</script>

<div class="code" bind:this={block}>
  <div class="code-head">
    {#if tabs.length > 1}
      <span class="tabs" role="tablist"
        >{#each tabs as tab, i}<button role="tab" aria-selected={i === on} onclick={() => (on = i)}>{tab}</button>{/each}</span
      >
    {:else}
      <span>{tabs[0]}</span>
    {/if}
    <button class="copy" onclick={() => clip.copy(panes()[on].innerText)}>{clip.copied ? 'Copied' : 'Copy'}</button>
  </div>
  {@render children()}
</div>
