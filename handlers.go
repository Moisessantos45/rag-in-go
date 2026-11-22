package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	usecase DocumentUsecase
	uploads string
}

func NewHandler(usecase DocumentUsecase, uploadsDir string) *Handler {
	return &Handler{
		usecase: usecase,
		uploads: uploadsDir,
	}
}

func (h *Handler) IndexView(c *gin.Context) {
	docs, _ := h.usecase.ListDocuments(c.Request.Context())
	c.HTML(http.StatusOK, "index.html", gin.H{
		"documents": docs,
	})
}

func (h *Handler) UploadDocument(c *gin.Context) {
	file, err := c.FormFile("document")
	if err != nil {
		c.JSON(400, gin.H{"error": "No se encontro el archivo"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := []string{".txt", ".pdf", ".md", ".docx"}
	if !slices.Contains(allowed, ext) {
		c.JSON(400, gin.H{"error": "Extension no soportada", "allowed": allowed})
		return
	}

	os.MkdirAll(h.uploads, 0755)
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)
	path := filepath.Join(h.uploads, filename)

	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(500, gin.H{"error": "Error guardando archivo"})
		return
	}

	docName := strings.TrimSpace(c.PostForm("name"))
	if docName == "" {
		docName = strings.TrimSpace(c.PostForm("doc_name"))
	}
	if docName == "" {
		docName = strings.TrimSpace(c.PostForm("title"))
	}
	if docName == "" {
		docName = file.Filename
	}

	ctx := context.Background()
	doc, totalChunks, err := h.usecase.UploadAndIndex(ctx, file.Filename, docName, path)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message":      "Documento recibido correctamente. Indexacion iniciada en segundo plano.",
		"document_id":  doc.ID.String(),
		"name":         doc.Filename,
		"filename":     file.Filename,
		"chunks_total": totalChunks,
		"status":       "indexing",
	})
}

func (h *Handler) ListDocuments(c *gin.Context) {
	ctx := context.Background()
	docs, err := h.usecase.ListDocuments(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error consultando documentos"})
		return
	}

	c.JSON(200, gin.H{"documents": docs})
}

func (h *Handler) Search(c *gin.Context) {
	var input SearchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Query requerido"})
		return
	}

	ctx := context.Background()
	results, err := h.usecase.Search(ctx, input)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"query":    input.Query,
		"document": input.Document,
		"results":  results,
	})
}

func (h *Handler) Query(c *gin.Context) {
	var input QueryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Query requerido"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	if input.Stream && input.Generate {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")

		flusher, ok := c.Writer.(http.Flusher)

		err := h.usecase.QueryStream(ctx, input,
			func(meta QueryOutput) error {
				if meta.Answer != "" {
					errPayload, _ := json.Marshal(gin.H{
						"answer":         meta.Answer,
						"max_similarity": meta.MaxSimilarity,
					})
					fmt.Fprintf(c.Writer, "event: done\ndata: %s\n\n", string(errPayload))
				} else {
					metaPayload, _ := json.Marshal(meta)
					fmt.Fprintf(c.Writer, "event: meta\ndata: %s\n\n", string(metaPayload))
				}
				if ok {
					flusher.Flush()
				}
				return nil
			},
			func(token string) error {
				tokenPayload, _ := json.Marshal(gin.H{"token": token})
				fmt.Fprintf(c.Writer, "event: token\ndata: %s\n\n", string(tokenPayload))
				if ok {
					flusher.Flush()
				}
				return nil
			},
		)

		if err != nil {
			errPayload, _ := json.Marshal(gin.H{"error": err.Error()})
			fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", string(errPayload))
			if ok {
				flusher.Flush()
			}
			return
		}

		fmt.Fprintf(c.Writer, "event: done\ndata: [DONE]\n\n")
		if ok {
			flusher.Flush()
		}
		return
	}

	output, err := h.usecase.Query(ctx, input)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, output)
}
