export const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');

export const clamp = (value, min = 0, max = 1) => Math.min(max, Math.max(min, value));
