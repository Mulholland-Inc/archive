<script>
  let { tabs, children } = $props();

  let on = $state(0);
  let copied = $state(false);
  let block;

  const panes = () => [...block.querySelectorAll('pre')];

  $effect(() => {
    panes().forEach((pre, i) => (pre.hidden = i !== on));
  });

  function copy() {
    navigator.clipboard.writeText(panes()[on].innerText);
    copied = true;
    setTimeout(() => (copied = false), 1400);
  }
</script>

<div class="code" bind:this={block}>
  <div class="code-bar">
    <span class="dots"><i></i><i></i><i></i></span><span class="tabs" role="tablist"
      >{#each tabs as tab, i}<button role="tab" aria-selected={i === on} onclick={() => (on = i)}>{tab}</button>{/each}</span
    ><button class="copy" onclick={copy}>{copied ? 'Copied' : 'Copy'}</button>
  </div>
  {@render children()}
</div>
