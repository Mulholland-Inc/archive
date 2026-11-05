import { Pencil, Trash2, Plus, ExternalLink, MessageSquarePlus, Share, Download, Eye, Play, RotateCcw, Copy, Users, Ban, Settings, ArrowRightLeft, FolderInput } from '@lucide/svelte';
import { SEPARATOR } from './contextmenu.svelte.js';
import { app, deleteGroup, groupUses, updateGroup, toggleGroup, createGroup, isArchiveOwner, ws, removeAgent, askAgent, renameFile, removeFile, downloadFile, openShare, runWorkflow, removeWorkflow, removeMember, stopSharing, removeGuest, switchArchive, openSettings, deleteArchive } from './app.svelte.js';
import { prompt, confirmDialog, notify } from './dialogs.svelte.js';
import { clearAgent } from './agent.svelte.js';
import { go } from './router.svelte.js';

async function copyText(text) {
  await navigator.clipboard?.writeText(text).catch(() => {});
}

async function deleteAgent(agent) {
  if (await confirmDialog({ title: `Delete ${agent.name}?`, body: 'Everyone’s conversations with it go too.', confirm: 'Delete' })) removeAgent(agent.id);
}

export async function clearConversation(agent) {
  if (!(await confirmDialog({ title: `Clear your conversation with ${agent.name}?`, body: 'It forgets everything you said to it. Nobody else’s conversation changes.', confirm: 'Clear' }))) return;
  try {
    await clearAgent(agent.id);
  } catch (e) {
    notify({ title: 'Couldn’t clear the conversation', body: e.message });
  }
}

export function agentItems(agent) {
  return [
    { label: 'Settings', icon: Settings, onselect: () => (app.agentDialog = agent.id) },
    SEPARATOR,
    { label: 'Delete', icon: Trash2, danger: true, onselect: () => deleteAgent(agent) }
  ];
}

export function askAboutFile(file) {
  go(askAgent(`What is in ${file.name}?`, [{ id: file.id, name: file.name, kind: file.kind, size: file.size, tone: file.tone }]));
}

export async function renameFilePrompt(file) {
  const name = await prompt({ title: 'Rename file', label: 'Name', value: file.name });
  renameFile(file, name);
}

export function fileItems(file, { onopen, onmove, onaccess } = {}) {
  return [
    onopen && { label: 'Open', icon: Eye, onselect: () => onopen(file) },
    { label: 'Ask about this', icon: MessageSquarePlus, onselect: () => askAboutFile(file) },
    SEPARATOR,
    !file.connection && { label: 'Rename', icon: Pencil, onselect: () => renameFilePrompt(file) },
    !file.connection && onmove && { label: 'Move', icon: FolderInput, onselect: () => onmove(file) },
    !file.connection && file.kind === 'folder' && onaccess && { label: 'Who sees it', icon: Users, onselect: () => onaccess(file) },
    { label: 'Share', icon: Share, onselect: () => openShare('file', file.id) },
    file.held && file.kind !== 'folder' && { label: 'Download', icon: Download, onselect: () => downloadFile(file) },
    !file.connection && SEPARATOR,
    !file.connection && { label: 'Delete', icon: Trash2, danger: true, onselect: () => removeFile(file.id) }
  ].filter(Boolean);
}

export function askAboutTable(table) {
  go(askAgent(`What is in the ${table.name} table?`));
}

export function tableItems(table) {
  return [
    { label: 'Open', icon: ExternalLink, onselect: () => go(`/database/${table.id}`) },
    { label: 'Ask about this', icon: MessageSquarePlus, onselect: () => askAboutTable(table) },
    SEPARATOR,
    { label: 'Share', icon: Share, onselect: () => openShare('table', table.id) },
    { label: 'Copy name', icon: Copy, onselect: () => copyText(table.id) }
  ];
}

export async function deleteWorkflow(workflow) {
  if (await confirmDialog({ title: 'Delete this workflow?', body: `“${workflow.name}” and its runs will be removed.`, confirm: 'Delete' })) removeWorkflow(workflow.id);
}

export function runItems(run, workflow, { onrun = () => runWorkflow(workflow.id) } = {}) {
  if (run && ['running', 'waiting'].includes(run.status)) return [{ label: 'Running…', icon: Play, disabled: true }];
  return [{ label: run && ['failed', 'stopped'].includes(run.status) ? 'Run again' : 'Run now', icon: run && ['failed', 'stopped'].includes(run.status) ? RotateCcw : Play, onselect: onrun }];
}

