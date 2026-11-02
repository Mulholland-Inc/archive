// Syntax highlighting for code blocks, without a library: code split into runs of one kind each, which
// CodeBlock colours. TypeScript and JavaScript, and SQL, are told apart; any other language is one plain run.

const ts = new Set(
  'abstract as async await break case catch class const continue debugger declare default delete do else enum export extends false finally for from function get if implements import in instanceof interface keyof let new null of private protected public readonly return satisfies set static super switch this throw true try type typeof undefined var void while with yield'.split(' ')
);

const sql = new Set(
  'add all alter and any as asc between by case cascade check column comment constraint create default delete desc distinct do drop else end exists false foreign from full function group having if in index inner insert into is join key left like limit not null of offset on or order outer policy primary references rename returning right schema select set table then to trigger true type union unique update using values view when where with'.split(' ')
);

// One token at a time: a comment, a string, a number, a word, or one character of anything else. SQL's
// comments start with --, which in TypeScript is a decrement, and its strings are only single-quoted.
const tsToken = /(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|('(?:\\.|[^'\\\n])*'?|"(?:\\.|[^"\\\n])*"?|`(?:\\.|[^`\\])*`?)|(\b\d[\d_]*(?:\.\d+)?\b)|([A-Za-z_$][\w$]*)|([\s\S])/g;
const sqlToken = /(--[^\n]*|\/\*[\s\S]*?\*\/)|('(?:''|[^'])*'?)|(\b\d+(?:\.\d+)?\b)|([A-Za-z_][\w$]*)|([\s\S])/g;

const langs = { ts, typescript: ts, js: ts, javascript: ts, sql };

// highlight is code as [{ text, kind }], kind one of comment, string, number, keyword, fn, or '' for plain.
export function highlight(code, lang = 'text') {
  const words = langs[lang?.toLowerCase()];
  if (!words) return [{ text: code, kind: '' }];
  const isSQL = words === sql;
  const out = [];
  const push = (text, kind) => {
    const last = out.at(-1);
    if (last && last.kind === kind) last.text += text;
    else out.push({ text, kind });
  };
  for (const m of code.matchAll(isSQL ? sqlToken : tsToken)) {
    const [text, comment, string, number, word] = m;
    if (comment) push(text, 'comment');
    else if (string) push(text, 'string');
    else if (number) push(text, 'number');
    else if (word && words.has(isSQL ? word.toLowerCase() : word)) push(text, 'keyword');
    else if (word && !isSQL && /^\s*\(/.test(code.slice(m.index + text.length, m.index + text.length + 8))) push(text, 'fn');
    else push(text, '');
  }
  return out;
}
