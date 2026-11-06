import hljs from 'highlight.js/lib/core';
import typescript from 'highlight.js/lib/languages/typescript';
import json from 'highlight.js/lib/languages/json';
import sql from 'highlight.js/lib/languages/sql';

hljs.registerLanguage('typescript', typescript);
hljs.registerAliases(['ts', 'js', 'javascript'], { languageName: 'typescript' });
hljs.registerLanguage('json', json);
hljs.registerLanguage('sql', sql);

const escapes = { '&': '&amp;', '<': '&lt;', '>': '&gt;' };

export function highlighted(code, lang) {
  const name = lang?.toLowerCase();
  if (name && hljs.getLanguage(name)) return hljs.highlight(code, { language: name, ignoreIllegals: true }).value;
  return code.replace(/[&<>]/g, (c) => escapes[c]);
}
