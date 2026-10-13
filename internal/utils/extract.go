package utils

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
)

// documentExtractionExtensions lists file extensions whose binary contents need
// dedicated text extraction before chunking or inlining (Construct library).
var documentExtractionExtensions = map[string]struct{}{
	".pdf":  {},
	".docx": {},
}

// IsDocumentExtractionType returns true when the file needs binary text
// extraction (PDF or DOCX) rather than being plain text already.
func IsDocumentExtractionType(fileName, contentType string) bool {
	if contentType == "application/pdf" || contentType ==
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	_, ok := documentExtractionExtensions[ext]
	return ok
}

// ExtractDocumentText pulls UTF-8 text out of PDF or DOCX bytes. For any other
// content it returns the bytes unchanged (plain-text passthrough).
func ExtractDocumentText(fileName string, contentType string, raw []byte) (string, error) {
	if !IsDocumentExtractionType(fileName, contentType) {
		return string(raw), nil
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		if contentType == "application/pdf" {
			ext = ".pdf"
		} else {
			ext = ".docx"
		}
	}
	if ext == ".pdf" {
		return extractPDFText(raw)
	}
	return extractDOCXText(raw)
}

// extractPDFText decodes every page of the PDF and joins the plain text.
func extractPDFText(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("empty pdf content")
	}
	r, err := pdflib.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("opening pdf: %w", err)
	}
	var out strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, _ := page.GetPlainText(nil)
		out.WriteString(text)
		out.WriteString("\n\n")
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return "", fmt.Errorf("pdf contained no extractable text (scanned/image pdf?)")
	}
	return text, nil
}

var (
	docxParagraphTag = regexp.MustCompile(`</w:p>`)
	docxAnyTag       = regexp.MustCompile(`<[^>]*>`)
	docxEntities     = map[string]string{"&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": "‟", "&apos;": "'"}
)

// extractDOCXText unzips the Word file and strips the main document XML down
// to readable paragraphs.
func extractDOCXText(raw []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("opening docx: %w", err)
	}
	var docXML []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("reading docx body: %w", err)
			}
			docXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", fmt.Errorf("reading docx body: %w", err)
			}
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("docx has no word/document.xml")
	}
	body := docxParagraphTag.ReplaceAllString(string(docXML), "\n")
	body = docxAnyTag.ReplaceAllString(body, "")
	for entity, literal := range docxEntities {
		body = strings.ReplaceAll(body, entity, literal)
	}
	text := strings.TrimSpace(body)
	if text == "" {
		return "", fmt.Errorf("docx contained no extractable text")
	}
	return text, nil
}
