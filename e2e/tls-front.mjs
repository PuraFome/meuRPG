// An https front for the local stack, for the mobile smoke: WebKit on Linux keeps no Secure cookie over http, and the app's cookies are all Secure.
// It forwards every request to the stack over http, streams both ways, and leaves Host and Origin as the browser sent them.
// It drops the HSTS header the app adds to https requests.
// Usage: E2E_TLS_KEY=key.pem E2E_TLS_CERT=cert.pem node tls-front.mjs   (E2E_TLS_PORT, default 8443; E2E_TLS_TARGET, default http://localhost:8080)
import { readFileSync } from 'node:fs';
import http from 'node:http';
import https from 'node:https';

const port = Number(process.env.E2E_TLS_PORT ?? 8443);
const target = new URL(process.env.E2E_TLS_TARGET ?? 'http://localhost:8080');
const key = readFileSync(process.env.E2E_TLS_KEY ?? 'tls-key.pem');
const cert = readFileSync(process.env.E2E_TLS_CERT ?? 'tls-cert.pem');

const server = https.createServer({ key, cert }, (req, res) => {
  const upstream = http.request(
    {
      hostname: target.hostname,
      port: target.port,
      method: req.method,
      path: req.url,
      headers: { ...req.headers, 'x-forwarded-proto': 'https' },
    },
    (answer) => {
      // The app's HSTS covers subdomains: WebKit would then open devidp (idp.localhost, http) over https.
      const { 'strict-transport-security': _hsts, ...headers } = answer.headers;
      res.writeHead(answer.statusCode ?? 502, answer.statusMessage, headers);
      res.flushHeaders();
      answer.pipe(res);
      answer.on('error', () => res.destroy());
    },
  );
  upstream.on('error', () => {
    if (res.headersSent) {
      res.destroy();
    } else {
      res.writeHead(502).end();
    }
  });
  // The browser went away (or the stream ended): the upstream request goes with it.
  res.on('close', () => upstream.destroy());
  req.on('error', () => upstream.destroy());
  req.pipe(upstream);
});
// A stream stays open for minutes: no idle limit.
server.requestTimeout = 0;
server.headersTimeout = 0;
server.timeout = 0;
server.keepAliveTimeout = 0;
server.listen(port, '127.0.0.1', () => console.log(`https://localhost:${port} -> ${target.origin}`));
