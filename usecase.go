package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type RAGUsecase struct {
	repo               DocumentRepository
	ollama             *OllamaClient
	chunkSize          int
	chunkOverlap       int
	concurrencyWorkers int
}

func NewRAGUsecase(repo DocumentRepository, ollama *OllamaClient, cfg Config) *RAGUsecase {
	return &RAGUsecase{
		repo:               repo,
		ollama:             ollama,
		chunkSize:          cfg.ChunkSize,
		chunkOverlap:       cfg.ChunkOverlap,
		concurrencyWorkers: cfg.ConcurrencyWorkers,
	}
}

func (u *RAGUsecase) UploadAndIndex(ctx context.Context, originalFilename, customName, tempFilePath string) (*Document, int, error) {
	content, err := ExtractText(tempFilePath)
	if err != nil {
		os.Remove(tempFilePath)
		return nil, 0, fmt.Errorf("error procesando texto del archivo: %w", err)
	}

	docName := strings.TrimSpace(customName)
	if docName == "" {
		docName = originalFilename
	}

	docID := uuid.New()
	_ = u.repo.DeleteDocumentByName(ctx, docName)

	if err := u.repo.CreateDocument(ctx, docID, docName, content); err != nil {
		os.Remove(tempFilePath)
		return nil, 0, fmt.Errorf("error guardando documento: %w", err)
	}

	chunks := ChunkText(content, u.chunkSize, u.chunkOverlap)
	totalChunks := len(chunks)
	startTime := time.Now()

	log.Printf("Documento '%s' recibido: %d caracteres | %d chunks (tamano: %d, overlap: %d). Iniciando indexacion en segundo plano...",
		docName, len(content), totalChunks, u.chunkSize, u.chunkOverlap)

	go func(docUUID uuid.UUID, docFilename, tempPath string, docChunks []Chunk) {
		defer os.Remove(tempPath)
		bgCtx := context.Background()

		chunkChan := make(chan Chunk, len(docChunks))
		for _, chunk := range docChunks {
			chunkChan <- chunk
		}
		close(chunkChan)

		var wg sync.WaitGroup
		var processedCount int64
		var errorCount atomic.Int64
		numWorkers := min(u.concurrencyWorkers, len(docChunks))
		numWorkers = max(numWorkers, 1)

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for chunk := range chunkChan {
					embedding, err := u.ollama.GenerateEmbedding(bgCtx, chunk.Content)
					if err != nil {
						log.Printf("[Worker %d] Error en embedding chunk %d: %v", workerID, chunk.ChunkIdx, err)
						errorCount.Add(1)
						continue
					}

					if err := u.repo.InsertChunk(bgCtx, docUUID, chunk.Content, chunk.ChunkIdx, embedding); err != nil {
						log.Printf("[Worker %d] Error insertando chunk %d: %v", workerID, chunk.ChunkIdx, err)
						errorCount.Add(1)
						continue
					}

					current := atomic.AddInt64(&processedCount, 1)
					if current%25 == 0 || current == int64(len(docChunks)) {
						pct := float64(current) / float64(len(docChunks)) * 100
						log.Printf("Progreso [%s]: %d/%d chunks (%.1f%%)", docFilename, current, len(docChunks), pct)
					}
				}
			}(w + 1)
		}

		wg.Wait()
		duration := time.Since(startTime)
		log.Printf("Documento '%s' completado en %v (%d/%d chunks indexados)",
			docFilename, duration.Round(time.Millisecond), processedCount, len(docChunks))
	}(docID, docName, tempFilePath, chunks)

	doc := &Document{
		ID:        docID,
		Filename:  docName,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return doc, totalChunks, nil
}

func (u *RAGUsecase) ListDocuments(ctx context.Context) ([]DocInfo, error) {
	return u.repo.ListDocuments(ctx)
}

func cleanDocReferences(q string) string {
	patterns := []string{
		"del doc ", "del documento ", "en el doc ", "en el documento ",
		"del archivo ", "en el archivo ", "del libro ", "en el libro ",
		"segun el doc ", "segun el documento ", "segun el archivo ",
		"del doc", "del documento", "en el doc", "en el documento",
		"doc ", "documento ", "archivo ", "libro ",
	}
	res := q
	lower := strings.ToLower(res)
	for _, p := range patterns {
		if idx := strings.Index(lower, p); idx != -1 {
			res = res[:idx] + res[idx+len(p):]
			lower = strings.ToLower(res)
		}
	}
	return strings.TrimSpace(res)
}

