package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"breast-cancer-side-effect-agent/internal/ai"
	"breast-cancer-side-effect-agent/internal/httpapi"
	"breast-cancer-side-effect-agent/internal/rules"
	"breast-cancer-side-effect-agent/internal/store"
)

func main() {
	port := getenv("PORT", "8080")
	storePath := getenv("STORE_PATH", filepath.Join("data", "store.json"))
	staticDir := getenv("STATIC_DIR", filepath.Join("internal", "httpapi", "static"))

	fileStore, err := store.NewFileStore(storePath)
	if err != nil {
		log.Fatalf("create store: %v", err)
	}
	analyzer := ai.NewAnalyzerFromEnv()
	server := httpapi.NewServer(fileStore, analyzer, rules.NewEngine(), staticDir)

	addr := ":" + port
	log.Printf("breast cancer side-effect agent listening on http://localhost%s", addr)
	log.Printf("rule_version=%s ai_enabled=%v store=%s", rules.Version, analyzer.Enabled(), storePath)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
