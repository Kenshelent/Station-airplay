package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func panelPresets(path string) http.HandlerFunc {
	var mu sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == "GET" {
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				data = []byte("{}")
				err = nil
			}
			if err != nil || len(data) > 16384 || !json.Valid(data) {
				http.Error(w, "cannot read presets", 500)
				return
			}
			w.Write(data)
			return
		}
		if r.Method != "PUT" {
			w.WriteHeader(405)
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, 16385))
		var presets map[string]json.RawMessage
		if err != nil || len(data) > 16384 || json.Unmarshal(data, &presets) != nil || presets == nil || len(presets) > 16 {
			http.Error(w, "invalid presets", 400)
			return
		}
		for name, value := range presets {
			var object map[string]json.RawMessage
			if strings.TrimSpace(name) == "" || len(name) > 80 || json.Unmarshal(value, &object) != nil || object == nil {
				http.Error(w, "invalid preset", 400)
				return
			}
		}
		file, err := os.CreateTemp(filepath.Dir(path), ".panel-presets-")
		if err != nil {
			http.Error(w, "cannot save presets", 500)
			return
		}
		defer os.Remove(file.Name())
		_, err = file.Write(data)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(file.Name(), path)
		}
		if err != nil {
			http.Error(w, "cannot save presets", 500)
			return
		}
		w.Write([]byte(`{"saved":true}`))
	}
}
