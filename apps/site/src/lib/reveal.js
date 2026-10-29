let observer;

export function reveal(node, on = true) {
  if (!on) return;
  observer ??= new IntersectionObserver(
    (entries) =>
      entries.forEach((e) => {
        if (!e.isIntersecting) return;
        e.target.classList.add('in');
        observer.unobserve(e.target);
      }),
    { rootMargin: '0px 0px -8% 0px' }
  );
  observer.observe(node);
  return { destroy: () => observer.unobserve(node) };
}
