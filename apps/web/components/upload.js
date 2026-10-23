import { ApiError, toSignIn } from './api.js';
import { endpoints } from './endpoints.js';

const version = '1.0.0';
const chunk = 8 << 20;
const retries = 3;

const b64 = (s) => btoa(String.fromCharCode(...new TextEncoder().encode(s)));

async function failure(res) {
  if (res.status === 401) return toSignIn();
  return new ApiError(res.status, await res.json().catch(() => null));
}

function patch(url, offset, blob, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PATCH', url);
    xhr.setRequestHeader('Tus-Resumable', version);
    xhr.setRequestHeader('Upload-Offset', String(offset));
    xhr.setRequestHeader('Content-Type', 'application/offset+octet-stream');
    xhr.upload.onprogress = (e) => onProgress(offset + e.loaded);
    xhr.onload = () =>
      xhr.status === 204
        ? resolve({ offset: Number(xhr.getResponseHeader('Upload-Offset')), id: xhr.getResponseHeader('Archive-File-Id') })
        : xhr.status === 401
          ? toSignIn()
          : reject(new ApiError(xhr.status, JSON.parse(xhr.responseText || 'null')));
    xhr.onerror = () => reject(new Error('The connection dropped.'));
    xhr.send(blob);
  });
}

export async function upload(file, { space, intake, onProgress = () => {} }) {
  const key = `archive.tus.${[file.name, file.size, file.lastModified, space, intake].join('/')}`;
  let url = localStorage.getItem(key);
  let offset = 0;
  if (url) {
    const res = await fetch(url, { method: 'HEAD', headers: { 'Tus-Resumable': version } });
    if (res.ok) offset = Number(res.headers.get('Upload-Offset'));
    else url = null;
  }
  if (!url) {
    const meta = Object.entries({ filename: file.name, space, intake })
      .filter(([, v]) => v)
      .map(([k, v]) => `${k} ${b64(v)}`)
      .join(',');
    const res = await fetch(endpoints.tus, {
      method: 'POST',
      headers: { 'Tus-Resumable': version, 'Upload-Length': String(file.size), 'Upload-Metadata': meta },
    });
    if (res.status !== 201) throw await failure(res);
    if (res.headers.get('Archive-File-Id')) return res.headers.get('Archive-File-Id');
    url = res.headers.get('Location');
    localStorage.setItem(key, url);
  }
  onProgress(offset / file.size);
  for (let attempt = 0; ; ) {
    try {
      const done = await patch(url, offset, file.slice(offset, offset + chunk), (n) => onProgress(n / file.size));
      offset = done.offset;
      attempt = 0;
      if (done.id) {
        localStorage.removeItem(key);
        return done.id;
      }
    } catch (err) {
      if (err instanceof ApiError || ++attempt > retries) throw err;
      await new Promise((r) => setTimeout(r, 1000 * attempt));
      const res = await fetch(url, { method: 'HEAD', headers: { 'Tus-Resumable': version } });
      if (!res.ok) throw await failure(res);
      offset = Number(res.headers.get('Upload-Offset'));
    }
  }
}
