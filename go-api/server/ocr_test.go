package server

import (
	"strings"
	"testing"
)

// Scanned pages are read by the AI service only when the gateway says so, and it says so only for signed-in people
// (reading pictures of text is real work on the server).

func TestSignedInPeopleAllowScannedPagesToBeRead(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	if r := cl.upload("/summarize", "file", map[string][]byte{"scan.pdf": pdfBytes}); r.Status != 200 {
		t.Fatalf("single: %d %s", r.Status, r.Body)
	}
	if sent := lastSummaryRequest(a); !strings.Contains(sent, `name="ocr"`) || !strings.Contains(sent, "true") {
		t.Errorf("a single PDF was sent without permission to read scans: %s", sent)
	}

	if r := cl.upload("/summarize-multiple", "files", map[string][]byte{"a.pdf": pdfBytes, "b.pdf": pdfBytes}); r.Status != 200 {
		t.Fatalf("multiple: %d %s", r.Status, r.Body)
	}
	if sent := lastSummaryRequest(a); !strings.Contains(sent, `name="ocr"`) {
		t.Errorf("several PDFs were sent without permission to read scans: %s", sent)
	}

	if r := cl.post("/summarize-url", map[string]string{"url": "https://files.example/scan.pdf"}); r.Status != 200 {
		t.Fatalf("link: %d %s", r.Status, r.Body)
	}
	if sent := lastSummaryRequest(a); !strings.Contains(sent, "ocr=true") {
		t.Errorf("a link was sent without permission to read scans: %s", sent)
	}
}

func TestPublicRoutesNeverAskForScannedPagesToBeRead(t *testing.T) {
	a := newApp(t)
	anon := a.newClient()

	if r := anon.upload("/public/summarize", "file", map[string][]byte{"scan.pdf": pdfBytes}); r.Status != 200 {
		t.Fatalf("single: %d %s", r.Status, r.Body)
	}
	if sent := lastSummaryRequest(a); strings.Contains(sent, `name="ocr"`) {
		t.Errorf("a public upload must not enable OCR: %s", sent)
	}

	if r := anon.post("/public/summarize-url", map[string]string{"url": "https://files.example/scan.pdf"}); r.Status != 200 {
		t.Fatalf("link: %d %s", r.Status, r.Body)
	}
	if sent := lastSummaryRequest(a); strings.Contains(sent, "ocr=") {
		t.Errorf("a public link must not enable OCR: %s", sent)
	}
}
