import { Pin, PinOff, Pencil, Trash2, ExternalLink, MessageSquarePlus, Share, Download, Eye, Play, Pause, Square, RotateCcw, Copy, UserPlus, UserMinus, Users, Ban, Settings, ArrowRightLeft } from '@lucide/svelte';
import { SEPARATOR } from './contextmenu.svelte.js';
import { app, togglePin, removeChat, renameChat, newChat, renameFile, removeFile, downloadFile, openShare, runWorkflow, pauseRun, stopRun, removeWorkflow, removeMember, invite, stopSharing, removeGuest, switchVault, openVaultSettings, deleteVault } from './app.svelte.js';
import { prompt, confirmDialog } from './dialogs.svelte.js';
import { toast } from './toast.svelte.js';
import { go } from './router.svelte.js';

export async function copyText(text, label = 'Copied') {
  try {
    await navigator.clipboard.writeText(text);
    toast(label);
  } catch {
    toast('Couldn’t copy', { tone: 'danger' });
  }
}

export async function renameChatPrompt(chat) {
  const next = await prompt({ title: 'Rename chat', label: 'Title', value: chat.title });
  if (next) renameChat(chat.id, next);
}

export async function deleteChat(chat) {
  if (await confirmDialog({ title: 'Delete this chat?', body: `“${chat.title}” will be removed.`, confirm: 'Delete' })) removeChat(chat.id);
}

export function chatItems(chat, { open = true, unpin = true, rename = () => renameChatPrompt(chat) } = {}) {
  return [
    open && { label: 'Open', icon: ExternalLink, onselect: () => go(`/chat/${chat.id}`) },
    (!chat.pinned || unpin) && { label: chat.pinned ? 'Unpin' : 'Pin', icon: chat.pinned ? PinOff : Pin, onselect: () => togglePin(chat.id) },
    rename && { label: 'Rename', icon: Pencil, onselect: rename },
    SEPARATOR,
    { label: 'Delete', icon: Trash2, danger: true, onselect: () => deleteChat(chat) }
  ];
}

export function askAboutFile(file) {
  go(`/chat/${newChat(`About ${file.name}`, { text: `What is in ${file.name}?`, attachments: [{ id: file.id, name: file.name, kind: file.kind, size: file.size, tone: file.tone }] })}`);
}

export async function renameFilePrompt(file) {
  const name = await prompt({ title: 'Rename file', label: 'Name', value: file.name });
  if (name) renameFile(file.id, name);
}

export function fileItems(file, { onopen } = {}) {
  return [
    onopen && { label: 'Open', icon: Eye, onselect: () => onopen(file) },
    { label: 'Ask about this', icon: MessageSquarePlus, onselect: () => askAboutFile(file) },
    SEPARATOR,
    { label: 'Rename', icon: Pencil, onselect: () => renameFilePrompt(file) },
    { label: 'Share', icon: Share, onselect: () => openShare('file', file.id) },
    { label: 'Download', icon: Download, onselect: () => downloadFile(file) },
    SEPARATOR,
    { label: 'Move to trash', icon: Trash2, danger: true, onselect: () => removeFile(file.id) }
  ];
}

export function askAboutTable(table) {
  go(`/chat/${newChat(`About ${table.name}`, { text: `What is in the ${table.name} table?` })}`);
}

export function tableItems(table) {
  return [
    { label: 'Open', icon: ExternalLink, onselect: () => go(`/database/${table.id}`) },
    { label: 'Ask about this', icon: MessageSquarePlus, onselect: () => askAboutTable(table) },
    SEPARATOR,
    { label: 'Share', icon: Share, onselect: () => openShare('table', table.id) },
    { label: 'Copy name', icon: Copy, onselect: () => copyText(table.id, 'Table name copied') }
  ];
}

export async function deleteWorkflow(workflow) {
  if (await confirmDialog({ title: 'Delete this workflow?', body: `“${workflow.name}” and its runs will be removed.`, confirm: 'Delete' })) removeWorkflow(workflow.id);
}

export function runItems(run, workflow, { onrun = () => runWorkflow(workflow.id) } = {}) {
  if (run && ['running', 'waiting'].includes(run.status))
    return [
      { label: run.paused ? 'Resume' : 'Pause', icon: run.paused ? Play : Pause, onselect: () => pauseRun(run) },
      { label: 'Stop', icon: Square, onselect: () => stopRun(run) }
    ];
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

export function memberItems(member, { roles = [], onrole } = {}) {
  const invited = member.status === 'invited';
  return [
    { heading: 'Role' },
    ...roles.map((r) => ({ label: r.label, checked: member.role === r.value, onselect: () => member.role !== r.value && onrole?.(member, r.value) })),
    SEPARATOR,
    { label: 'Copy email', icon: Copy, onselect: () => copyText(member.email, 'Email copied') },
    SEPARATOR,
    { label: invited ? 'Revoke invitation' : 'Remove from vault', icon: Trash2, danger: true, onselect: () => removeMember(member.id) }
  ];
}

export function guestItems(guest) {
  return [
    { label: 'Invite to vault', icon: UserPlus, onselect: () => invite({ emails: [guest.email], role: 'viewer' }) },
    { label: 'Copy email', icon: Copy, onselect: () => copyText(guest.email, 'Email copied') },
    SEPARATOR,
    { label: 'Remove all access', icon: UserMinus, danger: true, onselect: () => removeGuest(guest.email) }
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

export async function deleteVaultConfirm(vault) {
  if (await confirmDialog({ title: 'Delete this vault?', body: `“${vault.name}” and everything in it will be removed.`, confirm: 'Delete' })) deleteVault(vault.id);
}

export function vaultItems(vault) {
  const current = vault.id === app.vault;
  return [
    !current && { label: 'Switch to vault', icon: ArrowRightLeft, onselect: () => switchVault(vault.id) },
    {
      label: 'Vault settings',
      icon: Settings,
      onselect: () => {
        switchVault(vault.id);
        openVaultSettings();
      }
    },
    SEPARATOR,
    { label: 'Delete vault', icon: Trash2, danger: true, disabled: app.vaults.length < 2, onselect: () => deleteVaultConfirm(vault) }
  ];
}

export function messageText(message) {
  if (message.text) return message.text;
  return (message.blocks ?? []).map((b) => b.text ?? (b.items ?? []).map((i) => `- ${i}`).join('\n')).filter(Boolean).join('\n\n');
}

export function messageItems(message, { onedit } = {}) {
  const text = messageText(message);
  return [
    text && { label: 'Copy text', icon: Copy, onselect: () => copyText(text) },
    message.role === 'user' && onedit && { label: 'Edit', icon: Pencil, onselect: () => onedit(message.text) }
  ];
}