export function workflowItems(workflow, { open = true, onrun, ondelete = () => deleteWorkflow(workflow) } = {}) {
  return [
    open && { label: 'Open', icon: ExternalLink, onselect: () => go(`/workflows/${workflow.id}`) },
    ...runItems(workflow.runs[0], workflow, onrun && { onrun }),
    SEPARATOR,
    { label: 'Share', icon: Share, onselect: () => openShare('workflow', workflow.id) },
    SEPARATOR,
    { label: 'Delete', icon: Trash2, danger: true, onselect: ondelete }
  ];
}

// removeMemberConfirm removes member, or revokes their invitation, once you confirm.
export async function removeMemberConfirm(member) {
  const invited = member.status === 'invited';
  if (await confirmDialog({ title: invited ? `Revoke ${member.name}’s invitation?` : `Remove ${member.name}?`, body: invited ? 'The invitation stops working.' : 'They lose access to this archive.', confirm: invited ? 'Revoke' : 'Remove' })) removeMember(member.id);
}

// removeGuestConfirm ends everything shared with guest once you confirm.
export async function removeGuestConfirm(guest) {
  if (await confirmDialog({ title: `Remove ${guest.name}?`, body: 'Nothing stays shared with them.', confirm: 'Remove' })) removeGuest(guest.email);
}

// usesText says where a group is used, like 3 rows in invoice and 1 in file.
export function usesText(uses) {
  return uses.map((u, i) => `${u.rows} ${i === 0 ? (u.rows === 1 ? 'row' : 'rows') + ' ' : ''}in ${u.table}`).join(', ');
}

// deleteGroupConfirm deletes group once you confirm, and only when no row's access is it: a row whose group went
// would be seen by everyone, so Archive refuses, and this says where it is used instead.
export async function deleteGroupConfirm(group) {
  const uses = await groupUses(group.id).catch(() => []);
  if (uses.length) {
    await notify({ title: `${group.name} is in use`, body: `It is the access of ${usesText(uses)}. Give them other access, then delete it.` });
    return false;
  }
  const people = ws().members.filter((m) => m.groups.includes(group.id));
  const left = people.length ? ` ${people.length === 1 ? '1 member leaves it.' : `Its ${people.length} members leave it.`}` : '';
  if (!(await confirmDialog({ title: `Delete ${group.name}?`, body: `No rows use it.${left}`, confirm: 'Delete group' }))) return false;
  return deleteGroup(group);
}

export async function renameGroupPrompt(group) {
  const name = await prompt({ title: 'Rename group', label: 'Name', value: group.name });
  if (name && name !== group.name) updateGroup(group, { name });
}

export async function newGroupPrompt(people = []) {
  const name = await prompt({ title: 'New group', label: 'Name', placeholder: 'finance', confirm: 'Create' });
  return name ? createGroup({ name, people }) : null;
}

// memberGroupItems are the groups a member is not in yet, for an owner to add them to, and a new one to make with
// them in it.
export function memberGroupItems(member) {
  if (!isArchiveOwner()) return [];
  return [
    ...ws().groups.filter((g) => !member.groups.includes(g.id)).map((g) => ({ label: g.name, onselect: () => toggleGroup(member, g.id) })),
    { label: 'New group…', icon: Plus, onselect: () => newGroupPrompt([member.id]) }
  ];
}

export function shareItems(share, item) {
  return [
    { label: 'Manage sharing', icon: Users, onselect: () => openShare(share.kind, share.target) },
    item?.path && { label: 'Open', icon: ExternalLink, onselect: () => go(item.path) },
    SEPARATOR,
    { label: 'Stop sharing', icon: Ban, danger: true, onselect: () => stopSharing(share.id) }
  ];
}

async function deleteArchiveConfirm(archive) {
  if (await confirmDialog({ title: 'Delete this archive?', body: `“${archive.name}” and everything in it will be removed.`, confirm: 'Delete' })) deleteArchive(archive.id);
}

export function archiveItems(archive) {
  const current = archive.id === app.archive;
  return [
    !current && { label: 'Switch to this archive', icon: ArrowRightLeft, onselect: () => switchArchive(archive.id) },
    {
      label: 'Archive settings',
      icon: Settings,
      onselect: () => {
        switchArchive(archive.id);
        openSettings('general');
      }
    },
    SEPARATOR,
    { label: 'Delete archive', icon: Trash2, danger: true, disabled: app.archives.length < 2, onselect: () => deleteArchiveConfirm(archive) }
  ];
}

function messageText(message) {
  if (message.text) return message.text;
  return (message.blocks ?? []).map((b) => b.text ?? (b.items ?? []).map((i) => `- ${i}`).join('\n')).filter(Boolean).join('\n\n');
}

export function messageItems(message) {
  const text = messageText(message);
  return [
    text && { label: 'Copy text', icon: Copy, onselect: () => copyText(text) }
  ];
}
