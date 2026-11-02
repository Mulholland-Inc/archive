// Syntax highlighting for code blocks: highlight.js's core with only the languages Archive shows, so the
// bundle carries none of the others. A language it does not have is shown plain.
import hljs from 'highlight.js/lib/core';
import typescript from 'highlight.js/lib/languages/typescript';
import json from 'highlight.js/lib/languages/json';
import sql from 'highlight.js/lib/languages/sql';

hljs.registerLanguage('typescript', typescript);
hljs.registerAliases(['ts', 'js', 'javascript'], { languageName: 'typescript' });
hljs.registerLanguage('json', json);
hljs.registerLanguage('sql', sql);

const escapes = { '&': '&amp;', '<': '&lt;', '>': '&gt;' };

// highlighted is code as HTML: its text escaped, and its tokens in spans of highlight.js's classes.
export function highlighted(code, lang) {
  const name = lang?.toLowerCase();
  if (name && hljs.getLanguage(name)) return hljs.highlight(code, { language: name, ignoreIllegals: true }).value;
  return code.replace(/[&<>]/g, (c) => escapes[c]);
}
