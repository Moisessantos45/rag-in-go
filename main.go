package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No se encontro archivo .env, usando variables del entorno del sistema")
	}

	config := LoadConfig()

	ctx := context.Background()

	db, err := NewDatabase(ctx, config.DatabaseURL)
	if err != nil {
		log.Fatalf("Error conectando a PostgreSQL: %v", err)
	}
	defer db.Close()

	if err := InitSchema(ctx, db); err != nil {
		log.Fatalf("Error creando esquema: %v", err)
	}
	log.Println("Esquema de base de datos listo")

	ollama := NewOllamaClient(config.OllamaBaseURL, config.OllamaEmbedModel, config.OllamaGenModel)
	log.Printf("Ollama configurado: Embeddings=%s | Generacion=%s", config.OllamaEmbedModel, config.OllamaGenModel)

	uploadsDir := "./uploads"
	os.MkdirAll(uploadsDir, 0755)

	repo := NewPostgresDocumentRepository(db.Pool)
	usecase := NewRAGUsecase(repo, ollama, config)
	handler := NewHandler(usecase, uploadsDir)
	router := SetupRouter(handler)

	addr := fmt.Sprintf(":%s", config.Port)
	if strings.TrimSpace(os.Getenv("GO_ENV")) == "prod" {
		addr = fmt.Sprintf("127.0.0.1:%s", config.Port)
	}

	log.Printf("Servidor RAG iniciado en %s (Workers: %d, ChunkSize: %d, Overlap: %d)",
		addr, config.ConcurrencyWorkers, config.ChunkSize, config.ChunkOverlap)

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
