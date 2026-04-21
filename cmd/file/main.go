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
)

type FileService struct {
	store  *internal.FileStore
	secret string
}

// Нужен хендлер для Upload

// func (fs *FileStore) Upload(ctx context.Context, name string, reader io.Reader, size int) (string, error)

func (fs *FileService) UploadHandler(w http.ResponseWriter, r *http.Request) {

	token := r.Header.Get("Authorization")
	_, err := internal.ParseToken(token, fs.secret)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 1. Ограничиваем объем памяти для парсинга (например, 10 МБ).
	// Если файл больше, он будет временно сохранен на диск.
	r.ParseMultipartForm(10 << 20)

	// 2. Получаем файл по ключу (в данном примере "myFile")
	file, handler, err := r.FormFile("myFile")
	if err != nil {
		http.Error(w, "Ошибка получения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fmt.Printf("Загружен файл: %s, Размер: %d байт\n", handler.Filename, handler.Size)

	var url string

	url, err = fs.store.Upload(context.Background(), handler.Filename, file, handler.Size)
	if err != nil {
		http.Error(w, "Ошибка сохранения", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"url":    url,
	})

}

func main() {
	port := flag.String("port", "8083", "port")
	fmt.Println("File service...")

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// db, err := internal.NewDatabase(cfg)

	if err != nil {
		panic(err)
	}

	fileStore, err := internal.NewFileStore(context.Background(), cfg)
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()

	fileSvc := &FileService{store: fileStore, secret: cfg.Secret}

	mux.HandleFunc("POST /upload", fileSvc.UploadHandler)

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
