package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// spaFileServer serves the built frontend from dir. Any path that isn't a
// real file (e.g. a client-side route like /reports) falls back to
// index.html, since routing is handled in the browser, not the server.
func spaFileServer(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		full := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(full); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func logRequest(method, path string) {
	log.Printf("%s %s", method, path)
}

func main() {
	store, categories, budgets, recurring, users, cleanupStorage, err := initStores()
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}
	defer cleanupStorage()

	if categories.IsEmptyFor(LegacyTenant) {
		for _, c := range defaultCategories() {
			if _, err := categories.Create(LegacyTenant, c); err != nil {
				log.Fatalf("failed to seed default categories: %v", err)
			}
		}
		log.Println("seeded default categories for the legacy PIN household")
	}

	auth, err := NewAuthStore("pin.json", "sessions.json")
	if err != nil {
		log.Fatalf("failed to initialize auth store: %v", err)
	}

	// Goals are deliberately kept as a local JSON file regardless of
	// DATABASE_URL, same reasoning as auth: small, not business data in
	// the same sense as transactions, and not worth the Postgres plumbing
	// yet. Revisit if that ever changes.
	goals, err := NewFileStore[*Goal]("goals.json")
	if err != nil {
		log.Fatalf("failed to initialize goals store: %v", err)
	}

	// Catch up on any recurring transactions due since the app last ran —
	// important since this isn't a server that's always on.
	GenerateDueTransactions(store, recurring)

	// Also re-check periodically while the app stays open, so a rule due
	// mid-session (e.g. you leave it running overnight) still fires without
	// needing a restart. Same ticker also sweeps out expired sessions so
	// sessions.json doesn't just grow forever on a long-running instance.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			GenerateDueTransactions(store, recurring)
			if err := auth.PruneExpiredSessions(); err != nil {
				log.Printf("failed to prune expired sessions: %v", err)
			}
		}
	}()

	api := NewAPI(store, categories, budgets, recurring, goals, auth, users)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/transactions", api.HandleTransactions)
	mux.HandleFunc("/api/transactions/", api.HandleTransactionByID)
	mux.HandleFunc("/api/reports", api.HandleReports)
	mux.HandleFunc("/api/categories", api.HandleCategories)
	mux.HandleFunc("/api/categories/", api.HandleCategoryByID)
	mux.HandleFunc("/api/budgets", api.HandleBudgets)
	mux.HandleFunc("/api/budgets/status", api.HandleBudgetStatus)
	mux.HandleFunc("/api/budgets/history", api.HandleBudgetHistory)
	mux.HandleFunc("/api/budgets/", api.HandleBudgetByID)
	mux.HandleFunc("/api/recurring", api.HandleRecurring)
	mux.HandleFunc("/api/recurring/", api.HandleRecurringByID)
	mux.HandleFunc("/api/goals", api.HandleGoals)
	mux.HandleFunc("/api/goals/", api.HandleGoalByID)
	mux.HandleFunc("/api/export/csv", api.HandleExportCSV)
	mux.HandleFunc("/api/export/xlsx", api.HandleExportXLSX)
	mux.HandleFunc("/api/export/html", api.HandleExportHTML)
	mux.HandleFunc("/api/import/csv", api.HandleImportCSV)
	mux.HandleFunc("/api/import/file", api.HandleImportFile)
	mux.HandleFunc("/api/auth/status", api.HandleAuthStatus)
	mux.HandleFunc("/api/auth/setup", api.HandleAuthSetup)
	mux.HandleFunc("/api/auth/login", api.HandleAuthLogin)
	mux.HandleFunc("/api/auth/register", api.HandleAuthRegister)
	mux.HandleFunc("/api/auth/account-login", api.HandleAuthAccountLogin)
	mux.HandleFunc("/api/auth/logout", api.HandleAuthLogout)
	mux.HandleFunc("/api/auth/session", api.HandleAuthSession)
	mux.HandleFunc("/api/auth/change-pin", api.HandleAuthChangePIN)
	mux.HandleFunc("/api/auth/enable-pin", api.HandleAuthEnablePIN)
	mux.HandleFunc("/api/auth/guest-pin", api.HandleAuthGuestPIN)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Serves the built frontend (npm run build) so the whole app can run
	// off this one port — handy for tunneling with cloudflared/ngrok.
	// Override with FRONTEND_DIST if you build/copy it somewhere else.
	// Registered last / on "/" so it never shadows the /api routes above.
	distDir := os.Getenv("FRONTEND_DIST")
	if distDir == "" {
		distDir = "../frontend/dist"
	}
	if _, err := os.Stat(distDir); err == nil {
		mux.Handle("/", spaFileServer(distDir))
		log.Printf("serving frontend from %s", distDir)
	} else {
		log.Printf("no frontend build found at %s — API only (run `npm run build` in frontend/ to serve the UI too)", distDir)
	}

	handler := withLogging(withCORS(withAuth(auth, users)(mux)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("expense tracker API listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
