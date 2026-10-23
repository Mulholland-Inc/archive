import { reducedMotion } from './motion.js';

const targets = document.querySelectorAll('[data-reveal]');

if (!reducedMotion.matches && 'IntersectionObserver' in window && targets.length) {
  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        entry.target.classList.add('is-revealed');
        observer.unobserve(entry.target);
      }
    },
    { rootMargin: '0px 0px -12% 0px' },
  );
  const view = window.innerHeight;
  for (const target of targets) {
    if (target.getBoundingClientRect().top < view * 0.9) target.classList.add('is-revealed');
    else observer.observe(target);
  }
  document.documentElement.classList.add('reveal-ready');
}
