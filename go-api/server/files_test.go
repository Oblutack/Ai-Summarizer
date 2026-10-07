package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"testing"
)

// upload posts PDFs to a summarize route as a browser form would.
func (cl *client) upload(path, field string, files map[string][]byte) reply {
	cl.a.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for name, data := range files {
		part, err := w.CreateFormFile(field, name)
		if err != nil {
			cl.a.t.Fatal(err)
		}
		_, _ = part.Write(data)
	}
	_ = w.Close()

	req, _ := http.NewRequest(http.MethodPost, cl.a.srv.URL+path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Origin", cl.origin)
	resp, err := cl.http.Do(req)
	if err != nil {
		cl.a.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return reply{resp.StatusCode, resp.Header, data}
}

// listedDocuments returns the signed-in user's documents as the list endpoint shows them.
func listedDocuments(t *testing.T, cl *client) []map[string]any {
	t.Helper()
	var docs []map[string]any
	if err := jsonUnmarshal(cl.get("/documents").Body, &docs); err != nil {
		t.Fatalf("listing documents: %v", err)
	}
	return docs
}

func filesOf(doc map[string]any) []map[string]any {
	var out []map[string]any
	for _, f := range doc["files"].([]any) {
		out = append(out, f.(map[string]any))
	}
	return out
}

var pdfBytes = []byte("%PDF-1.4 pretend this is the real file\x00\x01\x02 with binary bytes")

func TestUploadedPDFsAreKeptAndCanBeDownloadedByTheirOwner(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	if r := cl.upload("/summarize", "file", map[string][]byte{"manual.pdf": pdfBytes}); r.Status != 200 {
		t.Fatalf("summarize: %d %s", r.Status, r.Body)
	}
	docs := listedDocuments(t, cl)
	files := filesOf(docs[0])
	if len(files) != 1 || files[0]["name"] != "manual.pdf" || files[0]["size"] != float64(len(pdfBytes)) {
		t.Fatalf("the list should describe the stored file: %v", docs[0]["files"])
	}

	url := "/documents/" + strconv.Itoa(int(docs[0]["ID"].(float64))) + "/files/" + strconv.Itoa(int(files[0]["id"].(float64)))
	r := cl.get(url)
	if r.Status != 200 || !bytes.Equal(r.Body, pdfBytes) {
		t.Fatalf("download: %d, bytes equal: %v", r.Status, bytes.Equal(r.Body, pdfBytes))
	}
	if r.Header.Get("Content-Type") != "application/pdf" || r.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers: %v", r.Header)
	}
	if got := r.Header.Get("Content-Disposition"); got != `inline; filename=manual.pdf` {
		t.Errorf("Content-Disposition: %q", got)
	}
}

func TestSeveralPDFsAreKeptSeparately(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	r := cl.upload("/summarize-multiple", "files", map[string][]byte{"a.pdf": []byte("%PDF a"), "b.pdf": []byte("%PDF bb")})
	if r.Status != 200 {
		t.Fatalf("summarize-multiple: %d %s", r.Status, r.Body)
	}
	files := filesOf(listedDocuments(t, cl)[0])
	if len(files) != 2 {
		t.Fatalf("both files should be kept: %v", files)
	}
}

func TestPastedTextAndAnonymousSummariesKeepNoFiles(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	cl.post("/summarize-text", map[string]string{"text": "just pasted words"})
	if files := filesOf(listedDocuments(t, cl)[0]); len(files) != 0 {
		t.Errorf("pasted text has no file: %v", files)
	}

	anon := a.newClient()
	if r := anon.upload("/public/summarize", "file", map[string][]byte{"x.pdf": pdfBytes}); r.Status != 200 {
		t.Fatalf("public summarize: %d", r.Status)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("anonymous summaries must store nothing, found %d files", n)
	}
}

func TestOtherUsersCannotDownloadAFile(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	alice.upload("/summarize", "file", map[string][]byte{"secret.pdf": pdfBytes})
	doc := listedDocuments(t, alice)[0]
	file := filesOf(doc)[0]
	url := "/documents/" + strconv.Itoa(int(doc["ID"].(float64))) + "/files/" + strconv.Itoa(int(file["id"].(float64)))

	if r := bob.get(url); r.Status != http.StatusNotFound {
		t.Errorf("Bob must not see Alice's file, got %d", r.Status)
	}
	if r := a.newClient().get(url); r.Status != http.StatusUnauthorized {
		t.Errorf("a visitor must be asked to sign in, got %d", r.Status)
	}
	// Right file id under the wrong document id is no way in either.
	if r := alice.get("/documents/999999/files/" + strconv.Itoa(int(file["id"].(float64)))); r.Status != http.StatusNotFound {
		t.Errorf("mismatched document and file ids: %d", r.Status)
	}
	if r := alice.get("/documents/abc/files/1"); r.Status != http.StatusBadRequest {
		t.Errorf("bad id: %d", r.Status)
	}
}

func TestDeletingADocumentDeletesItsFiles(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	cl.upload("/summarize", "file", map[string][]byte{"gone.pdf": pdfBytes})
	doc := listedDocuments(t, cl)[0]
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 1 {
		t.Fatalf("setup: %d files", n)
	}
	if r := cl.delete("/documents/"+strconv.Itoa(int(doc["ID"].(float64))), nil); r.Status != 200 {
		t.Fatalf("delete: %d", r.Status)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("the original PDF must go with the document, %d left", n)
	}
}

func TestDeletingTheAccountDeletesEveryFile(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	cl.upload("/summarize", "file", map[string][]byte{"one.pdf": pdfBytes})
	cl.upload("/summarize", "file", map[string][]byte{"two.pdf": pdfBytes})
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 2 {
		t.Fatalf("setup: %d files", n)
	}
	if r := cl.delete("/account", map[string]string{"confirmEmail": email, "password": goodPass}); r.Status != 200 {
		t.Fatalf("delete account: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("no file may outlive its account, %d left", n)
	}
}

func TestTheStorageCapKeepsTheSummaryButSkipsTheFile(t *testing.T) {
	a := newApp(t)
	t.Setenv("STORED_FILES_MB_PER_USER", "1")
	cl, _ := a.newUser()

	big := bytes.Repeat([]byte("x"), 700<<10) // 0.7 MB
	if r := cl.upload("/summarize", "file", map[string][]byte{"first.pdf": big}); r.Status != 200 {
		t.Fatalf("first: %d", r.Status)
	}
	if r := cl.upload("/summarize", "file", map[string][]byte{"second.pdf": big}); r.Status != 200 {
		t.Fatalf("a full storage must not fail the summary: %d", r.Status)
	}
	docs := listedDocuments(t, cl)
	if len(docs) != 2 {
		t.Fatalf("both summaries are saved: %d", len(docs))
	}
	if len(filesOf(docs[1])) != 1 || len(filesOf(docs[0])) != 0 {
		t.Errorf("only the first (older) document should have its file: new=%v old=%v", docs[0]["files"], docs[1]["files"])
	}
}

func TestStoringCanBeSwitchedOff(t *testing.T) {
	a := newApp(t)
	t.Setenv("STORED_FILES_MB_PER_USER", "0")
	cl, _ := a.newUser()
	cl.upload("/summarize", "file", map[string][]byte{"x.pdf": pdfBytes})
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("0 MB means nothing is kept, found %d", n)
	}
}

func TestStreamedSummariesKeepTheirFilesToo(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	if r := cl.upload("/summarize?stream=true", "file", map[string][]byte{"streamed.pdf": pdfBytes}); r.Status != 200 {
		t.Fatalf("stream: %d", r.Status)
	}
	if files := filesOf(listedDocuments(t, cl)[0]); len(files) != 1 || files[0]["name"] != "streamed.pdf" {
		t.Errorf("streamed summaries must keep the original too: %v", files)
	}
}

func TestTheDataExportListsStoredFilesWithoutIncludingThem(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	cl.upload("/summarize", "file", map[string][]byte{"kept.pdf": pdfBytes})

	r := cl.get("/account/export")
	if r.Status != 200 {
		t.Fatalf("export: %d %s", r.Status, r.Body)
	}
	docs := r.JSON()["documents"].([]any)
	listed := docs[0].(map[string]any)["originalFiles"].([]any)
	if len(listed) != 1 || listed[0].(map[string]any)["name"] != "kept.pdf" {
		t.Fatalf("the export should name the stored original: %v", listed)
	}
	if bytes.Contains(r.Body, []byte("pretend this is the real file")) {
		t.Error("the file's bytes must not be in the JSON export")
	}
}
