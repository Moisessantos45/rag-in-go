#!/bin/bash

echo " Configurando RAG..."

echo " Creando base de datos 'rag'..."
createdb -U user_db rag 2>/dev/null || echo "La base de datos ya existe o no se pudo crear directamente"


echo " Inicializando esquema..."
if command -v sudo >/dev/null 2>&1; then
    sudo -u postgres psql -d rag -f init_db.sql
else
    psql -h localhost -U user_db -d rag -f init_db.sql
fi

echo ""
echo " Setup completado!"
echo ""
echo "Ahora ejecuta tu aplicación:"
echo "  go run main.go"