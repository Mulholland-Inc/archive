export type Field = {
  type: string;
  description?: string;
  required?: boolean;
  list?: boolean;
  indexed?: boolean;
  to?: string;
  enum?: string[];
};

export type TypeDef = {
  description?: string;
  icon?: string;
  extends?: string;
  category?: string;
  sensitivity?: 'public' | 'personal' | 'secret';
  schema?: Record<string, unknown>;
  fields?: { [name: string]: Field };
  examples?: unknown[];
  parse?: string;
  format?: string;
};

export type Spec = { version: number; types: { [name: string]: TypeDef } };

export type Ontology = { spec: Spec; builtin: string[]; edited: string[] };

export type ArchiveRecord = {
  id: string;
  type: string;
  spec_version: number;
  data: { [field: string]: unknown };
  created_at: string;
  updated_at: string;
  space: string;
};

export type Provenance = {
  id: string;
  record_id: string;
  field_path: string;
  raw_ids: string[];
  method: string;
  confidence: number;
  original?: string;
  space: string;
  file_id?: string;
  page?: number;
};

export type RecordLink = { id: string; from_id: string; to_id: string; kind: string; space: string; from_type: string };

export type ArchiveFile = {
  id: string;
  sha256: string;
  name: string;
  mime: string;
  size: number;
  metadata: { [key: string]: unknown };
  status: 'pending' | 'processing' | 'done' | 'error' | string;
  error?: string;
  intake?: string;
  space: string;
  created_at: string;
};

export type Source = {
  id: string;
  kind: string;
  name: string;
  space: string;
  created_by: string;
  type?: string;
  mode: string;
  schedule?: string;
  rate: number;
  config: { [key: string]: unknown };
  state: { [key: string]: unknown };
  status: string;
  error?: string;
  failures: number;
  last_run_at?: string;
  next_run_at?: string;
  created_at: string;
  auth_url?: string;
};

export type SyncResult = { source: string; fetched: number; new: number; mapped: number; status: string; error?: string };

export type View = {
  id: string;
  name: string;
  space: string;
  type: string;
  layout: 'table' | 'board' | 'calendar';
  by?: string;
  where: { [field: string]: string };
  created_by: string;
  created_at: string;
};

export type ViewInput = { name: string; space?: string; type: string; layout: string; by?: string; where: { [field: string]: string } };

export type SearchHit = {
  id: string;
  kind: 'record' | 'file' | 'raw' | string;
  type?: string;
  source?: string;
  space: string;
  date: string;
  data?: { [key: string]: unknown };
  snippet?: string;
  score: number;
};

export type Actor = { user_id: string; email: string; name: string; token_id?: string; token_name?: string; acting_as: string };

export type Space = { id: string; name: string; level: 'viewer' | 'editor' | 'admin' | 'owner' };

export type Member = { email: string; name: string; level: string };

export type Token = {
  id: string;
  name: string;
  spaces: { [space: string]: string };
  full?: boolean;
  expires_at?: string;
  created_at: string;
  token?: string;
};

export type Op = { op: string; type: string; field?: string; to?: string; def?: unknown };

export type Change = {
  id: string;
  proposed_by: string;
  status: string;
  ops: Op[];
  from_version: number;
  to_version: number;
  created_at: string;
};

export type PreviewRecord = {
  id: string;
  type: string;
  before?: { [k: string]: unknown };
  after?: { [k: string]: unknown };
  errors?: { path: string; message: string }[];
};

export type ChangePreview = {
  change: Change;
  touched: number;
  failing: number;
  failures?: PreviewRecord[];
  samples?: PreviewRecord[];
};

export type Me = { actor: Actor; spaces: Space[]; home: string };

export type SigninMethods = { providers: string[]; mail: boolean };
