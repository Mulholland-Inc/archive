<script>
  let { steps, columns = false } = $props();
</script>

<ul class={['letters', columns && 'columns']}>
  {#each steps as [name, text], i}
    <li><span class="l">{String.fromCharCode(65 + i)}</span><div><h3>{name}</h3><p>{text}</p></div></li>
  {/each}
</ul>
