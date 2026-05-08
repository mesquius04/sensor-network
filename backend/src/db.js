'use strict';

const Database = require('better-sqlite3');
const path = require('path');

const dbPath = process.env.DB_PATH || path.join(__dirname, '..', 'central.db');
const db = new Database(dbPath);

db.exec(`
  CREATE TABLE IF NOT EXISTS devices (
    device_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    local_ip      TEXT    NOT NULL UNIQUE,
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP
  )
`);

const stmtFindByIp  = db.prepare('SELECT * FROM devices WHERE local_ip = ?');
const stmtInsert    = db.prepare('INSERT INTO devices (local_ip) VALUES (?)');
const stmtFindById  = db.prepare('SELECT * FROM devices WHERE device_id = ?');
const stmtAll       = db.prepare('SELECT * FROM devices ORDER BY device_id ASC');

function registerDevice(local_ip) {
  const existing = stmtFindByIp.get(local_ip);
  if (existing) return existing;
  const { lastInsertRowid } = stmtInsert.run(local_ip);
  return stmtFindById.get(lastInsertRowid);
}

function getAllDevices() {
  return stmtAll.all();
}

module.exports = { registerDevice, getAllDevices };
