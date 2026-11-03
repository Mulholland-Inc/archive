package main

import (
	"path"
	"strings"

	"github.com/Mulholland-Inc/archive/connect"
)

// Text extraction that uploads and synced files share: a file's words, from a PDF, a Word document, an Excel
// workbook, a PowerPoint deck, HTML or plain text, so search finds what is in it wherever it came from. The
// connectors read it as connect.Extract, as they cannot import this package.
func init() { connect.Extract = mediaText }

// officeMIME are the Office formats whose text OfficeRead reads, by media type, as the extension it reads them by.
var officeMIME = map[string]string{
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   "docx",
	"application/vnd.ms-word.document.macroenabled.12":                          "docm",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.ms-excel.sheet.macroenabled.12":                            "xlsm",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/vnd.ms-powerpoint.presentation.macroenabled.12":                "pptm",
}

// mediaText is the text of b, a file named name of media type mime: "" for a kind it has no text of, or one
// it cannot read.
func mediaText(name, mime string, b []byte) string {
	mime = strings.ToLower(mime)
	top, _, _ := strings.Cut(mime, "/")
	if ext, ok := officeMIME[mime]; ok {
		return officeText(b, "file."+ext)
	}
	if _, err := OfficeKind(name); err == nil {
		return officeText(b, path.Base(name))
	}
	switch {
	case mime == "application/pdf":
		text, _, _ := mediaPDF(b)
		return text
	case mime == "text/html":
		return mediaHTML(b)
	case top == "text" || mime == "application/json":
		return string(b)
	}
	return ""
}

// officeText is the words of an Office file: a document's paragraphs and tables, a workbook's sheets as rows of
// cells, and a deck's slides, each on lines of their own.
func officeText(b []byte, name string) (text string) {
	defer func() {
		if recover() != nil {
			text = ""
		}
	}()
	m, err := OfficeRead(b, name)
	if err != nil {
		return ""
	}
	var sb strings.Builder
	line := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			sb.WriteString(s)
			sb.WriteByte('\n')
		}
	}
	for _, p := range m.Paragraphs {
		line(p.Text)
	}
	for _, t := range m.Tables {
		for _, r := range t.Rows {
			line(strings.Join(r, "\t"))
		}
	}
	for _, s := range m.Sheets {
		line("# " + s.Name)
		row, cells := "", []string{}
		flush := func() {
			line(strings.Join(cells, "\t"))
			cells = cells[:0]
		}
		for _, c := range s.Cells {
			if r := strings.TrimLeft(c.Ref, "ABCDEFGHIJKLMNOPQRSTUVWXYZ$"); r != row {
				flush()
				row = r
			}
			if c.Value != "" {
				cells = append(cells, c.Value)
			}
		}
		flush()
	}
	for _, s := range m.Slides {
		if s.Hidden {
			continue
		}
		for _, sh := range s.Shapes {
			for _, p := range sh.Paragraphs {
				line(p.Text)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
