package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// ── embed public/index.html into the binary ──────────────────────────────────
//
//go:embed public/index.html
var indexHTML []byte

var (
	db        *sql.DB
	startTime = time.Now()
)

func main() {
	port   := getenv("PORT", "3000")
	host   := getenv("HOST", "0.0.0.0")
	dbPath := getenv("DB_PATH", "./central.db")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS devices (
		device_id     INTEGER PRIMARY KEY AUTOINCREMENT,
		local_ip      TEXT    NOT NULL UNIQUE,
		registered_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatalf("create table: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health",      handleHealth)
	mux.HandleFunc("POST /register",   handleRegister)
	mux.HandleFunc("GET /api/devices", handleDevices)
	mux.HandleFunc("GET /",            handleIndex)

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── handlers ──────────────────────────────────────────────────────────────────

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"uptime": int64(time.Since(startTime).Seconds()),
	})
}

type Device struct {
	DeviceID     int64  `json:"device_id"`
	LocalIP      string `json:"local_ip"`
	RegisteredAt string `json:"registered_at"`
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LocalIP string `json:"local_ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid JSON"))
		return
	}
	ip := net.ParseIP(body.LocalIP)
	if ip == nil || ip.To4() == nil {
		writeJSON(w, http.StatusBadRequest, errMsg("local_ip is required and must be a valid IPv4 address"))
		return
	}

	res, err := db.Exec(`INSERT OR IGNORE INTO devices (local_ip) VALUES (?)`, body.LocalIP)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg("db error"))
		return
	}

	var d Device
	rowID, _ := res.LastInsertId()
	var query string
	var arg any
	if rowID == 0 {
		query, arg = `SELECT device_id, local_ip, registered_at FROM devices WHERE local_ip = ?`, body.LocalIP
	} else {
		query, arg = `SELECT device_id, local_ip, registered_at FROM devices WHERE device_id = ?`, rowID
	}
	if err := db.QueryRow(query, arg).Scan(&d.DeviceID, &d.LocalIP, &d.RegisteredAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg("db error"))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT device_id, local_ip, registered_at FROM devices ORDER BY device_id ASC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg("db error"))
		return
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.DeviceID, &d.LocalIP, &d.RegisteredAt); err != nil {
			continue
		}
		devices = append(devices, d)
	}
	writeJSON(w, http.StatusOK, devices)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errMsg(msg string) map[string]string {
	return map[string]string{"error": msg}
}
