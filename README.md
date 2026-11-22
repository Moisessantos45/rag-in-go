# RAG en Go - Microservicio Autónomo de Alto Rendimiento

Microservicio RAG (*Retrieval-Augmented Generation*) desarrollado en Go con arquitectura por capas, procesamiento concurrente en segundo plano, base de datos PostgreSQL con `pgvector`, modelos locales con Ollama y una interfaz web moderna e interactiva.

## 🎯 Propósito del proyecto

Este proyecto lo realicé para probar mis ideas y poner a prueba cómo construir un RAG desde cero. Su objetivo es experimentar con la ingesta y fragmentación de documentos, la generación de embeddings, la búsqueda semántica y la generación de respuestas con modelos locales, además de explorar una arquitectura práctica y escalable en Go.

---

## 🏗️ Arquitectura y Flujo del Sistema

El microservicio está estructurado bajo una arquitectura limpia y desacoplada dentro del paquete principal:

```
                  ┌──────────────────────────────────────────────┐
                  │                 INTERFAZ WEB                 │
                  │  (HTML5 Template + Tailwind + Marked + SSE)  │
                  └──────────────────────┬───────────────────────┘
                                         │ HTTP / SSE
                                         ▼
                  ┌──────────────────────────────────────────────┐
                  │                 HANDLER LAYER                │
                  │   - Validación HTTP / Multipart              │
                  │   - Streaming SSE con flusher.Flush()        │
                  │   - Serialización de eventos meta/token/done │
                  └──────────────────────┬───────────────────────┘
                                         │
                                         ▼
                  ┌──────────────────────────────────────────────┐
                  │                 USECASE LAYER                │
                  │   - Pipeline RAG y orquestación              │
                  │   - Chunking semántico con solapamiento      │
                  │   - Worker Pool concurrente en background    │
                  │   - Resolución difusa de documentos          │
                  │   - Purificación de consultas y prompts      │
                  └──────────────┬────────────────┬──────────────┘
                                 │                │
                SQL / pgvector   │                │   HTTP API
                                 ▼                ▼
                  ┌─────────────────────┐  ┌─────────────────────┐
                  │  REPOSITORY LAYER   │  │    OLLAMA CLIENT    │
                  │   (PostgreSQL +     │  │   - nomic-embed     │
                  │    pgxpool Pool)    │  │   - qwen3.5         │
                  └─────────────────────┘  └─────────────────────┘
```

---

## 🔄 1. Flujo de Subida y Vectorización de Documentos

Cuando un usuario sube un archivo mediante `POST /upload` o desde la interfaz web, el sistema ejecuta el siguiente pipeline:

```
[Archivo .pdf/.docx/.txt/.md] 
            │
            ▼
 1. Extracción de Texto ─────► Decodifica según el tipo MIME (pdfcpu, docx parser, UTF-8 text).
            │
            ▼
 2. Chunking Semántico  ─────► Divide el texto en fragmentos de ~1200 caracteres con 150 caracteres
            │                  de solapamiento (overlap), respetando límites de oraciones y párrafos.
            ▼
 3. Registro Inicial    ─────► Registra el documento en la tabla `documents` con UUID único.
            │
            ▼
 4. Respuesta Inmediata ─────► Devuelve `200 OK` con ID del documento y total de chunks estimados.
            │
            ▼ (En segundo plano)
 5. Worker Pool Concurrente ─► Spawnea N workers (`CONCURRENCY_WORKERS=6`) mediante goroutines y canales:
                               • Genera embeddings vectoriales de 768 dimensiones vía Ollama (`nomic-embed-text`).
                               • Inserta en PostgreSQL (`chunks`) con su índice posicional y vector de embedding.
                               • Muestra logs de progreso atómico (`atomic.Int64`) por porcentaje.
```

