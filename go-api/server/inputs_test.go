package server

import (
	"net/http"
	"strings"
	"testing"
)

// Web links and Word/PowerPoint files: the other ways to put a document in.

func lastSummaryRequest(a *app) string {
	v, _ := a.ai.lastSummary.Load().(string)
	return v
}

func TestASignedInUserCanSummarizeAWebLink(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	r := cl.post("/summarize-url?style=brief&wordCount=200", map[string]string{"url": "  https://news.example/post  "})
	if r.Status != http.StatusOK || r.Str("summary") != "fake url summary" || r.Str("filename") != "Fake Page Title" {
		t.Fatalf("summarize-url: %d %s", r.Status, r.Body)
	}
	// The address reached the AI service cleaned up, with the chosen options.
	sent := lastSummaryRequest(a)
	if !strings.Contains(sent, `"url":"https://news.example/post"`) || !strings.Contains(sent, "style=brief") || !strings.Contains(sent, "word_count=200") {
		t.Errorf("the AI service got: %s", sent)
	}

	// Saved with the page title, and with the page text so it can be chatted with.
	docs := listedDocuments(t, cl)
	if len(docs) != 1 || docs[0]["Filename"] != "Fake Page Title" || docs[0]["hasContent"] != true {
		t.Errorf("saved document: %v", docs)
	}
	if files := filesOf(docs[0]); len(files) != 0 {
		t.Errorf("a web page has no original file to keep: %v", files)
	}
}

func TestAWebLinkCanBeStreamedAndIsSavedUnderThePageTitle(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	r := cl.post("/summarize-url?stream=true", map[string]string{"url": "https://news.example/post"})
	if r.Status != http.StatusOK || !strings.Contains(string(r.Body), `"filename":"Fake Page Title"`) {
		t.Fatalf("stream: %d %s", r.Status, r.Body)
	}
	if strings.Contains(string(r.Body), "the page text") {
		t.Errorf("the page text must stay on the server: %s", r.Body)
	}
	if docs := listedDocuments(t, cl); len(docs) != 1 || docs[0]["Filename"] != "Fake Page Title" {
		t.Errorf("saved document: %v", docs)
	}
}

func TestAnonymousVisitorsCanSummarizeAWebLinkAndNothingIsSaved(t *testing.T) {
	a := newApp(t)
	anon := a.newClient()
	if r := anon.post("/public/summarize-url", map[string]string{"url": "https://news.example/post"}); r.Status != http.StatusOK {
		t.Fatalf("public summarize-url: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 0 {
		t.Errorf("anonymous summaries must store nothing, found %d documents", n)
	}
}

func TestSummarizingALinkNeedsAnAccount(t *testing.T) {
	a := newApp(t)
	if r := a.newClient().post("/summarize-url", map[string]string{"url": "https://news.example/post"}); r.Status != http.StatusUnauthorized {
		t.Errorf("signed-out use of the saving route: %d", r.Status)
	}
}

func TestBadWebAddressesAreRejectedBeforeTheAIServiceIsCalled(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	bad := map[string]string{
		"empty":       "",
		"blanks":      "   ",
		"ftp":         "ftp://files.example/a",
		"file":        "file:///etc/passwd",
		"javascript":  "javascript://alert(1)",
		"has a space": "https://news.example/a post",
		"too long":    "https://news.example/" + strings.Repeat("a", 2100),
	}
	for name, address := range bad {
		if r := cl.post("/summarize-url", map[string]string{"url": address}); r.Status != http.StatusBadRequest {
			t.Errorf("%s: got %d %s", name, r.Status, r.Body)
		}
	}
	if r := cl.post("/summarize-url", map[string]string{"text": "not a url"}); r.Status != http.StatusBadRequest {
		t.Errorf("a body without a url: %d", r.Status)
	}
	if r := cl.post("/summarize-url?style=poem", map[string]string{"url": "https://news.example/post"}); r.Status != http.StatusBadRequest {
		t.Errorf("an unknown style: %d", r.Status)
	}
	if n := a.ai.calls.Load(); n != 0 {
		t.Errorf("rejected requests must not reach the AI service, it was called %d times", n)
	}
}

func TestAFailedWebLinkIsRefunded(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "2")
	cl, _ := a.newUser()

	a.ai.fail.Store(true)
	for i := 0; i < 4; i++ {
		if r := cl.post("/summarize-url", map[string]string{"url": "https://news.example/post"}); r.Status != http.StatusBadGateway {
			t.Fatalf("AI failure should be a 502, got %d", r.Status)
		}
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != 0 {
		t.Errorf("failed requests must not use up the allowance, usage = %d", n)
	}
}

var (
	docxBytes = []byte("PK\x03\x04 pretend this is a Word file")
	pptxBytes = []byte("PK\x03\x04 pretend this is a PowerPoint file")
)

func TestWordAndPowerPointFilesAreAccepted(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	for _, name := range []string{"lease.docx", "Deck.PPTX"} {
		data := docxBytes
		if strings.HasSuffix(strings.ToLower(name), ".pptx") {
			data = pptxBytes
		}
		if r := cl.upload("/summarize", "file", map[string][]byte{name: data}); r.Status != http.StatusOK {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	// Saved, but only PDFs are kept as originals (the viewer shows PDF pages).
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 2 {
		t.Errorf("documents saved = %d, want 2", n)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("Word and PowerPoint originals are not kept, found %d", n)
	}
}

func TestAMixOfFilesKeepsOnlyThePDFs(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	r := cl.upload("/summarize-multiple", "files", map[string][]byte{"a.pdf": pdfBytes, "b.docx": docxBytes, "c.pptx": pptxBytes})
	if r.Status != http.StatusOK {
		t.Fatalf("summarize-multiple: %d %s", r.Status, r.Body)
	}
	files := filesOf(listedDocuments(t, cl)[0])
	if len(files) != 1 || files[0]["name"] != "a.pdf" {
		t.Errorf("only the PDF should be kept: %v", files)
	}
}

func TestOtherFileTypesAreRefusedWithAClearMessage(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	for _, name := range []string{"notes.txt", "old.doc", "slides.ppt", "sheet.xlsx", "script.exe", "noextension"} {
		r := cl.upload("/summarize", "file", map[string][]byte{name: []byte("data")})
		if r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "PDF, Word") {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	r := cl.upload("/summarize-multiple", "files", map[string][]byte{"a.pdf": pdfBytes, "b.zip": []byte("PK")})
	if r.Status != http.StatusBadRequest {
		t.Errorf("a mixed upload with one unsupported file: %d", r.Status)
	}
	if n := a.ai.calls.Load(); n != 0 {
		t.Errorf("refused files must not reach the AI service, it was called %d times", n)
	}
}
