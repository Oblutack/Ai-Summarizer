package server

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"ai-summarizer/go-api/controllers"
)

// Recordings: signed-in people can upload them, they are transcribed by the AI service, and the transcript is
// the document's text.

var mp3Bytes = []byte("ID3 pretend this is a recording")

func TestASignedInUserCanSummarizeARecordingAndItIsKeptToPlayBack(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	for _, name := range []string{"weekly sync.mp3", "Call.M4A", "voice.wav", "talk.ogg", "memo.flac", "clip.webm", "screen.mp4"} {
		if r := cl.upload("/summarize", "file", map[string][]byte{name: mp3Bytes}); r.Status != http.StatusOK {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 7 {
		t.Errorf("documents = %d, want 7", n)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 7 {
		t.Errorf("each recording is kept to be played back, found %d files", n)
	}
}

func TestRecordingsNeedAnAccountEvenOnThePublicRoutes(t *testing.T) {
	a := newApp(t)
	anon := a.newClient()

	r := anon.upload("/public/summarize", "file", map[string][]byte{"meeting.mp3": mp3Bytes})
	if r.Status != http.StatusUnauthorized || !strings.Contains(r.Str("error"), "Sign in") {
		t.Errorf("public single: %d %s", r.Status, r.Body)
	}
	r = anon.upload("/public/summarize-multiple", "files", map[string][]byte{"a.pdf": pdfBytes, "meeting.mp3": mp3Bytes})
	if r.Status != http.StatusUnauthorized {
		t.Errorf("public multiple with a recording: %d %s", r.Status, r.Body)
	}
	if n := a.ai.calls.Load(); n != 0 {
		t.Errorf("a refused recording must not reach the AI service (it was called %d times)", n)
	}
	// documents still work without an account
	if r := anon.upload("/public/summarize", "file", map[string][]byte{"x.pdf": pdfBytes}); r.Status != http.StatusOK {
		t.Errorf("a PDF without an account: %d", r.Status)
	}
}

func TestRecordingsAndDocumentsCanBeCombined(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	r := cl.upload("/summarize-multiple", "files", map[string][]byte{"agenda.pdf": pdfBytes, "sync.mp3": mp3Bytes})
	if r.Status != http.StatusOK {
		t.Fatalf("combined: %d %s", r.Status, r.Body)
	}
	files := filesOf(listedDocuments(t, cl)[0])
	if len(files) != 2 {
		t.Errorf("the PDF and the recording are both kept: %v", files)
	}
}

func TestEachKindOfFileHasItsOwnSizeLimit(t *testing.T) {
	t.Setenv("MAX_AUDIO_MB", "2")
	a := newApp(t)
	cl, _ := a.newUser()

	big := func(n int) []byte { return bytes.Repeat([]byte("x"), n) }
	// A PDF over 10 MB is refused even though recordings may be bigger.
	r := cl.upload("/summarize", "file", map[string][]byte{"huge.pdf": big(10<<20 + 100<<10)})
	if r.Status != http.StatusRequestEntityTooLarge || !strings.Contains(r.Str("error"), "max 10 MB") {
		t.Errorf("a big PDF: %d %s", r.Status, r.Body)
	}
	// A recording over the audio limit is refused, one under it is not.
	r = cl.upload("/summarize", "file", map[string][]byte{"long.mp3": big(2<<20 + 5)})
	if r.Status != http.StatusRequestEntityTooLarge || !strings.Contains(r.Str("error"), "max 2 MB for a recording") {
		t.Errorf("a big recording: %d %s", r.Status, r.Body)
	}
	if r := cl.upload("/summarize", "file", map[string][]byte{"ok.mp3": big(1 << 20)}); r.Status != http.StatusOK {
		t.Errorf("a recording under the limit: %d %s", r.Status, r.Body)
	}
	r = cl.upload("/summarize-multiple", "files", map[string][]byte{"long.mp3": big(2<<20 + 5), "a.pdf": pdfBytes})
	if r.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("a big recording among several files: %d %s", r.Status, r.Body)
	}
	// the public route keeps its small limit for documents
	anon := a.newClient()
	if r := anon.upload("/public/summarize", "file", map[string][]byte{"huge.pdf": big(12 << 20)}); r.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("a big PDF on the public route: %d", r.Status)
	}
}

func TestTheRecordingLimitDefaultsTo25MBAndIgnoresNonsense(t *testing.T) {
	for _, value := range []string{"", "abc", "0", "-4"} {
		t.Setenv("MAX_AUDIO_MB", value)
		if got := controllers.MaxAudioBytes(); got != 25<<20 {
			t.Errorf("MAX_AUDIO_MB=%q -> %d bytes, want 25 MB", value, got)
		}
	}
	t.Setenv("MAX_AUDIO_MB", "100")
	if got := controllers.MaxAudioBytes(); got != 100<<20 {
		t.Errorf("MAX_AUDIO_MB=100 -> %d bytes", got)
	}
}

func TestOtherFileTypesAreStillRefusedAndTheMessageNamesAudio(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	r := cl.upload("/summarize", "file", map[string][]byte{"notes.txt": []byte("hello")})
	if r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "audio") {
		t.Errorf("%d %s", r.Status, r.Body)
	}
}

