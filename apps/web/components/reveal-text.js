import { reducedMotion, clamp } from './motion.js';

function wrapWords(root) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const nodes = [];
  while (walker.nextNode()) nodes.push(walker.currentNode);
  const words = [];
  for (const node of nodes) {
    const parts = node.textContent.split(/(\s+)/);
    const fragment = document.createDocumentFragment();
    for (const part of parts) {
      if (!part) continue;
      if (/^\s+$/.test(part)) {
        fragment.append(part);
        continue;
      }
      const span = document.createElement('span');
      span.className = 'word';
      span.textContent = part;
      span.style.setProperty('--i', String(words.length));
      words.push(span);
      fragment.append(span);
    }
    node.replaceWith(fragment);
  }
  return words;
}

class RevealText extends HTMLElement {
  connectedCallback() {
    if (this.words || reducedMotion.matches) return;
    this.words = wrapWords(this);
    if (this.getAttribute('on') === 'load') {
      void this.offsetWidth;
      this.classList.add('is-on');
      return;
    }
    this.frame = 0;
    this.onScroll = () => {
      if (!this.frame) this.frame = requestAnimationFrame(() => this.update());
    };
    window.addEventListener('scroll', this.onScroll, { passive: true });
    window.addEventListener('resize', this.onScroll, { passive: true });
    this.update();
  }

  disconnectedCallback() {
    window.removeEventListener('scroll', this.onScroll);
    window.removeEventListener('resize', this.onScroll);
  }

  update() {
    this.frame = 0;
    const rect = this.getBoundingClientRect();
    const view = window.innerHeight;
    const startTop = view * 0.85;
    const endTop = view * 0.6 - rect.height;
    const progress = clamp((startTop - rect.top) / (startTop - endTop));
    const lit = Math.round(progress * this.words.length);
    this.words.forEach((word, index) => word.classList.toggle('is-lit', index < lit));
  }
}

customElements.define('reveal-text', RevealText);