func (u *RAGUsecase) resolveDocumentFilterAndQuery(ctx context.Context, explicitFilter, query string) (string, string) {
	if explicitFilter != "" {
		return explicitFilter, query
	}

	docs, err := u.repo.ListDocuments(ctx)
	if err != nil || len(docs) == 0 {
		return "", query
	}

	queryLower := strings.ToLower(query)
	var bestDoc string
	var bestScore int

	for _, d := range docs {
		fullName := strings.ToLower(d.Filename)
		baseName := strings.ToLower(strings.TrimSuffix(d.Filename, filepath.Ext(d.Filename)))

		if strings.Contains(queryLower, fullName) {
			return d.Filename, strings.ReplaceAll(query, d.Filename, "")
		}

		if len(baseName) >= 4 && strings.Contains(queryLower, baseName) {
			return d.Filename, strings.ReplaceAll(query, baseName, "")
		}

		cleanBase := strings.ReplaceAll(baseName, "_", " ")
		cleanBase = strings.ReplaceAll(cleanBase, "-", " ")
		cleanBase = strings.ReplaceAll(cleanBase, ".", " ")

		var acronymBuilder strings.Builder
		for w := range strings.FieldsSeq(cleanBase) {
			if len(w) > 0 {
				acronymBuilder.WriteByte(w[0])
			}
		}
		acronym := acronymBuilder.String()
		if len(acronym) >= 2 {
			for qw := range strings.FieldsSeq(queryLower) {
				cleanQw := strings.Trim(qw, ",.:;¿?¡!\"'()")
				if cleanQw == acronym {
					cleaned := strings.ReplaceAll(query, qw, "")
					return d.Filename, cleanDocReferences(cleaned)
				}
			}
		}

		score := 0
		for dw := range strings.FieldsSeq(cleanBase) {
			if len(dw) < 4 {
				continue
			}
			if strings.Contains(queryLower, dw) {
				score += len(dw)
			}
		}

		if score > bestScore && score >= 6 {
			bestScore = score
			bestDoc = d.Filename
		}
	}

	if bestDoc != "" {
		cleanQuery := cleanDocReferences(query)
		for w := range strings.FieldsSeq(strings.ReplaceAll(strings.ReplaceAll(bestDoc, "_", " "), "-", " ")) {
			if len(w) >= 4 {
				cleanQuery = strings.ReplaceAll(cleanQuery, w, "")
				cleanQuery = strings.ReplaceAll(cleanQuery, strings.ToLower(w), "")
			}
		}
		cleanQuery = strings.TrimSpace(cleanQuery)
		if len(cleanQuery) > 3 {
			return bestDoc, cleanQuery
		}
		return bestDoc, query
	}

	return "", query
}

func (u *RAGUsecase) Search(ctx context.Context, input SearchInput) ([]SearchResult, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}

	docFilter, searchQuery := u.resolveDocumentFilterAndQuery(ctx, input.Document, input.Query)
	embedding, err := u.ollama.GenerateEmbedding(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("error generando embedding: %w", err)
	}

	return u.repo.SearchChunks(ctx, embedding, docFilter, limit)
}

