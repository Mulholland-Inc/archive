const SHA = /^[0-9a-f]{40}$/;
const JEV = 'https://api.typesafe.ai/v1/systemone';
const HIGH = 0.5, NOTE = 0.25;
const MARKER = '<!-- jev-review:';
const { RULES, UNREVIEWED } = require('./rules.cjs');

function eligible(pr, repository) {
  return pr.state === 'open' && !pr.draft && pr.base.ref === 'main' &&
    pr.base.repo.full_name === repository && pr.head.repo?.full_name === repository;
}

function firstAddedLine(patch) {
  const m = /^@@ -\d+(?:,\d+)? \+(\d+)/m.exec(patch);
  return m ? Number(m[1]) : 1;
}

async function judge(github, core, params, key) {
  const files = await github.paginate(github.rest.pulls.listFiles, { ...params, per_page: 100 });
  const reviewable = files.filter(f => f.status !== 'removed' && typeof f.patch === 'string' &&
    !UNREVIEWED.some(re => re.test(f.filename)));
  const started = Date.now();
  const findings = [];
  let failed = 0;
  const ask = async f => {
    const res = await fetch(JEV, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${key}` },
      body: JSON.stringify({
        model: 'jev-latest',
        state: { path: f.filename, status: f.status, patch: f.patch.slice(0, 200000) },
        questions: Object.fromEntries(Object.entries(RULES).map(([k, r]) =>
          [k, { type: 'noul', instructions: r.instructions, criteria: r.criteria }])),
      }),
    });
    if (!res.ok) throw new Error(`Jev answered ${res.status} for ${f.filename}`);
    const { answers } = await res.json();
    for (const [k, r] of Object.entries(RULES)) {
      const p = answers?.[k]?.noul;
      if (typeof p !== 'number') throw new Error(`Jev gave no answer to ${k} for ${f.filename}`);
      if (p < NOTE) continue;
      findings.push({ priority: p >= HIGH ? 'P0' : 'P1', path: f.filename, line: firstAddedLine(f.patch), percent: Math.round(p * 100), rule: r });
    }
  };
  const queue = [...reviewable];
  await Promise.all(Array.from({ length: 8 }, async () => {
    for (let f = queue.shift(); f; f = queue.shift()) {
      try { await ask(f); } catch (err) { failed++; core.warning(err.message); }
    }
  }));
  findings.sort((a, b) => a.priority.localeCompare(b.priority) || a.path.localeCompare(b.path));
  const seconds = ((Date.now() - started) / 1000).toFixed(1);
  const skipped = files.length - reviewable.length;
  const summary = failed
    ? `Jev could not read ${failed} of ${reviewable.length} changed files.`
    : `Jev read ${reviewable.length} changed file${reviewable.length === 1 ? '' : 's'} in ${seconds}s` +
      (skipped ? ` (${skipped} skipped: docs, tests, workflows, binaries)` : '') +
      (findings.length ? '.' : ' and found nothing.');
  return { summary, findings };
}

async function review({ github, context, core }, { number, key, bot }) {
  if (!Number.isSafeInteger(number) || number < 1) throw new Error('Invalid PR number');
  if (typeof key !== 'string' || !key) throw new Error('No TypeSafe key');
  if (!/^[a-z0-9-]+\[bot\]$/.test(bot)) throw new Error('Invalid reviewer identity');
  const repository = `${context.repo.owner}/${context.repo.repo}`;
  const params = { ...context.repo, pull_number: number };
  const { data: pr } = await github.rest.pulls.get(params);
  if (!eligible(pr, repository)) {
    core.notice('Only open, non-draft PRs from this repository into main are reviewed.');
    return;
  }
  const head = pr.head.sha, base = pr.base.sha;
  if (![head, base].every(sha => SHA.test(sha))) throw new Error('Invalid commit SHA');
  const { summary, findings } = await judge(github, core, params, key);
  const marker = `${MARKER}${base}:${head} -->`;
  const notes = findings.map(f => {
    const path = f.path.split('/').map(encodeURIComponent).join('/');
    return `\n\n### [${f.priority}] ${f.rule.title}\n\n[${f.path}:${f.line}](https://github.com/${repository}/blob/${head}/${path}#L${f.line})\n\n` +
      `Jev puts this at ${f.percent}% for ${f.path}: ${f.rule.instructions}`;
  }).join('');
  const body = `${marker}\n## Jev review\n\n${summary}${notes}`.slice(0, 60000);
  const current = async () => {
    const { data: now } = await github.rest.pulls.get(params);
    return eligible(now, repository) && now.head.sha === head && now.base.sha === base;
  };
  const reviews = await github.paginate(github.rest.pulls.listReviews, { ...params, per_page: 100 });
  const previous = reviews.filter(r => r.user?.login === bot && r.body?.includes(MARKER));
  if (!(previous.at(-1)?.body.includes(marker) && previous.at(-1)?.state === 'APPROVED')) {
    for (const r of previous.filter(r => r.state === 'APPROVED')) {
      await github.rest.pulls.dismissReview({ ...params, review_id: r.id, message: 'Superseded by a fresh Jev review.' });
    }
    if (!await current()) return core.notice('The PR moved while publishing; the run for its new head decides.');
    await github.rest.pulls.createReview({ ...params, commit_id: head, event: 'APPROVE', body });
  }
  if (!await current()) return core.notice('The PR moved after publishing; the run for its new head decides.');
  const data = await github.graphql(`query($owner: String!, $repo: String!, $number: Int!) {
    repository(owner: $owner, name: $repo) { pullRequest(number: $number) { id headRefOid baseRefOid mergeStateStatus } }
  }`, { ...context.repo, number });
  const latest = data.repository.pullRequest;
  if (latest.headRefOid !== head || latest.baseRefOid !== base) return core.notice('The PR moved before merging.');
  const operation = ['CLEAN', 'HAS_HOOKS', 'UNSTABLE'].includes(latest.mergeStateStatus) ? 'mergePullRequest' : 'enablePullRequestAutoMerge';
  try {
    await github.graphql(`mutation($id: ID!, $head: GitObjectID!) {
      ${operation}(input: {pullRequestId: $id, expectedHeadOid: $head, mergeMethod: SQUASH}) { clientMutationId }
    }`, { id: latest.id, head });
  } catch (err) {
    return core.notice(`Approved; GitHub did not merge from here: ${err.message}`);
  }
  core.notice('Approved. GitHub squash-merges when the required checks pass.');
}

module.exports = { review };
