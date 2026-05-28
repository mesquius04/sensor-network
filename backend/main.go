package main

import (
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "modernc.org/sqlite"
)

// ── embed public/index.html into the binary ──────────────────────────────────
//
//go:embed public/index.html
var indexHTML []byte

var (
	db          *sql.DB
	mqttClient  mqtt.Client
	topicPrefix string
	startTime   = time.Now()
)

func main() {
	port := getenv("PORT", "3000")
	host := getenv("HOST", "0.0.0.0")
	dbPath := getenv("DB_PATH", "./central.db")
	topicPrefix = strings.TrimRight(getenv("MQTT_TOPIC_PREFIX", "house"), "/")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := initSchema(); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	// MQTT ingest + command publish; HTTP serves reads/UI below.
	mqttClient = connectMQTT()
	if mqttClient != nil {
		defer mqttClient.Disconnect(250)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/devices", handleDevices)
	mux.HandleFunc("GET /api/readings", handleReadings)
	mux.HandleFunc("POST /api/command", handleCommand)
	mux.HandleFunc("GET /", handleIndex)

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

// ── schema ──────────────────────────────────────────────────────────────────

func initSchema() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS devices (
			device_id  INTEGER PRIMARY KEY AUTOINCREMENT,
			node_name  TEXT    NOT NULL UNIQUE,
			first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen  DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS readings (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id   INTEGER NOT NULL REFERENCES devices(device_id),
			metric      TEXT    NOT NULL,
			value       REAL    NOT NULL,
			recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_readings_device_time
			ON readings(device_id, recorded_at);
	`)
	return err
}

// ── shared data access (used by both HTTP and MQTT paths) ─────────────────────

type Device struct {
	DeviceID  int64  `json:"device_id"`
	NodeName  string `json:"node_name"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

// upsertDevice inserts the node if new, otherwise refreshes last_seen, and
// returns the resulting row.
func upsertDevice(nodeName string) (Device, error) {
	var d Device
	if nodeName == "" || strings.ContainsAny(nodeName, "/+#") {
		return d, fmt.Errorf("node_name must be a single non-empty topic segment")
	}

	_, err := db.Exec(`
		INSERT INTO devices (node_name) VALUES (?)
		ON CONFLICT(node_name) DO UPDATE SET last_seen = CURRENT_TIMESTAMP`,
		nodeName)
	if err != nil {
		return d, err
	}

	err = db.QueryRow(`
		SELECT device_id, node_name, first_seen, last_seen
		FROM devices WHERE node_name = ?`, nodeName).
		Scan(&d.DeviceID, &d.NodeName, &d.FirstSeen, &d.LastSeen)
	return d, err
}

// insertReadings resolves (or creates) the device for nodeName, then stores one
// row per metric and bumps last_seen.
func insertReadings(nodeName string, metrics map[string]float64) error {
	d, err := upsertDevice(nodeName)
	if err != nil {
		return err
	}
	stmt, err := db.Prepare(`INSERT INTO readings (device_id, metric, value) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for metric, value := range metrics {
		if _, err := stmt.Exec(d.DeviceID, metric, value); err != nil {
			return err
		}
	}
	return nil
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"uptime": int64(time.Since(startTime).Seconds()),
	})
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT device_id, node_name, first_seen, last_seen
		FROM devices ORDER BY device_id ASC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg("db error"))
		return
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.DeviceID, &d.NodeName, &d.FirstSeen, &d.LastSeen); err != nil {
			continue
		}
		devices = append(devices, d)
	}
	writeJSON(w, http.StatusOK, devices)
}

type Reading struct {
	DeviceID   int64   `json:"device_id"`
	NodeName   string  `json:"node_name"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	RecordedAt string  `json:"recorded_at"`
}

func handleReadings(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	query := `
		SELECT r.device_id, d.node_name, r.metric, r.value, r.recorded_at
		FROM readings r JOIN devices d ON d.device_id = r.device_id`
	args := []any{}
	if node := r.URL.Query().Get("node"); node != "" {
		query += ` WHERE d.node_name = ?`
		args = append(args, node)
	}
	query += ` ORDER BY r.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg("db error"))
		return
	}
	defer rows.Close()

	readings := []Reading{}
	for rows.Next() {
		var rd Reading
		if err := rows.Scan(&rd.DeviceID, &rd.NodeName, &rd.Metric, &rd.Value, &rd.RecordedAt); err != nil {
			continue
		}
		readings = append(readings, rd)
	}
	writeJSON(w, http.StatusOK, readings)
}

// handleCommand publishes a literal command string to <prefix>/<node>/central.
// The ESP32 firmware reads the raw payload (e.g. "ENCENDER", "APAGAR").
func handleCommand(w http.ResponseWriter, r *http.Request) {
	log.Printf("cmd: POST /api/command from %s", r.RemoteAddr)

	var body struct {
		Node    string `json:"node"`
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("cmd: decode failed: %v", err)
		writeJSON(w, http.StatusBadRequest, errMsg("invalid JSON"))
		return
	}
	log.Printf("cmd: parsed body node=%q command=%q", body.Node, body.Command)

	if body.Node == "" || strings.ContainsAny(body.Node, "/+#") {
		log.Printf("cmd: rejected, invalid node %q", body.Node)
		writeJSON(w, http.StatusBadRequest, errMsg("node must be a single topic segment"))
		return
	}
	if body.Command == "" {
		log.Printf("cmd: rejected, empty command")
		writeJSON(w, http.StatusBadRequest, errMsg("command is required"))
		return
	}
	if mqttClient == nil || !mqttClient.IsConnected() {
		log.Printf("cmd: rejected, mqtt not connected (client=%v connected=%v)", mqttClient != nil, mqttClient != nil && mqttClient.IsConnected())
		writeJSON(w, http.StatusServiceUnavailable, errMsg("mqtt broker not connected"))
		return
	}

	topic := fmt.Sprintf("%s/%s/central", topicPrefix, body.Node)
	payload := body.Command // raw string bytes, NOT wrapped JSON
	log.Printf("cmd: publishing topic=%s qos=1 retained=false payload=%q (%d bytes)", topic, payload, len(payload))

	start := time.Now()
	t := mqttClient.Publish(topic, 1, false, payload)
	completed := t.WaitTimeout(3 * time.Second)
	elapsed := time.Since(start)

	if !completed {
		log.Printf("cmd: publish TIMEOUT after %s (token not done in 3s)", elapsed)
		writeJSON(w, http.StatusBadGateway, errMsg("publish timeout"))
		return
	}
	if err := t.Error(); err != nil {
		log.Printf("cmd: publish ERROR after %s: %v", elapsed, err)
		writeJSON(w, http.StatusBadGateway, errMsg("publish failed: "+err.Error()))
		return
	}
	log.Printf("cmd: publish OK in %s (topic=%s payload=%q)", elapsed, topic, payload)
	writeJSON(w, http.StatusOK, map[string]string{"topic": topic, "command": payload})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

// ── MQTT ingest ───────────────────────────────────────────────────────────────

// connectMQTT subscribes to <prefix>/+ (3-segment data topics). It returns nil
// (and logs) if the broker is unreachable, so the HTTP server still starts.
func connectMQTT() mqtt.Client {
	broker := getenv("MQTT_BROKER", "tcp://broker.hivemq.com:1883")
	clientID := getenv("MQTT_CLIENT_ID", "central-backend-"+randHex(4))

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		// Phone hotspots and CGNATs drop idle TCP after ~60s. Shorter
		// keepalive keeps the connection warm; longer ping timeout tolerates
		// slow round-trips over flaky uplinks.
		SetKeepAlive(15 * time.Second).
		SetPingTimeout(15 * time.Second).
		SetWriteTimeout(10 * time.Second).
		SetOnConnectHandler(onMQTTConnect).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			log.Printf("mqtt: connection lost: %v", err)
		})

	if user := os.Getenv("MQTT_USERNAME"); user != "" {
		opts.SetUsername(user).SetPassword(os.Getenv("MQTT_PASSWORD"))
	}

	client := mqtt.NewClient(opts)
	// With ConnectRetry the initial Connect won't block on a down broker; it
	// keeps retrying in the background and (re)subscribes via onMQTTConnect.
	client.Connect()
	log.Printf("mqtt: client started, broker=%s clientID=%s prefix=%s", broker, clientID, topicPrefix)
	return client
}

// onMQTTConnect (re)establishes the data subscription on every successful
// connect, so it survives reconnects. Topic shape: <prefix>/<node> (e.g.
// "house/room"). Command topics live one level deeper
// (<prefix>/<node>/central) and are explicitly excluded by the single-level
// wildcard.
func onMQTTConnect(client mqtt.Client) {
	dataTopic := topicPrefix + "/+"
	log.Printf("mqtt: connected, subscribing to %s", dataTopic)
	if t := client.Subscribe(dataTopic, 1, handleDataMessage); t.Wait() && t.Error() != nil {
		log.Printf("mqtt: subscribe %s failed: %v", dataTopic, t.Error())
	}
}

// topicNode returns the last segment of the topic, which by convention is the
// node identifier (e.g. "room", "salon", "cocina").
func topicNode(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// handleDataMessage parses the ESP32 JSON payload (numeric fields only) and
// stores one reading per field. Non-numeric fields are ignored so the firmware
// can add string fields later without breaking ingest.
func handleDataMessage(_ mqtt.Client, msg mqtt.Message) {
	node := topicNode(msg.Topic())
	if node == "" {
		log.Printf("mqtt: empty node on topic %q (payload=%q)", msg.Topic(), msg.Payload())
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg.Payload(), &raw); err != nil {
		log.Printf("mqtt: bad payload on %s: %v (raw=%q)", msg.Topic(), err, msg.Payload())
		return
	}
	metrics := make(map[string]float64, len(raw))
	for k, v := range raw {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			metrics[k] = f
		}
	}
	if len(metrics) == 0 {
		log.Printf("mqtt: no numeric metrics on %s (payload=%q)", msg.Topic(), msg.Payload())
		return
	}
	if err := insertReadings(node, metrics); err != nil {
		log.Printf("mqtt: store readings for %q failed: %v", node, err)
		return
	}
	log.Printf("mqtt: ingested node=%q metrics=%d topic=%s", node, len(metrics), msg.Topic())
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

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a timestamp; uniqueness is best-effort, not security.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
