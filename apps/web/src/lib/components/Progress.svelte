<script lang="ts">
  let { value, label }: { value: number; label: string } = $props();
</script>

<div class="progress" role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(value * 100)}>
  <span class="bar" style:--value={value}></span>
</div>

<style>
  .progress {
    width: 100%;
    height: var(--progress-height);
    border-radius: var(--radius-full);
    background: var(--tint-2);
    overflow: hidden;
  }
  .bar {
    display: block;
    height: 100%;
    width: 100%;
    border-radius: inherit;
    background: var(--accent);
    transform: scaleX(var(--value));
    transform-origin: left;
    transition: transform var(--duration-normal) var(--ease-out);
  }
</style>