func (u *RAGUsecase) prepareContext(ctx context.Context, input QueryInput) ([]string, []string, float32, string, string, error) {
	topK := input.TopK
	if topK <= 0 {
		topK = 5
	}

	docFilter, searchQuery := u.resolveDocumentFilterAndQuery(ctx, input.Document, input.Query)
	embedding, err := u.ollama.GenerateEmbedding(ctx, searchQuery)
	if err != nil {
		return nil, nil, 0, "", "", fmt.Errorf("error generando embedding de la consulta: %w", err)
	}

	results, err := u.repo.SearchChunks(ctx, embedding, docFilter, topK)
	if err != nil {
		return nil, nil, 0, "", "", fmt.Errorf("error recuperando contexto de la base de datos: %w", err)
	}

	contexts := []string{}
	sources := []string{}
	var maxSimilarity float32

	for _, r := range results {
		if r.Similarity > maxSimilarity {
			maxSimilarity = r.Similarity
		}
		if input.MinSimilarity > 0 && r.Similarity < input.MinSimilarity {
			continue
		}
		trimmed := strings.TrimSpace(r.Content)
		if len(trimmed) < 40 {
			continue
		}
		contexts = append(contexts, trimmed)
		sources = append(sources, r.Filename)
	}

	var contextBuilder strings.Builder
	for i, ctxItem := range contexts {
		source := "Desconocido"
		if i < len(sources) {
			source = sources[i]
		}
		contextBuilder.WriteString(fmt.Sprintf("--- Fragmento %d [Fuente: %s] ---\n%s\n\n", i+1, source, ctxItem))
	}

	systemPrompt := `Eres un asistente de Inteligencia Artificial experto y preciso. Tu tarea es responder a la pregunta del usuario utilizando la informacion contenida en los fragmentos de contexto proporcionados.

REGLAS DE RESPUESTA:
1. Responde siempre en espanol con tono claro, estructurado y profesional.
2. Los fragmentos de contexto pueden estar en ingles (por ejemplo: isolation levels -> niveles de aislamiento, read committed -> lectura confirmada, snapshot isolation -> aislamiento de instantanea, repeat read -> lectura repetible, serializability -> serializabilidad, dirty reads -> lecturas sucias). Interpreta y traduce los conceptos tecnicos al espanol con precision basandote en ellos.
3. Cita las fuentes de forma concisa y natural [Fuente: nombre_documento].
4. Explica detalladamente los conceptos, mecanismos y ejemplos descritos en los fragmentos.
5. Unicamente si los fragmentos no guardan relacion tematica con la pregunta formulada, indica: "No encontre informacion relevante sobre este tema en mis documentos."`

	prompt := fmt.Sprintf("FRAGMENTOS DE CONTEXTO DISPONIBLES:\n%s\nPREGUNTA:\n%s\n\nINSTRUCCIONES:\nSintetiza la respuesta en espanol basandote estrictamente en los fragmentos anteriores de forma fluida y estructurada, citando las fuentes pertinentes sin redundancias.\n\nRESPUESTA:",
		contextBuilder.String(), input.Query)

	return contexts, sources, maxSimilarity, prompt, systemPrompt, nil
}

func (u *RAGUsecase) Query(ctx context.Context, input QueryInput) (*QueryOutput, error) {
	contexts, sources, maxSimilarity, prompt, systemPrompt, err := u.prepareContext(ctx, input)
	if err != nil {
		return nil, err
	}

	includeCtx := true
	if input.IncludeContexts != nil {
		includeCtx = *input.IncludeContexts
	}

	outContexts := contexts
	if !includeCtx {
		outContexts = []string{}
	}

	output := &QueryOutput{
		Query:         input.Query,
		Contexts:      outContexts,
		Sources:       sources,
		MaxSimilarity: maxSimilarity,
	}

	think := false
	if input.Think != nil {
		think = *input.Think
	}

	if input.Generate {
		if len(contexts) == 0 || (input.MinSimilarity > 0 && maxSimilarity < input.MinSimilarity) {
			output.Answer = "No encontre informacion relevante sobre este tema en mis documentos."
		} else {
			answer, err := u.ollama.GenerateCompletion(ctx, prompt, systemPrompt, think)
			if err != nil {
				log.Printf("Error generando respuesta con Ollama: %v", err)
				output.Answer = fmt.Sprintf("Error generando respuesta con el modelo: %v", err)
			} else {
				output.Answer = answer
			}
		}
	}

	return output, nil
}

func (u *RAGUsecase) QueryStream(ctx context.Context, input QueryInput, onMeta func(meta QueryOutput) error, onToken func(token string) error) error {
	contexts, sources, maxSimilarity, prompt, systemPrompt, err := u.prepareContext(ctx, input)
	if err != nil {
		return err
	}

	includeCtx := true
	if input.IncludeContexts != nil {
		includeCtx = *input.IncludeContexts
	}

	outContexts := contexts
	if !includeCtx {
		outContexts = []string{}
	}

	if len(contexts) == 0 || (input.MinSimilarity > 0 && maxSimilarity < input.MinSimilarity) {
		return onMeta(QueryOutput{
			Query:         input.Query,
			Contexts:      outContexts,
			Sources:       sources,
			MaxSimilarity: maxSimilarity,
			Answer:        "No encontre informacion relevante sobre este tema en mis documentos.",
		})
	}

	if err := onMeta(QueryOutput{
		Query:         input.Query,
		Contexts:      outContexts,
		Sources:       sources,
		MaxSimilarity: maxSimilarity,
	}); err != nil {
		return err
	}

	think := false
	if input.Think != nil {
		think = *input.Think
	}

	return u.ollama.GenerateStream(ctx, prompt, systemPrompt, think, onToken)
}
