package ingest

import (
	"strings"
	"unicode/utf8"

	"implcache-mcp/store"
)

const defaultMaxChunkBytes = 3500

// ChunkMarkdown splits markdown on ATX headings, then size-bounds large sections.
func ChunkMarkdown(content string) []store.Chunk {
	lines := splitLines(content)
	type section struct {
		heading   string
		startLine int
		lines     []string
	}

	var sections []section
	cur := section{heading: "", startLine: 1}
	for i, line := range lines {
		lineNo := i + 1
		if level, title, ok := parseATXHeading(line); ok && level >= 1 && level <= 6 {
			if len(cur.lines) > 0 || cur.heading != "" {
				sections = append(sections, cur)
			}
			cur = section{heading: title, startLine: lineNo, lines: []string{line}}
			continue
		}
		cur.lines = append(cur.lines, line)
	}
	if len(cur.lines) > 0 || cur.heading != "" {
		sections = append(sections, cur)
	}
	if len(sections) == 0 {
		return sizeChunk("", lines, 1, defaultMaxChunkBytes)
	}

	var out []store.Chunk
	for _, sec := range sections {
		parts := sizeChunk(sec.heading, sec.lines, sec.startLine, defaultMaxChunkBytes)
		out = append(out, parts...)
	}
	return out
}

// ChunkSource splits a source file by size with soft breaks on blank lines.
func ChunkSource(content string) []store.Chunk {
	lines := splitLines(content)
	return sizeChunk("", lines, 1, defaultMaxChunkBytes)
}

func sizeChunk(heading string, lines []string, startLine int, maxBytes int) []store.Chunk {
	if len(lines) == 0 {
		return nil
	}
	var (
		out       []store.Chunk
		buf       []string
		bufStart  = startLine
		bufBytes  int
		lineIndex int
	)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		body := strings.Join(buf, "\n")
		endLine := bufStart + len(buf) - 1
		out = append(out, store.Chunk{
			Heading:   heading,
			Body:      body,
			StartLine: bufStart,
			EndLine:   endLine,
		})
		buf = nil
		bufBytes = 0
	}

	for lineIndex < len(lines) {
		line := lines[lineIndex]
		lineBytes := len(line) + 1
		if bufBytes > 0 && bufBytes+lineBytes > maxBytes {
			flush()
			bufStart = startLine + lineIndex
			continue
		}
		buf = append(buf, line)
		bufBytes += lineBytes
		lineIndex++

		// Soft break on blank line once we have enough content.
		if line == "" && bufBytes >= maxBytes/2 {
			flush()
			bufStart = startLine + lineIndex
		}
	}
	flush()
	return out
}

func parseATXHeading(line string) (level int, title string, ok bool) {
	trimmed := strings.TrimRight(line, " \t")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return 0, "", false
	}
	if i < len(trimmed) && trimmed[i] != ' ' && trimmed[i] != '\t' {
		return 0, "", false
	}
	title = strings.TrimSpace(trimmed[i:])
	title = strings.TrimRight(title, "#")
	title = strings.TrimSpace(title)
	return i, title, true
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// IsValidUTF8 reports whether data is valid UTF-8 text (not binary).
func IsValidUTF8(data []byte) bool {
	return utf8.Valid(data)
}

// DecodeTextToUTF8 returns data unchanged when it is valid UTF-8.
// Otherwise it repairs invalid bytes using Windows-1252 (common in
// Word-exported HTML that embeds 0xA0 NBSPs as raw Latin-1).
func DecodeTextToUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	var b strings.Builder
	b.Grow(len(data) + 8)
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(windows1252Rune(data[i]))
			i++
			continue
		}
		b.WriteRune(r)
		i += size
	}
	return []byte(b.String())
}

// windows1252Rune maps a single byte as Windows-1252 to Unicode.
func windows1252Rune(c byte) rune {
	switch c {
	case 0x80:
		return 0x20AC // €
	case 0x82:
		return 0x201A
	case 0x83:
		return 0x0192
	case 0x84:
		return 0x201E
	case 0x85:
		return 0x2026
	case 0x86:
		return 0x2020
	case 0x87:
		return 0x2021
	case 0x88:
		return 0x02C6
	case 0x89:
		return 0x2030
	case 0x8A:
		return 0x0160
	case 0x8B:
		return 0x2039
	case 0x8C:
		return 0x0152
	case 0x8E:
		return 0x017D
	case 0x91:
		return 0x2018
	case 0x92:
		return 0x2019
	case 0x93:
		return 0x201C
	case 0x94:
		return 0x201D
	case 0x95:
		return 0x2022
	case 0x96:
		return 0x2013
	case 0x97:
		return 0x2014
	case 0x98:
		return 0x02DC
	case 0x99:
		return 0x2122
	case 0x9A:
		return 0x0161
	case 0x9B:
		return 0x203A
	case 0x9C:
		return 0x0153
	case 0x9E:
		return 0x017E
	case 0x9F:
		return 0x0178
	default:
		// 0x00-0x7F, 0xA0-0xFF, and undefined 0x81/0x8D/0x8F/0x90/0x9D
		return rune(c)
	}
}
