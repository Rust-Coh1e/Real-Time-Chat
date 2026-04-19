package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"real-time-chat/config"
	"real-time-chat/internal"
	"real-time-chat/middleware"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	db     *internal.Database
	secret string
}

type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewAuthService(db *internal.Database, cfg *config.Config) *AuthService {
	return &AuthService{
		db:     db,
		secret: cfg.Secret,
	}
}

func (authS *AuthService) RegisterHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "Application/json")

	var req RegisterReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	id, status := authS.db.CreateUser(ctx, req.Username, req.Password)
	if status != nil {
		http.Error(w, "Invalid username", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	// json.NewEncoder(w).Encode(shortURL)
	// Save

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"id":     id.String(),
	})
}

func (authS *AuthService) LoginHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "Application/json")

	var req RegisterReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	id, hash, status := authS.db.GetUserByName(ctx, req.Username)
	if status != nil {
		http.Error(w, "Invlid username", http.StatusForbidden)
		return
	}

	status = bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password))
	if status != nil {
		http.Error(w, "invalid creds", http.StatusUnauthorized)
		return
	}

	token, status := internal.GenerateToken(id, req.Username, authS.secret)
	if status != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"token":  token,
	})
}

func main() {
	port := flag.String("port", "8082", "port")
	fmt.Println("Auth service...")

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	db, err := internal.NewDatabase(cfg)

	if err != nil {
		panic(err)
	}

	auth := NewAuthService(db, cfg)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /register", auth.RegisterHandler)
	mux.HandleFunc("POST /login", auth.LoginHandler)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	server := &http.Server{
		Addr:    ":" + *port,
		Handler: middleware.CorsMiddleware(mux),
	}

	// http.ListenAndServe(":8080", mux)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// Ждём сигнала
	<-quit
	fmt.Println("Shutting down...")

	// Graceful shutdown с таймаутом 5 секунд
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		panic(err)
	}
	// server.Shutdown(ctx)
	// mainRepo.Close()
}
