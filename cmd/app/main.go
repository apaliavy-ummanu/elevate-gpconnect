package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	gpConnectClient "github.com/Cleo-Systems/elevate-gpconnect/client/http"
	"github.com/Cleo-Systems/elevate-gpconnect/internal/service/common"
)

// --- simple in-memory idempotency cache (process lifetime only) ---
type idemEntry struct {
	BodyHash     [32]byte
	MessageID    string
	ResponseBody []byte
}

type idempotencyStore struct {
	m sync.Map // key:string => idemEntry
}

func (s *idempotencyStore) Get(key string) (idemEntry, bool) {
	v, ok := s.m.Load(key)
	if !ok {
		return idemEntry{}, false
	}
	return v.(idemEntry), true
}
func (s *idempotencyStore) Put(key string, e idemEntry) {
	s.m.Store(key, e)
}

var (
	idem = &idempotencyStore{}
)

func main() {
	// Config you’d normally load from env/secret manager
	cfg := common.Config{
		SenderMeshMailbox:                 getenv("SENDER_MESH_MAILBOX_ID", "SENDER_MESH_MAILBOX_ID"),
		DefaultSenderODS:                  getenv("DEFAULT_SENDER_ODS", "A(*)"),
		DefaultBusinessAckRequested:       true,
		DefaultInfrastructureAckRequested: true,
		DefaultRecipientType:              "FI",
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/update-record/messages", postOnly(withJSON(submitHandler(cfg))))

	srv := &http.Server{
		Addr:              getenv("PORT", ":8084"),
		Handler:           logMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// graceful shutdown
	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// wait for ctrl-c / SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("server stopped")
}

func submitHandler(cfg common.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := r.Header.Get("X-Correlation-ID")
		if corrID == "" {
			corrID = uuid.New().String()
		}
		w.Header().Set("X-Correlation-ID", corrID)

		// read body (limit to something sane, e.g. 5MB)
		r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("read body: %v", err))
			return
		}

		// idempotency
		idemKey := r.Header.Get("Idempotency-Key")
		bodyHash := sha256.Sum256(body)
		if idemKey != "" {
			if prev, ok := idem.Get(idemKey); ok {
				if prev.BodyHash != bodyHash {
					writeErr(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "same Idempotency-Key used with a different body")
					return
				}
				// return previous response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write(prev.ResponseBody)
				return
			}
		}

		// decode request
		var req gpConnectClient.UpdateRecordRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("invalid JSON: %v", err))
			return
		}

		// build FHIR message using your existing builder
		fhirBytes, err := common.BuildUpdateRecordFHIRXML(req, cfg)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "FHIR_VALIDATION_FAILED", err.Error())
			return
		}

		// Here you’d enqueue/send to MESH. We’ll just pretend it’s accepted.
		messageID := uuid.New().String()

		resp := map[string]any{
			"messageId":     messageID,
			"status":        "accepted",
			"meshMessageId": nil, // filled once enqueued to MESH
			"links": map[string]string{
				"self":   fmt.Sprintf("/v1/update-record/messages/%s", messageID),
				"status": fmt.Sprintf("/v1/update-record/messages/%s/status", messageID),
			},
		}

		respBytes, _ := json.Marshal(resp)

		// store idempotent result
		if idemKey != "" {
			idem.Put(idemKey, idemEntry{
				BodyHash:     bodyHash,
				MessageID:    messageID,
				ResponseBody: respBytes,
			})
		}

		// (Optional) log the built FHIR message (redact in real life)
		//log.Printf("built FHIR message for %s (bytes=%d)", messageID, len(fhirBytes))
		//log.Printf(string(fhirBytes))

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(fhirBytes)
	})
}

// --- helpers ---

func postOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func withJSON(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
	})
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
