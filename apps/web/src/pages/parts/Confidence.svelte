<script lang="ts">
  import { pct, tone } from '../../lib/review';
  let { value }: { value: number } = $props();
</script>

<span class="gauge" data-tone={tone(value)} title={`${pct(value)} confident`} role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(value * 100)} aria-label="Confidence">
  <i style:width={pct(value)}></i>
</span>

<style>
  .gauge {
    display: inline-block;
    flex: none;
    width: 3.5rem;
    height: 0.3125rem;
    border-radius: var(--radius-full);
    background: var(--tint-3);
    overflow: hidden;
    vertical-align: middle;
  }
  i {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--c);
  }
  [data-tone='high'] {
    --c: var(--emerald-500);
  }
  [data-tone='mid'] {
    --c: var(--tangerine-500);
  }
  [data-tone='low'] {
    --c: var(--red-500);
  }
</style>
