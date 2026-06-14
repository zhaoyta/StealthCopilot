package resume

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestExtractTextFromDOCX(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Create document.xml: %v", err)
	}
	_, _ = w.Write([]byte(`<w:document><w:body><w:p><w:r><w:t>Distributed systems</w:t></w:r></w:p><w:p><w:r><w:t>Go services</w:t></w:r></w:p></w:body></w:document>`))
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}

	got := extractText("resume.docx", buf.Bytes())
	if got != "Distributed systems\nGo services" {
		t.Fatalf("extractText(docx) = %q", got)
	}
}

func TestExtractTextFromPDFLiterals(t *testing.T) {
	data := []byte(`%PDF-1.4
BT
(Hello \(platform\)) Tj
(Production ready) Tj
ET
%%EOF`)

	got := extractText("resume.pdf", data)
	if got != "Hello (platform)\nProduction ready" {
		t.Fatalf("extractText(pdf) = %q", got)
	}
}
