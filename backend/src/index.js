'use strict';

const fastify = require('fastify')({ logger: true });
const path = require('path');
const fs = require('fs');
const { registerDevice, getAllDevices } = require('./db');

const PORT = parseInt(process.env.PORT || '3000', 10);
const HOST = process.env.HOST || '0.0.0.0';

const IP_RE = /^(\d{1,3}\.){3}\d{1,3}$/;

fastify.get('/health', async () => {
  return { status: 'ok', uptime: Math.floor(process.uptime()) };
});

fastify.post('/register', async (req, reply) => {
  const { local_ip } = req.body || {};
  if (!local_ip || !IP_RE.test(local_ip)) {
    return reply.code(400).send({ error: 'local_ip is required and must be a valid IPv4 address' });
  }
  return registerDevice(local_ip);
});

fastify.get('/api/devices', async () => {
  return getAllDevices();
});

fastify.get('/', async (req, reply) => {
  const html = fs.readFileSync(path.join(__dirname, 'public', 'index.html'), 'utf8');
  return reply.type('text/html').send(html);
});

fastify.listen({ port: PORT, host: HOST }, (err) => {
  if (err) {
    fastify.log.error(err);
    process.exit(1);
  }
});
