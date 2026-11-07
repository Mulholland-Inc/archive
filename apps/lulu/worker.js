const upstream = /^\/(api|auth)(\/|$)|^\/live$/;

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (!upstream.test(url.pathname)) {
      const asset = await env.ASSETS.fetch(request);
      const missing = url.pathname.startsWith('/assets/') && (asset.headers.get('Content-Type') || '').startsWith('text/html');
      return missing ? new Response('not found', { status: 404 }) : asset;
    }
    const origin = (env.API_ORIGIN || `https://api.${url.hostname.split('.').slice(1).join('.')}`).replace(/\/$/, '');
    return fetch(new Request(origin + url.pathname + url.search, new Request(request, { redirect: 'manual' })));
  }
};