### ¿Qué se tiene en cuenta durante la ingesta?
1. **Preservación del Contexto Cruzado (*Overlap*)**: El solapamiento de 150 caracteres entre fragmentos consecutivos garantiza que ideas, definiciones o nombres que caigan en el corte de un chunk no pierdan su significado semántico.
2. **Respeto a Fronteras de Oraciones**: El chunker no corta palabras a la mitad; busca puntos (`.`), signos de interrogación (`?`), exclamación (`!`) o saltos de línea para realizar divisiones naturales.
3. **No Bloqueo del Servidor HTTP**: La respuesta al cliente es instantánea tras guardar el archivo temporal, ejecutando la vectorización de cientos de fragmentos de forma asíncrona mediante un pool de workers.

---

## 🔍 2. Flujo de Consulta RAG (Búsqueda y Generación)

Cuando un usuario realiza una pregunta mediante `POST /query` o en el chat web, se ejecuta el siguiente flujo:

```
[Pregunta del Usuario: "¿Cómo se maneja el aislamiento de transacciones en el doc DDIA?"]
                                         │
                                         ▼
 1. Resolución Difusa de Documentos (Fuzzy Document Matching)
    • Analiza si el usuario mencionó un documento específico o acrónimo ("DDIA", "lineamientos", etc.).
    • Si coincide con algún archivo indexado, extrae el filtro automáticamente.
    • Purifica la consulta eliminando conectores ("del doc...", "en el libro...").
    • Consulta purificada para embedding: "Cómo se maneja el aislamiento de transacciones"
                                         │
                                         ▼
 2. Generación de Embedding de la Pregunta
    • Ollama genera el vector de 768 dimensiones correspondiente a la pregunta purificada.
                                         │
                                         ▼
 3. Búsqueda Vectorial por Similitud de Coseno en PostgreSQL
    • Ejecuta la consulta SQL optimizada con pgvector:
      SELECT content, 1 - (embedding <=> $1) AS similarity 
      FROM chunks WHERE ($2 = '' OR filename ILIKE '%' || $2 || '%')
      ORDER BY embedding <=> $1 LIMIT $3;
                                         │
                                         ▼
 4. Filtrado de Calidad y Umbral de Similitud
    • Descarta fragmentos vacíos o con similitud inferior a `min_similarity`.
    • Si no hay fragmentos válidos, responde inmediatamente sin invocar al modelo:
      "No encontré fragmentos relevantes en los documentos..."
                                         │
                                         ▼
 5. Inyección en Prompt Anti-Alucinaciones y Multilingüe
    • Inyecta los fragmentos numerados con su fuente explícita.
    • Instruye al modelo a responder estrictamente con base en los fragmentos y a traducir/interpretar
      fielmente al español términos técnicos si el documento original está en inglés.
                                         │
                                         ▼
 6. Generación / Streaming SSE en Tiempo Real
    • Emite evento `event: meta` con fuentes y similitud máxima.
    • Emite eventos `event: token` a medida que Ollama genera cada token.
    • Emite evento `event: done` con `[DONE]` para cerrar la conexión.
```

---

## 🚀 Requisitos y Configuración

### 1. Requisitos Previos
- **Go** 1.22 o superior instalado.
- **PostgreSQL** con la extensión `pgvector` habilitada.
- **Ollama** con los modelos configurados:
  ```bash
  ollama pull nomic-embed-text:latest
  ollama pull qwen3.5:2b
  ```

### 2. Variables de Entorno (`.env`)
Configura tu archivo `.env` en la raíz del proyecto:
```env
DB_USER=postgres
DB_PASSWORD=tu_password
DB_HOST=localhost
DB_PORT=5432
DB_NAME=rag_db
DB_SSLMODE=disable

OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_EMBED_MODEL=nomic-embed-text:latest
OLLAMA_GEN_MODEL=qwen3.5:2b

PORT=8080
CHUNK_SIZE=1200
CHUNK_OVERLAP=150
CONCURRENCY_WORKERS=6
GO_ENV=dev
```

