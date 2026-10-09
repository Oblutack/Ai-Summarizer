package server

import (
	"net/http"
	"strings"
	"testing"

	"ai-summarizer/go-api/controllers"
)

// Photos of pages: signed-in people can upload them, the AI service reads the text, and several photos in one
// upload are the pages of one document. The pictures themselves are not kept.

var jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("pretend this is a photo")...)

func TestASignedInUserCanSummarizePhotosOfPages(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	for _, name := range []string{"page.jpg", "Page.JPEG", "scan.png", "web.webp", "IMG 0042.jpg"} {
		if r := cl.upload("/summarize", "file", map[string][]byte{name: jpegBytes}); r.Status != http.StatusOK {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
		if sent := lastSummaryRequest(a); !strings.Contains(sent, `name="ocr"`) {
			t.Errorf("%s was sent without permission to read its text: %s", name, sent)
		}
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 5 {
		t.Errorf("documents = %d, want 5", n)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("photos are not kept, found %d stored files", n)
	}
}

func TestPhotosNeedAnAccountEvenOnThePublicRoutes(t *testing.T) {
	a := newApp(t)
	anon := a.newClient()

	r := anon.upload("/public/summarize", "file", map[string][]byte{"page.jpg": jpegBytes})
	if r.Status != http.StatusUnauthorized || !strings.Contains(r.Str("error"), "Sign in") {
		t.Errorf("public single: %d %s", r.Status, r.Body)
	}
	r = anon.upload("/public/summarize-multiple", "files", map[string][]byte{"a.pdf": pdfBytes, "page.png": jpegBytes})
	if r.Status != http.StatusUnauthorized {
		t.Errorf("public multiple with a photo: %d %s", r.Status, r.Body)
	}
	if n := a.ai.calls.Load(); n != 0 {
		t.Errorf("a refused photo must not reach the AI service (it was called %d times)", n)
	}
}

func TestSeveralPhotosAndOtherFilesCanBeCombined(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	r := cl.upload("/summarize-multiple", "files", map[string][]byte{"p1.jpg": jpegBytes, "p2.jpg": jpegBytes, "agenda.pdf": pdfBytes})
	if r.Status != http.StatusOK {
		t.Fatalf("combined: %d %s", r.Status, r.Body)
	}
	sent := lastSummaryRequest(a)
	for _, name := range []string{"p1.jpg", "p2.jpg", "agenda.pdf"} {
		if !strings.Contains(sent, `filename="`+name+`"`) {
			t.Errorf("%s did not reach the AI service", name)
		}
	}
	// Only the PDF is kept as an original.
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 1 {
		t.Errorf("stored files = %d, want 1 (the PDF)", n)
	}
}

func TestOnlyPhotoFormatsThatCanBeReadAreAccepted(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	for _, name := range []string{"page.gif", "page.heic", "page.bmp", "page.tiff", "page.svg", "photo.jpg.exe"} {
		r := cl.upload("/summarize", "file", map[string][]byte{name: jpegBytes})
		if r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "photos (JPG, PNG, WebP)") {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	if n := a.ai.calls.Load(); n != 0 {
		t.Errorf("a refused file must not reach the AI service (it was called %d times)", n)
	}
}

func TestAPhotoCanBeNoBiggerThanADocument(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	big := make([]byte, controllers.MaxPDFBytes+1)
	r := cl.upload("/summarize", "file", map[string][]byte{"page.jpg": big})
	if r.Status != http.StatusRequestEntityTooLarge || !strings.Contains(r.Str("error"), "page.jpg") {
		t.Errorf("a photo over the limit: %d %s", r.Status, r.Body)
	}
}
