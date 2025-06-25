package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"ratelimiter-app/pkg/service"
	"strings"
)

type Handler struct {
	service *service.Service
}

func NewHandler(s *service.Service) *Handler {
	return &Handler{service: s}
}

func (h *Handler) Acquire(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	claims, err := parseJWT(token)
	if err != nil {
		http.Error(w, "invalid JWT", http.StatusUnauthorized)
		return
	}
	// Use "sub" (subject) or another claim as the key
	key, ok := claims["sub"].(string)
	if !ok || key == "" {
		http.Error(w, "JWT missing sub claim", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	input := map[string]interface{}{"key": key}
	result := h.service.Acquire(ctx, input)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	claims, err := parseJWT(token)
	if err != nil {
		http.Error(w, "invalid JWT", http.StatusUnauthorized)
		return
	}
	key, ok := claims["sub"].(string)
	if !ok || key == "" {
		http.Error(w, "JWT missing sub claim", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	result := h.service.Status(ctx, key)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(h.service.Metrics()))
}

func (h *Handler) AdminUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
  <title>Rate Limiter Admin UI</title>
  <style>
    body { font-family: sans-serif; margin: 2em; }
    table { border-collapse: collapse; }
    th, td { border: 1px solid #ccc; padding: 0.5em; }
  </style>
</head>
<body>
  <h1>Rate Limiter Admin UI</h1>
  <label>User Key: <input id="key" value="user1"></label>
  <button onclick="fetchStatus()">Get Status</button>
  <pre id="status"></pre>
  <h2>Prometheus Metrics</h2>
  <pre id="metrics"></pre>
  <script>
    function fetchStatus() {
      const key = document.getElementById('key').value;
      fetch('/api/status', {
        headers: { 'Authorization': 'Bearer ' + localStorage.getItem('jwt') }
      })
      .then(r => r.json())
      .then(data => {
        document.getElementById('status').textContent = JSON.stringify(data, null, 2);
      });
    }
    function fetchMetrics() {
      fetch('/metrics')
      .then(r => r.text())
      .then(data => {
        document.getElementById('metrics').textContent = data;
      });
    }
    fetchMetrics();
    setInterval(fetchMetrics, 5000);
  </script>
</body>
</html>
`))
}

// RegisterRoutes sets up the HTTP handlers using only net/http
func (h *Handler) RegisterRoutes() {
	http.HandleFunc("/api/acquire", h.Acquire)
	http.HandleFunc("/api/status", h.Status)
	http.HandleFunc("/metrics", h.Metrics)
	http.HandleFunc("/admin", h.AdminUI) // <-- Add this line
}

func parseJWT(tokenString string) (map[string]interface{}, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	// NOTE: This does NOT verify the signature! For demo only.
	return claims, nil
}