func TestTheOwnerCanReadTheTranscriptAndNobodyElseCan(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()

	// A streamed summary stores the text the AI service sends back (a transcript, for a recording).
	if r := alice.upload("/summarize?stream=true", "file", map[string][]byte{"sync.mp3": mp3Bytes}); r.Status != http.StatusOK {
		t.Fatalf("summarize: %d %s", r.Status, r.Body)
	}
	id := int(listedDocuments(t, alice)[0]["ID"].(float64))

	r := alice.get(docPath(id, "/text"))
	if r.Status != http.StatusOK || r.Str("text") != "the source text" {
		t.Fatalf("owner: %d %s", r.Status, r.Body)
	}
	if !strings.Contains(r.Header.Get("Cache-Control"), "no-store") {
		t.Errorf("the text must not be cached: %q", r.Header.Get("Cache-Control"))
	}
	if r := bob.get(docPath(id, "/text")); r.Status != http.StatusNotFound {
		t.Errorf("another user: %d", r.Status)
	}
	if r := a.newClient().get(docPath(id, "/text")); r.Status != http.StatusUnauthorized {
		t.Errorf("signed out: %d", r.Status)
	}
	if r := alice.get("/documents/abc/text"); r.Status != http.StatusBadRequest {
		t.Errorf("a bad id: %d", r.Status)
	}
	if r := alice.get(docPath(id+999, "/text")); r.Status != http.StatusNotFound {
		t.Errorf("a missing document: %d", r.Status)
	}
}

func TestADocumentWithoutStoredTextHasNoTextToShow(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	a.db.Exec("INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content) SELECT now(), now(), 'old.pdf', 's', id, false FROM users")
	id := int(listedDocuments(t, cl)[0]["ID"].(float64))
	if r := cl.get(docPath(id, "/text")); r.Status != http.StatusNotFound {
		t.Errorf("%d %s", r.Status, r.Body)
	}
}

func TestAShareLinkNeverCarriesTheTranscript(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	if r := cl.upload("/summarize?stream=true", "file", map[string][]byte{"sync.mp3": mp3Bytes}); r.Status != http.StatusOK {
		t.Fatalf("summarize: %d", r.Status)
	}
	id := int(listedDocuments(t, cl)[0]["ID"].(float64))
	token := cl.post(docPath(id, "/share"), nil).Str("token")
	if token == "" {
		t.Fatal("no share token")
	}
	r := a.newClient().get("/shared/" + token)
	if r.Status != http.StatusOK || strings.Contains(string(r.Body), "the source text") {
		t.Errorf("a shared summary must not include the source text: %d %s", r.Status, r.Body)
	}
}

func fileURL(docID int, file map[string]any) string {
	return docPath(docID, "/files/"+strconv.Itoa(int(file["id"].(float64))))
}

func TestAKeptRecordingIsServedToItsOwnerAsWhatItIs(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	if r := alice.upload("/summarize-multiple", "files", map[string][]byte{"agenda.pdf": pdfBytes, "weekly sync.mp3": mp3Bytes}); r.Status != http.StatusOK {
		t.Fatalf("upload: %d %s", r.Status, r.Body)
	}
	doc := listedDocuments(t, alice)[0]
	id := int(doc["ID"].(float64))
	types := map[string]string{"agenda.pdf": "application/pdf", "weekly sync.mp3": "audio/mpeg"}
	for _, file := range filesOf(doc) {
		name := file["name"].(string)
		r := alice.get(fileURL(id, file))
		if r.Status != http.StatusOK || r.Header.Get("Content-Type") != types[name] {
			t.Errorf("%s: %d as %q, want %q", name, r.Status, r.Header.Get("Content-Type"), types[name])
		}
		if r.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(r.Header.Get("Content-Disposition"), "inline") {
			t.Errorf("%s must never be sniffed: %v", name, r.Header)
		}
		if name == "weekly sync.mp3" && !bytes.Equal(r.Body, mp3Bytes) {
			t.Errorf("the recording must come back byte for byte")
		}
		if r := bob.get(fileURL(id, file)); r.Status != http.StatusNotFound {
			t.Errorf("%s must not be served to someone else: %d", name, r.Status)
		}
		if r := a.newClient().get(fileURL(id, file)); r.Status != http.StatusUnauthorized {
			t.Errorf("%s must not be served when signed out: %d", name, r.Status)
		}
	}
}

func TestADeletedDocumentTakesItsRecordingWithIt(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	cl.upload("/summarize", "file", map[string][]byte{"sync.mp3": mp3Bytes})
	id := int(listedDocuments(t, cl)[0]["ID"].(float64))
	if r := cl.delete(docPath(id, ""), nil); r.Status != http.StatusOK && r.Status != http.StatusNoContent {
		t.Fatalf("delete: %d", r.Status)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("the recording must be deleted with its document, found %d", n)
	}
}

func TestARecordingOverTheStorageCapIsNotKeptButIsStillSummarized(t *testing.T) {
	t.Setenv("STORED_FILES_MB_PER_USER", "1")
	a := newApp(t)
	cl, _ := a.newUser()
	big := bytes.Repeat([]byte("x"), 2<<20)
	if r := cl.upload("/summarize", "file", map[string][]byte{"long.mp3": big}); r.Status != http.StatusOK {
		t.Fatalf("summarize: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 1 {
		t.Errorf("the summary is saved anyway: %d", n)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_files"); n != 0 {
		t.Errorf("a recording past the cap is not kept, found %d", n)
	}
}

func TestACitedPassageOfARecordingIsNeverOfferedToThePDFViewer(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	if r := cl.upload("/summarize?stream=true", "file", map[string][]byte{"sync.mp3": mp3Bytes}); r.Status != http.StatusOK {
		t.Fatalf("summarize: %d", r.Status)
	}
	r := askLibrary(cl, "What does the source text say?")
	if r.Status != http.StatusOK {
		t.Fatalf("ask: %d %s", r.Status, r.Body)
	}
	sources := r.JSON()["sources"].([]any)
	if len(sources) == 0 {
		t.Fatalf("the transcript should be found: %s", r.Body)
	}
	for _, s := range sources {
		if s.(map[string]any)["fileId"] != nil {
			t.Errorf("a recording has no page to open in a PDF viewer: %v", s)
		}
	}
}
