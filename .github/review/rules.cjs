const RULES = {
  space: {
    title: 'A user can reach rows of a space they are not in',
    instructions: 'Does this change let one user read or write rows of a space they are not a member of? Every data table (records, raw, files, provenance, links, sources, views, proposals, secrets, embeddings) carries a space column and forces row-level security keyed on my_spaces(), and requests run as the user\'s own Postgres role through SET LOCAL ROLE, so a query carries no space filter of its own: the policy scopes it. A leak is a policy that stops keying on my_spaces(), a new data table without a space column or without enabled and forced row-level security, a GRANT that widens a role, or a request path that reads user data through System() (the owner connection that skips row-level security) instead of As().',
    criteria: {
      true: 'A policy no longer keys on my_spaces(), a CREATE TABLE for user data has no space column or no enable/force row level security, a GRANT widens a space or token role, or a request handler reads or writes user rows through System()',
      false: 'Tables carry space and forced policies, queries rely on those policies and run through As(), System() is used only for instance-wide bookkeeping, or the patch touches nothing that reads or writes rows',
    },
  },
  authorization: {
    title: 'An authorization or role check is missing or weakened',
    instructions: 'Does this change remove, bypass or weaken an authorization check on a path that had one: a space role (owner, admin, editor, viewer), the instance admin check (adminOnly), ontology changes needing admin, secret type grants, or a token narrowing its user\'s rights?',
    criteria: {
      true: 'A role, grant, adminOnly or secret-grant check is deleted, made optional, or its failure no longer stops the request, or a token can gain rights its user lacks',
      false: 'Authorization checks are untouched, added or tightened, or the patch has no request path',
    },
  },
  credential: {
    title: 'A credential, key or token leaves its store',
    instructions: 'Does this change leak a secret: an API or session token, a magic link, an OAuth or connector credential, the instance secret key, a value of a secret type (stored encrypted in the secrets table and masked by default), or a password? A leak is writing it to a log, an HTTP or MCP response, an env var or source, or keeping it outside its store. Tokens are stored as a sha256 hash in tokens; printing a freshly minted token once to the person who asked for it is not a leak.',
    criteria: {
      true: 'A token, key, password, connector credential or unmasked secret-type value is logged, returned to someone not granted it, put in source or an env var, or stored unhashed or unencrypted',
      false: 'Secrets stay hashed or encrypted in their table, secret-type values stay masked for anyone without a grant, a new token is shown once to its owner, or the patch handles no secret',
    },
  },
  injection: {
    title: 'Request input reaches a query, shell or path unsanitised',
    instructions: 'Does this change let input from a request, a connector or an uploaded file reach a SQL query, a shell command, a file path or an unsafe deserializer without parameterisation or validation? Identifiers built into SQL must go through pgx.Identifier or authIdent, and values must be bound parameters.',
    criteria: {
      true: 'Request-, connector- or file-controlled text is concatenated into SQL, a command line or a path, or reaches a deserializer that executes or resolves types',
      false: 'Input is bound as a parameter, identifiers are sanitised, input is validated against a closed set, or the patch handles no outside input',
    },
  },
  authentication: {
    title: 'An authentication, session or signature check can be skipped',
    instructions: 'Does this change let a request skip authentication: an /api, /mcp or file route that reaches user data without a bearer token or session cookie resolved against its stored hash, a signed file link whose signature or expiry is no longer checked, an OAuth or passkey flow whose state, nonce or origin is no longer verified, or a setup link usable more than once?',
    criteria: {
      true: 'A handler reaches user data with no token, session or signed-link check where its neighbours have one, or a signature, state, nonce, origin or expiry check is removed or its failure ignored',
      false: 'Every handler in the patch is behind a token, a session or a verified signed link, the patch only serves static files or health checks, or it adds no reachable entry point',
    },
  },
};

const UNREVIEWED = [
  /\.md$/i, /(^|\/)e2e_test\.go$/, /_test\.go$/, /(^|\/)testdata\//, /^\.github\//,
  /(^|\/)(package-lock\.json|go\.sum)$/, /\.(png|jpg|jpeg|gif|svg|webp|ico|woff2?|ttf|pdf)$/i,
  /(^|\/)demo\.json$/,
];

module.exports = { RULES, UNREVIEWED };
