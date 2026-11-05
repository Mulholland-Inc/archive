const MOST = 10 << 20;

export default {
  async email(message, env) {
    if (message.rawSize > MOST) return message.setReject('Message too large');
    const res = await fetch(`${env.API}/email`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${env.KEY}`, 'X-To': message.to, 'X-From': message.from },
      body: await new Response(message.raw).arrayBuffer()
    });
    if (res.status === 404) return message.setReject('No such address');
    if (res.status === 413) return message.setReject('Message too large');
    if (res.status === 400) return message.setReject('Not an email');
    if (!res.ok) throw new Error(`API answered ${res.status}`);
  }
};