### 3. Inicialización y Ejecución
1. Crear base de datos y esquema:
   ```bash
   ./setup.sh
   ```
2. Iniciar el microservicio:
   ```bash
   go run .
   ```
3. Abrir la interfaz web en tu navegador:
   ```
   http://localhost:8080/
   ```

---

## 💻 Interfaz Web

El microservicio incluye una interfaz web completa servida directamente mediante plantillas de Go (`html/template`) y Tailwind CSS:

- **Tarjeta Flotante Central**: Entrada de texto estilo Perplexity/CraftGPT con fondo de gradiente radial etéreo (*ambient glow*).
- **Subida Drag & Drop con Bloqueo Anti-Saturación**: Selector de archivos con indicador animado de carga en segundo plano (`isUploading`) que previene envíos duplicados.
- **Drawer de Documentos Expandido**: Panel lateral con lista completa de archivos indexados, conteo de chunks y botón de filtrado rápido.
- **Chat en Tiempo Real con SSE**: Streaming fluido con cursor pulsante, renderizado Markdown (código, tablas, listas), badges de citas `[Fuente: ...]` e inspección de fragmentos recuperados.
- **Scroll Inteligente**: Detecta si el usuario está leyendo hacia arriba para evitar tirones forzados de scroll durante el streaming.
- **Modal de Ajustes**:
  - **Modo Razonamiento (`Think`)**: Interruptor para activar/desactivar el bloque de pensamiento del modelo (desactivado por defecto).
  - **Top K**: Selector de 1 a 15 fragmentos recuperados.
  - **Similitud Mínima (`min_similarity`)**: Umbral ajustable de 0.0 a 0.8.
  - **Filtro de Documento**: Selector para trabajar con un archivo puntual o con todo el repositorio.

---

## 📡 Documentación de Endpoints

### 1. Estado y Salud

#### `GET /health`
Endpoint de monitoreo y health check.
- **Respuesta:**
  ```json
  {
    "status": "ok"
  }
  ```

---

### 2. Subida e Indexación de Documentos

#### `POST /upload`
Sube un documento, extrae su contenido e inicia la vectorización concurrente en segundo plano.

- **Content-Type:** `multipart/form-data`
- **Parámetros:**

| Parámetro | Tipo | Requerido | Descripción |
| :--- | :--- | :--- | :--- |
| `document` | `File` | **Sí** | Archivo `.pdf`, `.docx`, `.txt` o `.md`. |
| `name` | `String` | No | Nombre o alias personalizado para el documento. |

- **Ejemplo con cURL:**
  ```bash
  curl -X POST http://localhost:8080/upload \
    -F "document=@Designing_Data_Intensive_Applications.pdf" \
    -F "name=DDIA"
  ```

- **Respuesta JSON:**
  ```json
  {
    "message": "Documento recibido correctamente. Indexacion iniciada en segundo plano.",
    "document_id": "3b29efb7-9513-4a11-8a48-a006c715f187",
    "name": "DDIA",
    "filename": "Designing_Data_Intensive_Applications.pdf",
    "chunks_total": 420,
    "status": "indexing"
  }
  ```

---

### 3. Listar Documentos Indexados

#### `GET /documents`
Obtiene todos los documentos guardados en la base de datos junto con su total de chunks.

- **Ejemplo con cURL:**
  ```bash
  curl http://localhost:8080/documents
  ```

- **Respuesta JSON:**
  ```json
  {
    "documents": [
      {
        "id": "3b29efb7-9513-4a11-8a48-a006c715f187",
        "filename": "DDIA",
        "created_at": "2026-09-01T23:00:00Z",
        "chunk_count": 420
      }
    ]
  }
  ```

---

### 4. Búsqueda Semántica Pura

#### `POST /search`
Realiza una búsqueda de similitud coseno sin invocar al modelo generativo.

