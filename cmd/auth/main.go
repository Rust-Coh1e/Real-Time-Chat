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
	"real-time-chat/internal/jwt"
	"real-time-chat/internal/model"
	"real-time-chat/internal/repository"
	"real-time-chat/internal/service"
	"real-time-chat/middleware"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	db      *repository.Postgres
	mail    *service.Mailer
	cfg     *config.Config
	baseURL string
}

func NewAuthService(db *repository.Postgres, cfg *config.Config) *AuthService {
	return &AuthService{
		db:  db,
		cfg: cfg,
	}
}

func (authS *AuthService) RegisterHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "Application/json")

	var req model.RegisterReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid JSON",
		})
		return
	}

	ctx := context.Background()

	id, token, status := authS.db.CreateUser(ctx, req.Username, req.Email, req.Password)
	if status != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "username already taken",
		})
		return
	}

	tokenURL := authS.baseURL + "/verify?token=" + token

	status = authS.mail.SendVerification(req.Email, req.Username, tokenURL)

	if status != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "SMTP error",
		})
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

	var req model.RegisterReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	id, hash, verified, status := authS.db.GetUserByName(ctx, req.Username)
	if status != nil {
		http.Error(w, "Invlid username", http.StatusForbidden)
		return
	}

	if verified == false {
		http.Error(w, "Email not verified", http.StatusForbidden)
		return
	}

	status = bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password))
	if status != nil {
		http.Error(w, "invalid creds", http.StatusUnauthorized)
		return
	}

	token, status := jwt.GenerateToken(id, req.Username, authS.cfg.Secret)
	if status != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"token":  token,
	})
}

func (authS *AuthService) VerifyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "Application/json")

	ctx := context.Background()

	verifyToken := r.URL.Query().Get("token")

	err := authS.db.VerifyToken(ctx, verifyToken)

	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusCreated)

}

func (authS *AuthService) AvatarHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "Application/json")
	ctx := context.Background()

	clientToken := r.Header.Get("Authorization")

	var req struct {
		AvatarURL string `json:"avatar_url"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	clientClaims, err := jwt.ParseToken(clientToken, authS.cfg.Secret)

	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	clientID := clientClaims.UserID

	err = authS.db.UpdateAvatar(ctx, clientID, req.AvatarURL)
	w.WriteHeader(http.StatusCreated)
}

func main() {
	port := flag.String("port", "8082", "port")
	fmt.Println("Auth service...")

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	db, err := repository.NewPostgres(cfg)

	if err != nil {
		panic(err)
	}

	auth := &AuthService{
		db:      db,
		mail:    service.NewMailer(*cfg),
		cfg:     cfg,
		baseURL: "http://localhost:" + *port,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /register", auth.RegisterHandler)
	mux.HandleFunc("POST /login", auth.LoginHandler)
	mux.HandleFunc("GET /verify", auth.VerifyHandler)
	mux.HandleFunc("POST /avatar", auth.AvatarHandler)

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
