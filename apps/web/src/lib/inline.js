const pattern = /(\*\*[^*]+\*\*|`[^`]+`|\*[^*]+\*|\[[^\]]+\]\([^)]+\))/g;

export function inline(text) {
  const out = [];
  let last = 0;
  for (const match of text.matchAll(pattern)) {
    if (match.index > last) out.push({ type: 'text', text: text.slice(last, match.index) });
    const token = match[0];
    if (token.startsWith('**')) out.push({ type: 'strong', text: token.slice(2, -2) });
    else if (token.startsWith('`')) out.push({ type: 'code', text: token.slice(1, -1) });
    else if (token.startsWith('[')) {
      const [, label, url] = token.match(/\[([^\]]+)\]\(([^)]+)\)/);
      out.push({ type: 'link', text: label, url });
    } else out.push({ type: 'em', text: token.slice(1, -1) });
    last = match.index + token.length;
  }
  if (last < text.length) out.push({ type: 'text', text: text.slice(last) });
  return out;
}
