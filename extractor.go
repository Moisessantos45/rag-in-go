package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

func ExtractText(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".pdf":
		return extractPDFText(filePath)
	case ".docx":
		return extractDocxText(filePath)
	default:
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

func extractPDFText(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error abriendo archivo PDF: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("error extrayendo texto del PDF: %w", err)
	}
	if _, err := buf.ReadFrom(b); err != nil {
		return "", fmt.Errorf("error leyendo texto del PDF: %w", err)
	}

	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", fmt.Errorf("el PDF no contiene texto plano extraíble (posiblemente sea una imagen escaneada)")
	}
	return text, nil
}

func extractDocxText(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("error abriendo archivo DOCX: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			decoder := xml.NewDecoder(rc)
			var sb strings.Builder
			for {
				token, err := decoder.Token()
				if err != nil {
					break
				}
				switch elem := token.(type) {
				case xml.CharData:
					sb.Write(elem)
				case xml.EndElement:
					if elem.Name.Local == "p" || elem.Name.Local == "tr" {
						sb.WriteString("\n")
					}
				}
			}
			text := strings.TrimSpace(sb.String())
			if text == "" {
				return "", fmt.Errorf("el archivo DOCX está vacío")
			}
			return text, nil
		}
	}
	return "", fmt.Errorf("no se encontró el contenido principal en el archivo DOCX")
}
