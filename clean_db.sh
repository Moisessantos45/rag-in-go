#!/bin/bash

# Script para limpiar la base de datos y archivos residuales del RAG
echo " Iniciando limpieza de la base de datos RAG..."

# Cargar variables del .env si existe
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

DB_USER="${DB_USER:-user_db}"
DB_NAME="${DB_NAME:-rag}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"

# Truncar tablas (CASCADE limpia automáticamente chunks al limpiar documents)
SQL="TRUNCATE TABLE documents CASCADE;"

if [ -n "$DB_PASSWORD" ]; then
  export PGPASSWORD="$DB_PASSWORD"
  psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$SQL"
else
  # Intentar con usuario user_db por socket/tcp o con sudo postgres si es necesario
  psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$SQL" 2>/dev/null || \
  sudo -u postgres psql -d "$DB_NAME" -c "$SQL"
fi

# Limpiar archivos en la carpeta de uploads
if [ -d "./uploads" ]; then
  rm -rf ./uploads/*
  echo " Carpeta ./uploads vaciada"
fi

echo ""
echo " Estado actual de la base de datos:"
if [ -n "$DB_PASSWORD" ]; then
  export PGPASSWORD="$DB_PASSWORD"
  psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT (SELECT count(*) FROM documents) as total_documentos, (SELECT count(*) FROM chunks) as total_chunks;"
else
  psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT (SELECT count(*) FROM documents) as total_documentos, (SELECT count(*) FROM chunks) as total_chunks;" 2>/dev/null || \
  sudo -u postgres psql -d "$DB_NAME" -c "SELECT (SELECT count(*) FROM documents) as total_documentos, (SELECT count(*) FROM chunks) as total_chunks;"
fi

echo ""
echo " Base de datos RAG completamente limpia y lista para nuevas pruebas."
