package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/engine"
	"github.com/apache261/Byakugan-core/iso20022"
)

type Options struct{ APIKeys []string }

func New(engine *engine.Engine, options Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("POST /v1/transfer-checks", authenticate(options.APIKeys, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request domain.TransferCheckRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decodeOne(decoder, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if request.IdempotencyKey == "" {
			request.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		}
		writeJSON(w, http.StatusCreated, engine.Check(request))
	})))
	mux.Handle("POST /v1/transfer-checks/iso20022", authenticate(options.APIKeys, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := iso20022.ParsePacs008(http.MaxBytesReader(w, r.Body, 2<<20), r.Header.Get("Idempotency-Key"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, engine.Check(request))
	})))
	return mux
}

func decodeOne(decoder *json.Decoder, destination any) error {
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple json values")
		}
		return err
	}
	return nil
}

func authenticate(keys []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(keys) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimSpace(r.Header.Get("X-API-Key"))
		for _, key := range keys {
			if subtle.ConstantTimeCompare([]byte(provided), []byte(key)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeError(w, http.StatusUnauthorized, "valid X-API-Key required")
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
