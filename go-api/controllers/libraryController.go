package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Asking a question across all of a user's documents works in three steps: every saved document is
// cut into page-sized passages and indexed (at save time, or on first use for older documents), the
// best passages for the question are found, and the AI service answers from those passages and cites
// them.
//
// Finding passages combines two rankings: by meaning (each passage has a vector from a small local
// model in the AI service, see embeddings.go) and by keyword (Postgres full-text search with English
// stemming). If the AI service has no embeddings to give, the keyword ranking is used alone.
const (
	maxLibraryHits       = 10    // passages sent to the model
	maxPerDocument       = 4     // so one long document cannot crowd out the others
	maxLibraryChars      = 14000 // size of the excerpts sent to the model
	candidateRows        = 60    // rows fetched before the limits above are applied
	indexBackfillBatch   = 25    // older documents indexed per question
	indexTimeout         = 30 * time.Second
	maxQueryTerms        = 12
	prefixMinLength      = 4 // words at least this long match by prefix
	noMatchAnswer        = "I couldn't find anything about that in your saved documents."
	noQueryTermsAnswer   = "Please ask something more specific: I couldn't find searchable words in that question."
	insertPassagesBatch  = 100
	maxPassagesPerDocRow = 2_000 // a safety bound on what one document may add to the index
)

var queryWord = regexp.MustCompile(`[\p{L}\p{N}]+`)

// ---- indexing -------------------------------------------------------------------------------

type passageFromAI struct {
	Text     string  `json:"text"`
	Page     *int    `json:"page"`
	PageEnd  *int    `json:"pageEnd"`
	Document *string `json:"document"`
}

