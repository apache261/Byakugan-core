package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apache261/Byakugan-core/api"
	"github.com/apache261/Byakugan-core/engine"
	"github.com/apache261/Byakugan-core/rules"
)

func main() {
	ruleSet := engine.DefaultRules()
	if path := strings.TrimSpace(os.Getenv("BYAKUGAN_RULES_FILE")); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read rules: %v", err)
		}
		if err := json.Unmarshal(raw, &ruleSet); err != nil {
			log.Fatalf("decode rules: %v", err)
		}
	}
	for _, rule := range ruleSet {
		if err := rules.Validate(rule); err != nil {
			log.Fatalf("validate rule %q: %v", rule.ID, err)
		}
	}
	address := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if address == "" {
		address = ":8080"
	}
	maxIdempotencyEntries := engine.DefaultMaxIdempotencyEntries
	if value := strings.TrimSpace(os.Getenv("MAX_IDEMPOTENCY_ENTRIES")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			log.Fatal("MAX_IDEMPOTENCY_ENTRIES must be a positive integer")
		}
		maxIdempotencyEntries = parsed
	}
	decisionEngine := engine.NewWithOptions(ruleSet, engine.Options{MaxIdempotencyEntries: maxIdempotencyEntries})
	server := &http.Server{Addr: address, Handler: api.New(decisionEngine, api.Options{APIKeys: split(os.Getenv("API_KEYS"))}), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("byakugan-core listening on %s", address)
	log.Fatal(server.ListenAndServe())
}

func split(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
