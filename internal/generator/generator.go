// Package generator produces .docx files from source file listings.
// It builds the ZIP/XML structure of OOXML directly — no third-party
// Word libraries required.
//
// The generated document exposes two named paragraph styles that the user
// can freely modify inside Word after opening the file:
//
//   - "LG Title"  — file path headings
//   - "LG Code"   — source code blocks
package generator

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"listgen/internal/config"
	"listgen/internal/tree"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// ─── Public entry point ───────────────────────────────────────────────────────

func Generate(basePath string, filePaths []string, cfg *config.Config) error {
	gc := cfg.Generating

	doc := newDocBuilder(gc)

	// File tree
	if gc.GenerateTree {
		relPaths := make([]string, len(filePaths))
		for i, p := range filePaths {
			if rel, err := filepath.Rel(basePath, p); err == nil {
				relPaths[i] = rel
			} else {
				relPaths[i] = p
			}
		}
		doc.addTitle("Структура проекта:")
		doc.addCode(tree.Build(relPaths, filepath.Base(basePath)))
	}

	// File contents
	for _, filePath := range filePaths {
		label := filepath.Base(filePath)
		if gc.FilesWithPath {
			if rel, err := filepath.Rel(basePath, filePath); err == nil {
				label = rel
			}
		}
		doc.addTitle(label + ":")

		content, err := os.ReadFile(filePath)
		if err != nil {
			doc.addCode(fmt.Sprintf("[error reading file: %v]", err))
			continue
		}
		doc.addCode(sanitize(string(content)))
	}

	// Write
	if err := os.MkdirAll(filepath.Dir(gc.Output), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	f, err := os.Create(gc.Output)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if err := doc.writeTo(f); err != nil {
		return fmt.Errorf("saving document: %w", err)
	}

	fmt.Printf("Saved: %s\n", gc.Output)
	return nil
}

// ─── Internal units ───────────────────────────────────────────────────────────

// 1 cm → twips (twentieths of a point). 1 inch = 2.54 cm = 72 pt = 1440 twips.
func cmToTwips(cm float64) int { return int(cm / 2.54 * 1440) }

// pt → half-points (unit used by <w:sz>).
func ptToHalfPt(pt float64) int { return int(pt * 2) }

// lineSpacingVal converts a multiplier to OpenXML line spacing units (240 = single).
func lineSpacingVal(mult float64) int { return int(mult * 240) }

// ─── Style IDs ────────────────────────────────────────────────────────────────

const (
	styleIDTitle = "LGTitle"
	styleIDCode  = "LGCode"
)

// ─── docBuilder ───────────────────────────────────────────────────────────────

type docBuilder struct {
	gc   config.Generating
	body []string // raw XML snippets for <w:body>
}

func newDocBuilder(gc config.Generating) *docBuilder {
	return &docBuilder{gc: gc}
}

// addTitle appends a title paragraph referencing the LGTitle style.
func (d *docBuilder) addTitle(text string) {
	var sb strings.Builder
	sb.WriteString(`<w:p>`)
	sb.WriteString(`<w:pPr>`)
	sb.WriteString(`<w:pStyle w:val="` + styleIDTitle + `"/>`)

	// Optional spacing before/after title
	var spacingParts []string
	if d.gc.IntervalBeforeTitle {
		spacingParts = append(spacingParts, `w:before="240"`)
	}
	if d.gc.IntervalAfterTitle {
		spacingParts = append(spacingParts, `w:after="240"`)
	}
	if len(spacingParts) > 0 {
		sb.WriteString(`<w:spacing ` + strings.Join(spacingParts, " ") + `/>`)
	}

	sb.WriteString(`</w:pPr>`)
	sb.WriteString(`<w:r><w:t xml:space="preserve">` + xmlEscape(text) + `</w:t></w:r>`)
	sb.WriteString(`</w:p>`)
	d.body = append(d.body, sb.String())
}

// addCode appends one paragraph per line, all referencing the LGCode style.
// Borders are handled per-paragraph so the block looks like one unified box.
func (d *docBuilder) addCode(content string) {
	lines := strings.Split(content, "\n")

	// Strip trailing blank lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}

	n := len(lines)
	for i, line := range lines {
		var sb strings.Builder
		sb.WriteString(`<w:p>`)
		sb.WriteString(`<w:pPr>`)
		sb.WriteString(`<w:pStyle w:val="` + styleIDCode + `"/>`)

		lsv := lineSpacingVal(d.gc.CodeLineSpacing)
		sb.WriteString(fmt.Sprintf(
			`<w:spacing w:line="%d" w:lineRule="auto" w:before="0" w:after="0"/>`,
			lsv,
		))

		if d.gc.CodeIndent > 0 {
			sb.WriteString(fmt.Sprintf(`<w:ind w:left="%d"/>`, cmToTwips(d.gc.CodeIndent)))
		}

		// Draw a unified border box: left+right on every line,
		// top only on first, bottom only on last.
		if d.gc.CodeBorder {
			sb.WriteString(`<w:pBdr>`)
			sb.WriteString(borderTag("left", true))
			sb.WriteString(borderTag("right", true))
			sb.WriteString(borderTag("top", i == 0))
			sb.WriteString(borderTag("bottom", i == n-1))
			sb.WriteString(`</w:pBdr>`)
		}

		sb.WriteString(`</w:pPr>`)

		expandedLine := strings.ReplaceAll(line, "\t", "    ")
		sb.WriteString(`<w:r><w:t xml:space="preserve">` + xmlEscape(expandedLine) + `</w:t></w:r>`)

		sb.WriteString(`</w:p>`)
		d.body = append(d.body, sb.String())
	}
}

// borderTag returns an XML element for one side of a paragraph border.
// visible=false emits a "none" border, which overrides any style-level border
// so adjacent paragraphs inside the same block don't show internal lines.
func borderTag(side string, visible bool) string {
	if !visible {
		return fmt.Sprintf(`<w:%s w:val="none"/>`, side)
	}
	return fmt.Sprintf(
		`<w:%s w:val="single" w:sz="6" w:space="4" w:color="000000"/>`,
		side,
	)
}

// ─── DOCX assembly ────────────────────────────────────────────────────────────

func (d *docBuilder) writeTo(f *os.File) error {
	zw := zip.NewWriter(f)
	defer zw.Close()

	order := []struct{ name, content string }{
		{"[Content_Types].xml", d.contentTypes()},
		{"_rels/.rels", rootRels()},
		{"word/_rels/document.xml.rels", documentRels()},
		{"word/document.xml", d.documentXML()},
		{"word/styles.xml", d.stylesXML()},
		{"word/settings.xml", settingsXML()},
	}

	for _, e := range order {
		w, err := zw.Create(e.name)
		if err != nil {
			return fmt.Errorf("creating zip entry %q: %w", e.name, err)
		}
		if _, err := w.Write([]byte(e.content)); err != nil {
			return fmt.Errorf("writing zip entry %q: %w", e.name, err)
		}
	}

	return nil
}

// ─── XML builders ─────────────────────────────────────────────────────────────

func (d *docBuilder) contentTypes() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml"` +
		` ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/styles.xml"` +
		` ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
		`<Override PartName="/word/settings.xml"` +
		` ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>` +
		`</Types>`
}

func rootRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1"` +
		` Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"` +
		` Target="word/document.xml"/>` +
		`</Relationships>`
}

func documentRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1"` +
		` Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"` +
		` Target="styles.xml"/>` +
		`<Relationship Id="rId2"` +
		` Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"` +
		` Target="settings.xml"/>` +
		`</Relationships>`
}

func (d *docBuilder) documentXML() string {
	gc := d.gc

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`,
	)
	sb.WriteString(
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`,
	)
	sb.WriteString(`<w:body>`)

	for _, p := range d.body {
		sb.WriteString(p)
	}

	// Trailing empty paragraph (required by spec)
	sb.WriteString(`<w:p/>`)

	// Section properties: A4 page + margins
	sb.WriteString(fmt.Sprintf(
		`<w:sectPr>`+
			`<w:pgSz w:w="11906" w:h="16838"/>`+
			`<w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="708" w:footer="708" w:gutter="0"/>`+
			`</w:sectPr>`,
		cmToTwips(gc.MarginTop),
		cmToTwips(gc.MarginRight),
		cmToTwips(gc.MarginBottom),
		cmToTwips(gc.MarginLeft),
	))

	sb.WriteString(`</w:body></w:document>`)
	return sb.String()
}

// stylesXML produces the styles part with:
//   - A minimal "Normal" base style (required by Word)
//   - "LG Title" — for file path headings  (visible in Styles panel)
//   - "LG Code"  — for source code blocks  (visible in Styles panel)
//
// Both LG styles appear in Word's Styles panel so the user can click them
// and modify font, size, spacing, color, borders etc. for the whole document.
func (d *docBuilder) stylesXML() string {
	gc := d.gc

	mainSzHp := ptToHalfPt(gc.MainFontSize)
	codeSzHp := ptToHalfPt(gc.CodeFontSize)
	mainLsv := lineSpacingVal(gc.MainLineSpacing)
	codeLsv := lineSpacingVal(gc.CodeLineSpacing)
	mainIndTw := cmToTwips(gc.MainIndent)
	codeIndTw := cmToTwips(gc.CodeIndent)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`,
	)

	// ── Normal (required base style) ──────────────────────────────────────────
	sb.WriteString(`<w:style w:type="paragraph" w:default="1" w:styleId="Normal">`)
	sb.WriteString(`<w:name w:val="Normal"/>`)
	sb.WriteString(`<w:pPr><w:spacing w:after="0"/></w:pPr>`)
	sb.WriteString(`</w:style>`)

	// ── LG Title ──────────────────────────────────────────────────────────────
	sb.WriteString(`<w:style w:type="paragraph" w:styleId="` + styleIDTitle + `">`)
	sb.WriteString(`<w:name w:val="LG Title"/>`)
	sb.WriteString(`<w:basedOn w:val="Normal"/>`)
	sb.WriteString(`<w:qFormat/>`)
	sb.WriteString(`<w:pPr>`)
	sb.WriteString(
		fmt.Sprintf(`<w:spacing w:line="%d" w:lineRule="auto" w:before="0" w:after="0"/>`, mainLsv),
	)
	if mainIndTw > 0 {
		sb.WriteString(fmt.Sprintf(`<w:ind w:left="%d"/>`, mainIndTw))
	}
	sb.WriteString(`</w:pPr>`)
	sb.WriteString(`<w:rPr>`)
	sb.WriteString(fmt.Sprintf(
		`<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>`,
		gc.MainFont, gc.MainFont, gc.MainFont,
	))
	sb.WriteString(fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, mainSzHp, mainSzHp))
	sb.WriteString(`</w:rPr>`)
	sb.WriteString(`</w:style>`)

	// ── LG Code ───────────────────────────────────────────────────────────────
	sb.WriteString(`<w:style w:type="paragraph" w:styleId="` + styleIDCode + `">`)
	sb.WriteString(`<w:name w:val="LG Code"/>`)
	sb.WriteString(`<w:basedOn w:val="Normal"/>`)
	sb.WriteString(`<w:qFormat/>`)
	sb.WriteString(`<w:pPr>`)
	sb.WriteString(
		fmt.Sprintf(`<w:spacing w:line="%d" w:lineRule="auto" w:before="0" w:after="0"/>`, codeLsv),
	)
	if codeIndTw > 0 {
		sb.WriteString(fmt.Sprintf(`<w:ind w:left="%d"/>`, codeIndTw))
	}
	// Style-level border so Word shows it in the style preview.
	// Per-paragraph overrides in addCode() suppress interior top/bottom borders
	// so consecutive lines look like one unified box.
	if gc.CodeBorder {
		sb.WriteString(`<w:pBdr>`)
		for _, side := range []string{"top", "left", "bottom", "right"} {
			sb.WriteString(fmt.Sprintf(
				`<w:%s w:val="single" w:sz="6" w:space="4" w:color="000000"/>`, side,
			))
		}
		sb.WriteString(`</w:pBdr>`)
	}
	sb.WriteString(`</w:pPr>`)
	sb.WriteString(`<w:rPr>`)
	sb.WriteString(fmt.Sprintf(
		`<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>`,
		gc.CodeFont, gc.CodeFont, gc.CodeFont,
	))
	sb.WriteString(fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, codeSzHp, codeSzHp))
	sb.WriteString(`</w:rPr>`)
	sb.WriteString(`</w:style>`)

	sb.WriteString(`</w:styles>`)
	return sb.String()
}

func settingsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:defaultTabStop w:val="720"/>` +
		`</w:settings>`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func xmlEscape(s string) string {
	var buf strings.Builder
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r == utf8.RuneError && size == 1 {
			continue
		}
		if r == 0x09 || r == 0x0A || r == 0x0D ||
			(r >= 0x20 && r <= 0xD7FF) ||
			(r >= 0xE000 && r <= 0xFFFD) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