// fetchPassages asks the AI service to cut a document's text into page-aware passages.
func fetchPassages(ctx context.Context, requestID, text string) ([]passageFromAI, error) {
	payload, err := json.Marshal(gin.H{"text": text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, aiServiceURL("/passages"), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAIHeaders(req, requestID)

	body, apiErr := callAIService(req)
	if apiErr != nil {
		return nil, fmt.Errorf("%s", apiErr.Message)
	}
	var out struct {
		Passages []passageFromAI `json:"passages"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Passages, nil
}

// indexDocument (re)builds the searchable passages of one document and marks it indexed.
func indexDocument(ctx context.Context, requestID string, document models.Document) error {
	passages, err := fetchPassages(ctx, requestID, document.Content)
	if err != nil {
		return err
	}
	if len(passages) > maxPassagesPerDocRow {
		passages = passages[:maxPassagesPerDocRow]
	}
	rows := make([]models.DocumentPassage, 0, len(passages))
	for i, p := range passages {
		if strings.TrimSpace(p.Text) == "" {
			continue
		}
		row := models.DocumentPassage{DocumentID: document.ID, UserID: document.UserID, Ord: i, Page: p.Page, PageEnd: p.PageEnd, Text: p.Text}
		if p.Document != nil {
			row.DocName = *p.Document
		}
		rows = append(rows, row)
	}
	err = initializers.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM document_passages WHERE document_id = ?", document.ID).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			if err := tx.CreateInBatches(&rows, insertPassagesBatch).Error; err != nil {
				return err
			}
		}
		return tx.Exec("UPDATE documents SET indexed_at = now() WHERE id = ?", document.ID).Error
	})
	if err != nil {
		return err
	}
	toEmbed := make([]passageText, len(rows))
	for i, r := range rows {
		toEmbed[i] = passageText{ID: r.ID, Text: r.Text}
	}
	embedNewPassages(ctx, requestID, toEmbed)
	return nil
}

// indexAfterSave indexes a document that was just saved. A failure is logged and never fails the
// request (the summary is already made); the document is simply indexed on first use instead.
func indexAfterSave(requestID string, document models.Document) {
	if !document.HasContent || document.Content == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()
	if err := indexDocument(ctx, requestID, document); err != nil {
		slog.Warn("indexing a saved document failed; it will be indexed when first searched", "document_id", document.ID, "error", err, "request_id", requestID)
	}
}

// ensureIndexed indexes some of the user's documents that have not been (older ones, or ones whose
// indexing failed), so a question covers them too.
func ensureIndexed(ctx context.Context, requestID string, userID uint) {
	var pending []models.Document
	err := initializers.DB.WithContext(ctx).
		Select("id", "user_id", "content", "has_content").
		Where("user_id = ? AND has_content = true AND indexed_at IS NULL", userID).
		Order("id DESC").Limit(indexBackfillBatch).Find(&pending).Error
	if err != nil {
		slog.Error("looking for unindexed documents failed", "user_id", userID, "error", err, "request_id", requestID)
		return
	}
	for _, d := range pending {
		if d.Content == "" {
			continue
		}
		if err := indexDocument(ctx, requestID, d); err != nil {
			slog.Warn("indexing an older document failed", "document_id", d.ID, "error", err, "request_id", requestID)
		}
	}
}

// ---- searching ------------------------------------------------------------------------------

// searchQuery turns a question into an OR-query of its words, in to_tsquery syntax. Only letters and
// digits survive (plus the :* we add ourselves), so user text can never form query syntax. Common words
// are dropped by the database.
func searchQuery(question string) string {
	seen := map[string]bool{}
	var terms []string
	for _, w := range queryWord.FindAllString(strings.ToLower(question), -1) {
		if len([]rune(w)) < 2 || seen[w] {
			continue
		}
		seen[w] = true
		// Longer words also match words that begin with them (rent finds rental), which makes up a little
		// for the search being by keyword and not by meaning. Short words stay exact: "an:*" would match half the language.
		if len([]rune(w)) >= prefixMinLength {
			w += ":*"
		}
		terms = append(terms, w)
		if len(terms) == maxQueryTerms {
			break
		}
	}
	return strings.Join(terms, " | ")
}

type libraryHit struct {
	ID         uint
	DocumentID uint
	Page       *int
	PageEnd    *int
	DocName    string
	Text       string
	Title      string
	CreatedAt  time.Time
}

const searchSQL = `
WITH q AS (SELECT to_tsquery(@config, @query) AS query)
SELECT p.id, p.document_id, p.page, p.page_end, p.doc_name, p.text, d.filename AS title, d.created_at AS created_at
FROM document_passages p
JOIN documents d ON d.id = p.document_id AND d.deleted_at IS NULL
CROSS JOIN q
WHERE p.user_id = @user AND %[1]s @@ q.query
ORDER BY ts_rank_cd(%[1]s, q.query) DESC, p.id
LIMIT @limit`

// searchPassages finds the passages of the user's own documents that best match the question. English
// stemming is tried first; if nothing matches, a plain word match is tried (other languages).
func searchPassages(ctx context.Context, userID uint, query string) ([]libraryHit, error) {
	run := func(config, vector string) ([]libraryHit, error) {
		var hits []libraryHit
		err := initializers.DB.WithContext(ctx).Raw(fmt.Sprintf(searchSQL, vector),
			sql.Named("config", config), sql.Named("query", query), sql.Named("user", userID), sql.Named("limit", candidateRows),
		).Scan(&hits).Error
		return hits, err
	}
	hits, err := run("english", "p.tsv")
	if err != nil || len(hits) > 0 {
		return hits, err
	}
	return run("simple", "to_tsvector('simple', p.text)")
}

// loadHits fetches passages by id (only the user's own), in the order of ids.
func loadHits(ctx context.Context, userID uint, ids []uint) ([]libraryHit, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var hits []libraryHit
	err := initializers.DB.WithContext(ctx).Raw(`
		SELECT p.id, p.document_id, p.page, p.page_end, p.doc_name, p.text, d.filename AS title, d.created_at AS created_at
		FROM document_passages p
		JOIN documents d ON d.id = p.document_id AND d.deleted_at IS NULL
		WHERE p.user_id = ? AND p.id IN ?`, userID, ids).Scan(&hits).Error
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]libraryHit, len(hits))
	for _, h := range hits {
		byID[h.ID] = h
	}
	ordered := make([]libraryHit, 0, len(ids))
	for _, id := range ids {
		if h, ok := byID[id]; ok {
			ordered = append(ordered, h)
		}
	}
	return ordered, nil
}

// questionVector is the question as the AI service understands it, for searching by meaning.
type questionVector struct {
	model    string
	minScore float64
	vector   []byte
}

// searchLibrary finds the passages that best answer a question, best first: by meaning and by keyword
// combined when there is a question vector, else by keyword alone.
func searchLibrary(ctx context.Context, userID uint, keywordQuery string, question *questionVector) ([]libraryHit, error) {
	byKeyword, err := searchPassages(ctx, userID, keywordQuery)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return byKeyword, nil
	}
	bySemantic, err := semanticSearch(ctx, userID, question.model, question.vector, question.minScore)
	if err != nil {
		// Keyword search still works, so a failure here costs quality and not the answer.
		slog.Error("searching by meaning failed; using keyword search", "user_id", userID, "error", err)
		return byKeyword, nil
	}
	if len(bySemantic) == 0 {
		return byKeyword, nil
	}

	keywordIDs := make([]uint, len(byKeyword))
	known := make(map[uint]libraryHit, len(byKeyword))
	for i, h := range byKeyword {
		keywordIDs[i] = h.ID
		known[h.ID] = h
	}
	fused := fuseRankings(bySemantic, keywordIDs)
	if len(fused) > candidateRows {
		fused = fused[:candidateRows]
	}
	var missing []uint
	for _, id := range fused {
		if _, ok := known[id]; !ok {
			missing = append(missing, id)
		}
	}
	loaded, err := loadHits(ctx, userID, missing)
	if err != nil {
		return nil, err
	}
	for _, h := range loaded {
		known[h.ID] = h
	}
	hits := make([]libraryHit, 0, len(fused))
	for _, id := range fused {
		if h, ok := known[id]; ok {
			hits = append(hits, h)
		}
	}
	return hits, nil
}

// limitHits keeps the best passages within the per-document, count and size limits.
func limitHits(in []libraryHit) []libraryHit {
	perDoc := map[uint]int{}
	var out []libraryHit
	chars := 0
	for _, h := range in {
		if len(out) >= maxLibraryHits || perDoc[h.DocumentID] >= maxPerDocument {
			continue
		}
		if chars+len(h.Text) > maxLibraryChars && len(out) > 0 {
			continue
		}
		perDoc[h.DocumentID]++
		chars += len(h.Text)
		out = append(out, h)
	}
	return out
}

// label is how the model and the user are told which document a passage is from: the file name for
// combined PDFs, else the document's title.
func (h libraryHit) label() string {
	if h.DocName != "" {
		return h.DocName
	}
	return h.Title
}

// ---- the endpoint ---------------------------------------------------------------------------

// librarySource is a passage an answer cites, with where it lives so the client can open it.
type librarySource struct {
	ID            int       `json:"id"`
	Text          string    `json:"text"`
	Page          *int      `json:"page"`
	PageEnd       *int      `json:"pageEnd"`
	Document      string    `json:"document,omitempty"`
	DocumentID    uint      `json:"documentId"`
	DocumentTitle string    `json:"documentTitle"`
	SavedAt       time.Time `json:"savedAt"`
	FileID        *uint     `json:"fileId"`
	FileName      string    `json:"fileName,omitempty"`
}

// AskLibrary answers a question from all of the signed-in user's saved documents.
func AskLibrary(c *gin.Context) {
	user := middleware.CurrentUser(c)
	requestID := middleware.RequestIDFrom(c)

	var body chatRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body."})
		return
	}
	if apiErr := body.validate(); apiErr != nil {
		middleware.RefundQuota(c)
		apiErr.send(c)
		return
	}

	query := searchQuery(body.Question)
	if query == "" {
		middleware.RefundQuota(c) // no model call was made
		c.JSON(http.StatusOK, gin.H{"answer": noQueryTermsAnswer, "sources": []librarySource{}})
		return
	}

	ensureIndexed(c.Request.Context(), requestID, user.ID)

	// Searching by meaning is a bonus: when the AI service gives no embeddings, only keywords are used.
	var question *questionVector
	if embedded, err := embedTexts(c.Request.Context(), requestID, []string{body.Question}, kindQuery); err == nil {
		question = &questionVector{model: embedded.Model, minScore: embedded.MinScore, vector: embedded.Vectors[0]}
		ensureEmbedded(c.Request.Context(), requestID, user.ID, embedded.Model)
	}

	found, err := searchLibrary(c.Request.Context(), user.ID, query, question)
	if err != nil {
		middleware.RefundQuota(c)
		slog.Error("searching documents failed", "user_id", user.ID, "error", err, "request_id", requestID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search your documents."})
		return
	}
	hits := limitHits(found)
	if len(hits) == 0 {
		middleware.RefundQuota(c) // nothing to answer from, so no model call was made
		c.JSON(http.StatusOK, gin.H{"answer": noMatchAnswer, "sources": []librarySource{}})
		return
	}

	type askPassage struct {
		ID       int    `json:"id"`
		Text     string `json:"text"`
		Page     *int   `json:"page"`
		PageEnd  *int   `json:"pageEnd"`
		Document string `json:"document"`
	}
	passages := make([]askPassage, len(hits))
	for i, h := range hits {
		passages[i] = askPassage{ID: i + 1, Text: h.Text, Page: h.Page, PageEnd: h.PageEnd, Document: h.label()}
	}
	payload, err := json.Marshal(gin.H{"question": body.Question, "history": body.History, "passages": passages})
	if err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/ask"), bytes.NewReader(payload))
	if err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	prepareAIRequest(c, req)

	respBody, apiErr := callAIService(req)
	if apiErr != nil {
		middleware.RefundQuota(c)
		apiErr.send(c)
		return
	}
	var out struct {
		Answer  string       `json:"answer"`
		Sources []chatSource `json:"sources"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil || strings.TrimSpace(out.Answer) == "" {
		middleware.RefundQuota(c)
		c.JSON(http.StatusBadGateway, gin.H{"error": "The AI service returned an empty answer."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"answer": out.Answer, "sources": librarySources(out.Sources, hits)})
}

// librarySources describes the cited passages from our own records of what was sent, not from what
// the AI service says came back: only ids we sent can be cited, and where each lives is known here.
func librarySources(cited []chatSource, hits []libraryHit) []librarySource {
	files := originalFilesOf(hits)
	out := make([]librarySource, 0, len(cited))
	seen := map[int]bool{}
	for _, s := range cited {
		if s.ID < 1 || s.ID > len(hits) || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		h := hits[s.ID-1]
		source := librarySource{
			ID: s.ID, Text: truncateRunes(h.Text, maxSourceText), Page: h.Page, PageEnd: h.PageEnd,
			Document: h.DocName, DocumentID: h.DocumentID, DocumentTitle: h.Title, SavedAt: h.CreatedAt,
		}
		if f, ok := fileOfHit(files[h.DocumentID], h); ok {
			id := f.ID
			source.FileID, source.FileName = &id, f.Filename
		}
		out = append(out, source)
	}
	return out
}

// originalFilesOf loads the stored original PDFs (never their bytes) of the documents involved.
func originalFilesOf(hits []libraryHit) map[uint][]models.DocumentFile {
	ids := make([]uint, 0, len(hits))
	seen := map[uint]bool{}
	for _, h := range hits {
		if !seen[h.DocumentID] {
			seen[h.DocumentID] = true
			ids = append(ids, h.DocumentID)
		}
	}
	result := map[uint][]models.DocumentFile{}
	var files []models.DocumentFile
	if err := initializers.DB.Select("id", "document_id", "filename", "size_bytes").
		Where("document_id IN ?", ids).Order("id").Find(&files).Error; err != nil {
		slog.Error("loading original files failed", "error", err)
		return result
	}
	for _, f := range files {
		result[f.DocumentID] = append(result[f.DocumentID], f)
	}
	return result
}

// fileOfHit picks the stored PDF a passage belongs to: by name when several files were combined,
// else the document's only file.
func fileOfHit(files []models.DocumentFile, h libraryHit) (models.DocumentFile, bool) {
	if h.DocName != "" {
		for _, f := range files {
			if f.Filename == h.DocName {
				return f, true
			}
		}
		return models.DocumentFile{}, false
	}
	if len(files) == 1 {
		return files[0], true
	}
	return models.DocumentFile{}, false
}
