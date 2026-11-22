package main

import (
	"strings"
)

type Chunk struct {
	Content  string
	ChunkIdx int
}

func ChunkText(text string, maxChunkSize, overlap int) []Chunk {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	rawParagraphs := strings.Split(text, "\n\n")
	var units []string

	for _, p := range rawParagraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if len(p) <= maxChunkSize {
			units = append(units, p)
			continue
		}

		for line := range strings.SplitSeq(p, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if len(line) <= maxChunkSize {
				units = append(units, line)
				continue
			}

			sentences := splitSentences(line)
			for _, s := range sentences {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}

				if len(s) <= maxChunkSize {
					units = append(units, s)
				} else {
					words := strings.Fields(s)
					currentWordGroup := ""
					for _, w := range words {
						if len(currentWordGroup)+len(w)+1 <= maxChunkSize {
							if currentWordGroup != "" {
								currentWordGroup += " "
							}
							currentWordGroup += w
						} else {
							if currentWordGroup != "" {
								units = append(units, currentWordGroup)
							}
							currentWordGroup = w
						}
					}
					if currentWordGroup != "" {
						units = append(units, currentWordGroup)
					}
				}
			}
		}
	}

	var chunks []Chunk
	var currentUnits []string
	currentLen := 0
	chunkIdx := 0

	for _, unit := range units {
		addedLen := len(unit)
		if len(currentUnits) > 0 {
			addedLen += 2
		}

		if currentLen+addedLen <= maxChunkSize {
			currentUnits = append(currentUnits, unit)
			currentLen += addedLen
		} else {
			if len(currentUnits) > 0 {
				chunks = append(chunks, Chunk{
					Content:  strings.Join(currentUnits, "\n\n"),
					ChunkIdx: chunkIdx,
				})
				chunkIdx++
			}

			var overlapUnits []string
			overlapLen := 0
			for i := len(currentUnits) - 1; i >= 0; i-- {
				u := currentUnits[i]
				if overlapLen+len(u) <= overlap {
					overlapUnits = append([]string{u}, overlapUnits...)
					overlapLen += len(u) + 2
				} else {
					break
				}
			}

			currentUnits = append(overlapUnits, unit)
			currentLen = 0
			for i, u := range currentUnits {
				currentLen += len(u)
				if i > 0 {
					currentLen += 2
				}
			}
		}
	}

	if len(currentUnits) > 0 {
		chunks = append(chunks, Chunk{
			Content:  strings.Join(currentUnits, "\n\n"),
			ChunkIdx: chunkIdx,
		})
	}

	return chunks
}

func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	n := len(runes)

	for i := range n {
		current.WriteRune(runes[i])
		if (runes[i] == '.' || runes[i] == '!' || runes[i] == '?' || runes[i] == ';') && (i+1 == n || runes[i+1] == ' ' || runes[i+1] == '\n') {
			sentences = append(sentences, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}
	return sentences
}