- **Content-Type:** `application/json`
- **Parámetros:**

| Campo | Tipo | Requerido | Por defecto | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| `query` | `String` | **Sí** | - | Texto de búsqueda semántica. |
| `document` | `String` | No | `""` | Filtra por nombre de documento o UUID. |
| `limit` | `Integer` | No | `5` | Número de fragmentos a recuperar. |

- **Ejemplo con cURL:**
  ```bash
  curl -X POST http://localhost:8080/search \
    -H "Content-Type: application/json" \
    -d '{
      "query": "niveles de aislamiento en transacciones",
      "document": "DDIA",
      "limit": 3
    }'
  ```

---

### 5. Consulta RAG y Generación (`POST /query`)

#### `POST /query`
Recupera los fragmentos pertinentes y genera la respuesta fundamentada con el modelo LLM.

- **Content-Type:** `application/json`
- **Parámetros:**

| Campo | Tipo | Requerido | Por defecto | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| `query` | `String` | **Sí** | - | Pregunta formulada por el usuario. |
| `document` | `String` | No | `""` | Filtra por documento (si se omite, se detecta automáticamente en la consulta o se busca en todos). |
| `top_k` | `Integer` | No | `5` | Cantidad de fragmentos relevantes a recuperar. |
| `generate` | `Boolean` | No | `false` | Si es `true`, genera la respuesta con Ollama. |
| `stream` | `Boolean` | No | `false` | Si es `true`, transmite la respuesta token por token vía SSE. |
| `think` | `Boolean` | No | `false` | Habilita o deshabilita la visualización del razonamiento interno del modelo. |
| `min_similarity` | `Float` | No | `0.0` | Umbral mínimo de similitud coseno requerido. |
| `include_contexts` | `Boolean` | No | `true` | Si es `false`, omite el array de textos en la respuesta. |

---

#### Ejemplo 1: Consulta JSON Estándar (No Streaming)

```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "¿Qué es el aislamiento de instantánea (Snapshot Isolation)?",
    "document": "DDIA",
    "top_k": 5,
    "generate": true,
    "include_contexts": false
  }'
```

**Respuesta JSON:**
```json
{
  "query": "¿Qué es el aislamiento de instantánea (Snapshot Isolation)?",
  "contexts": [],
  "sources": [
    "DDIA"
  ],
  "max_similarity": 0.512,
  "answer": "El aislamiento de instantánea (Snapshot Isolation) es un nivel de aislamiento en bases de datos que garantiza que cada transacción lea una instantánea consistente de la base de datos tomada en el momento en que la transacción comenzó [Fuente: DDIA].\n\n- **Principio clave:** Los lectores nunca bloquean a los escritores, y los escritores nunca bloquean a los lectores [Fuente: DDIA].\n- **Implementación:** Se basa habitualmente en Control de Concurrencia Multiversión (MVCC), donde la base de datos conserva múltiples versiones de un mismo objeto etiquetadas con números de transacción [Fuente: DDIA]."
}
```

---

#### Ejemplo 2: Consulta con Streaming en Tiempo Real (SSE)

```bash
curl -N -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "¿Cuáles son los niveles de aislamiento débiles según DDIA?",
    "generate": true,
    "stream": true,
    "include_contexts": false
  }'
```

**Flujo de Eventos Recibido:**
```
event: meta
data: {"contexts":[],"max_similarity":0.495,"query":"¿Cuáles son los niveles de aislamiento débiles según DDIA?","sources":["DDIA"]}

event: token
data: {"token":"Los"}

event: token
data: {"token":" niveles"}

event: token
data: {"token":" de"}

event: token
data: {"token":" aislamiento"}

event: token
data: {"token":" más"}

event: token
data: {"token":" comunes"}

...

event: done
data: [DONE]
```

---

## 🧹 Mantenimiento

Para limpiar la base de datos y eliminar archivos temporales:
```bash
./clean_db.sh
```
