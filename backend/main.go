package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"blockemulator-web/backend/internal/emulator"
)

type apiResponse struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	defaultRoot := firstExistingPath(
		filepath.Join(cwd, "..", "block-emulator-x"),
		filepath.Join(cwd, "..", "..", "block-emulator-x"),
	)
	if envRoot := os.Getenv("BLOCK_EMULATOR_X_ROOT"); envRoot != "" {
		defaultRoot = envRoot
	}

	service, err := emulator.NewService(defaultRoot, filepath.Join(cwd, "workdir"))
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", withCORS(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: service.LoadConfig()})
		case http.MethodPost:
			var cfg emulator.WebConfig
			if err := decodeJSON(r, &cfg); err != nil {
				writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error()})
				return
			}
			if err := service.SaveConfig(cfg); err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: cfg})
		default:
			methodNotAllowed(w)
		}
	}))

	mux.HandleFunc("/api/config/validate", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var cfg emulator.WebConfig
		if err := decodeJSON(r, &cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error()})
			return
		}
		if err := emulator.ValidateConfig(cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: map[string]string{"message": "valid"}})
	}))

	mux.HandleFunc("/api/ip-table", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var cfg emulator.WebConfig
		if err := decodeJSON(r, &cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error()})
			return
		}
		table, err := service.SaveIPTable(cfg.System.ShardNum, cfg.System.NodeNum)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: table})
	}))

	mux.HandleFunc("/api/config/saved", withCORS(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configs, err := service.ListSavedConfigs()
			if err != nil {
				writeError(w, err)
				return
			}
			if configs == nil {
				configs = []emulator.SavedConfigMeta{}
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: configs})
		case http.MethodPost:
			var body struct {
				Description string            `json:"description"`
				Config      emulator.WebConfig `json:"config"`
			}
			if err := decodeJSON(r, &body); err != nil {
				writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error()})
				return
			}
			meta, err := service.SaveConfigNamed(body.Description, body.Config)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: meta})
		default:
			methodNotAllowed(w)
		}
	}))

	mux.HandleFunc("/api/config/saved/{id}", withCORS(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: "missing id"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			cfg, err := service.LoadSavedConfig(id)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: cfg})
		case http.MethodDelete:
			if err := service.DeleteSavedConfig(id); err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: map[string]string{"message": "deleted"}})
		default:
			methodNotAllowed(w)
		}
	}))

	mux.HandleFunc("/api/upload/tx-source", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: "failed to parse upload: " + err.Error()})
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: "missing file: " + err.Error()})
			return
		}
		defer file.Close()
		destPath, err := service.SaveUploadedFile(file, header.Filename)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: map[string]string{"path": destPath}})
	}))

	mux.HandleFunc("/api/experiments/start", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var cfg emulator.WebConfig
		if err := decodeJSON(r, &cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: err.Error()})
			return
		}
		status, err := service.StartExperiment(cfg)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: status})
	}))

	mux.HandleFunc("/api/experiments/stop", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		status, err := service.StopExperiment("stopped by user")
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: status})
	}))

	mux.HandleFunc("/api/experiments/status", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: service.Status()})
	}))

	mux.HandleFunc("/api/experiments/logs/sources", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: service.LogSources()})
	}))

	mux.HandleFunc("/api/experiments/logs", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		source := r.URL.Query().Get("source")
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: service.Logs(source)})
	}))

	mux.HandleFunc("/api/results", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		results, err := service.Results()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: results})
	}))

	mux.HandleFunc("/api/results/download/", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/results/download/")
		path, err := service.ResultFile(name)
		if err != nil {
			writeError(w, err)
			return
		}
		http.ServeFile(w, r, path)
	}))

	addr := ":8080"
	if envAddr := os.Getenv("BLOCK_EMULATOR_WEB_ADDR"); envAddr != "" {
		addr = envAddr
	}

	log.Printf("BlockEmulator Web backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func firstExistingPath(candidates ...string) string {
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		if _, err := os.Stat(filepath.Join(clean, "config.yaml")); err == nil {
			return clean
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return filepath.Clean(candidates[0])
}

func decodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, emulator.ErrValidation) || errors.Is(err, emulator.ErrNotFound) || errors.Is(err, emulator.ErrRunning) {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, apiResponse{OK: false, Error: err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Error: "method not allowed"})
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}
