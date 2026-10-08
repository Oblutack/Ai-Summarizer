package controllers

import (
	"ai-summarizer/go-api/initializers"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Searching by meaning. Every passage has a vector made by the AI service; a question is turned into
// a vector the same way, and the passages whose vectors point the same way are the ones about the
// same thing, whatever words they use. This works next to the keyword search (see searchLibrary), and
// everything falls back to keyword search when the AI service has no embeddings to give.
const (
	embedBatch               = 64  // texts per call to the AI service (its limit is 128)
	embedAtSaveLimit         = 256 // passages embedded while a document is saved; the rest wait for a question
	embedBackfillPerQuestion = 128 // older passages embedded per question, so a library fills in gradually
	semanticScanLimit        = 20_000
	embedTimeout             = 20 * time.Second
	embeddingBackoff         = 30 * time.Second // after a failure, don't ask again for this long
	semanticWeight           = 1.0              // how much the ranking by meaning counts when the two are combined
	keywordWeight            = 0.15             // and the ranking by keyword (it settles ties and rescues exact terms)
	rankFusionK              = 10
	kindQuery                = "query"
	kindPassage              = "passage"
)

var errEmbeddingsPaused = errors.New("embeddings are paused after a recent failure")

// embeddingsDownUntil is when the AI service may be asked for embeddings again (unix nanoseconds).
var embeddingsDownUntil atomic.Int64

// ResetEmbeddingBackoff forgets a recent failure. Used by tests.
func ResetEmbeddingBackoff() { embeddingsDownUntil.Store(0) }

type embeddedTexts struct {
	Model    string
	MinScore float64
	Vectors  [][]byte // float32 little-endian, unit length
}

// embedTexts asks the AI service for one vector per text. After a failure it answers errEmbeddingsPaused
// for a while instead of making every request wait for a service that is not giving embeddings.
func embedTexts(ctx context.Context, requestID string, texts []string, kind string) (*embeddedTexts, error) {
	if time.Now().UnixNano() < embeddingsDownUntil.Load() {
		return nil, errEmbeddingsPaused
	}
	result, err := requestEmbeddings(ctx, requestID, texts, kind)
	if err != nil && ctx.Err() == nil {
		embeddingsDownUntil.Store(time.Now().Add(embeddingBackoff).UnixNano())
		slog.Warn("embeddings are not available; searching by keyword only for a while", "error", err, "request_id", requestID)
	}
	return result, err
}

func requestEmbeddings(ctx context.Context, requestID string, texts []string, kind string) (*embeddedTexts, error) {
	ctx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()
	payload, err := json.Marshal(gin.H{"texts": texts, "kind": kind})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, aiServiceURL("/embed"), bytes.NewReader(payload))
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
		Model    string   `json:"model"`
		Dim      int      `json:"dim"`
		MinScore float64  `json:"minScore"`
		Vectors  []string `json:"vectors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Model == "" || out.Dim <= 0 || len(out.Vectors) != len(texts) {
		return nil, errors.New("the AI service sent no usable embeddings")
	}
	result := &embeddedTexts{Model: out.Model, MinScore: out.MinScore, Vectors: make([][]byte, len(out.Vectors))}
	for i, v := range out.Vectors {
		raw, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(raw) != out.Dim*4 {
			return nil, errors.New("the AI service sent a malformed embedding")
		}
		result.Vectors[i] = raw
	}
	return result, nil
}

// dot is the cosine of two unit-length vectors given as float32 bytes. Vectors of different lengths
// (made by different models) never match.
func dot(a, b []byte) float64 {
	if len(a) != len(b) || len(a)%4 != 0 {
		return math.Inf(-1)
	}
	var sum float64
	for i := 0; i < len(a); i += 4 {
		x := math.Float32frombits(binary.LittleEndian.Uint32(a[i:]))
		y := math.Float32frombits(binary.LittleEndian.Uint32(b[i:]))
		sum += float64(x) * float64(y)
	}
	return sum
}

type passageText struct {
	ID   uint
	Text string
}

// storeEmbeddings saves vectors for passages, all or none.
func storeEmbeddings(ctx context.Context, model string, passages []passageText, vectors [][]byte) error {
	return initializers.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, p := range passages {
			if err := tx.Exec("UPDATE document_passages SET embedding = ?, embedding_model = ? WHERE id = ?", vectors[i], model, p.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// embedAndStore embeds passages in batches and stores the vectors. It stops at the first failure and
// reports it; what was stored before stays.
func embedAndStore(ctx context.Context, requestID string, passages []passageText) (model string, err error) {
	for start := 0; start < len(passages); start += embedBatch {
		batch := passages[start:min(start+embedBatch, len(passages))]
		texts := make([]string, len(batch))
		for i, p := range batch {
			texts[i] = p.Text
		}
		result, err := embedTexts(ctx, requestID, texts, kindPassage)
		if err != nil {
			return model, err
		}
		if err := storeEmbeddings(ctx, result.Model, batch, result.Vectors); err != nil {
			return model, err
		}
		model = result.Model
	}
	return model, nil
}

// embedNewPassages gives the passages of a document that was just indexed their vectors. It is
// best effort: without embeddings the document is still found by keyword, and what is left over is
// embedded when a question is asked.
func embedNewPassages(ctx context.Context, requestID string, passages []passageText) {
	if len(passages) == 0 {
		return
	}
	if len(passages) > embedAtSaveLimit {
		passages = passages[:embedAtSaveLimit]
	}
	if _, err := embedAndStore(ctx, requestID, passages); err != nil && !errors.Is(err, errEmbeddingsPaused) {
		slog.Warn("embedding a new document failed; it is found by keyword until embedded", "error", err, "request_id", requestID)
	}
}

// ensureEmbedded embeds some of the user's passages that have no vector from the current model (older
// documents, ones whose embedding failed, or ones made by a model that has since been replaced), so a
// library gains search by meaning gradually without any one question waiting for all of it.
//
// With docIDs set, only the passages of those documents are looked at (a question about one collection
// should not spend its allowance on documents outside it).
func ensureEmbedded(ctx context.Context, requestID string, userID uint, model string, docIDs []uint) {
	var pending []passageText
	scope, args := scopeClause(docIDs, userID, model)
	err := initializers.DB.WithContext(ctx).Raw(`
		SELECT p.id, p.text FROM document_passages p
		JOIN documents d ON d.id = p.document_id AND d.deleted_at IS NULL
		WHERE p.user_id = ? AND (p.embedding IS NULL OR p.embedding_model <> ?)`+scope+`
		ORDER BY p.id DESC LIMIT ?`, append(args, embedBackfillPerQuestion)...).Scan(&pending).Error
	if err != nil {
		slog.Error("looking for passages to embed failed", "user_id", userID, "error", err, "request_id", requestID)
		return
	}
	if len(pending) == 0 {
		return
	}
	if _, err := embedAndStore(ctx, requestID, pending); err != nil && !errors.Is(err, errEmbeddingsPaused) {
		slog.Warn("embedding older passages failed", "user_id", userID, "error", err, "request_id", requestID)
	}
}

// scopeClause restricts a passage query to a set of documents (nil means all of the user's), and returns
// the query arguments so far: the user, the model, then the documents when there are any.
func scopeClause(docIDs []uint, userID uint, model string) (string, []any) {
	args := []any{userID, model}
	if docIDs == nil {
		return "", args
	}
	return " AND p.document_id IN ?", append(args, docIDs)
}

// semanticSearch returns the ids of the user's passages whose meaning is closest to the question,
// best first, keeping only those at least minScore similar.
func semanticSearch(ctx context.Context, userID uint, model string, question []byte, minScore float64, docIDs []uint) ([]uint, error) {
	scope, args := scopeClause(docIDs, userID, model)
	rows, err := initializers.DB.WithContext(ctx).Raw(`
		SELECT p.id, p.embedding FROM document_passages p
		JOIN documents d ON d.id = p.document_id AND d.deleted_at IS NULL
		WHERE p.user_id = ? AND p.embedding_model = ? AND p.embedding IS NOT NULL`+scope+`
		ORDER BY p.id DESC LIMIT ?`, append(args, semanticScanLimit)...).Rows()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return rankBySimilarity(rows, question, minScore)
}

func rankBySimilarity(rows *sql.Rows, question []byte, minScore float64) ([]uint, error) {
	type scored struct {
		id    uint
		score float64
	}
	var found []scored
	for rows.Next() {
		var id uint
		var vector []byte
		if err := rows.Scan(&id, &vector); err != nil {
			return nil, err
		}
		if score := dot(question, vector); score >= minScore {
			found = append(found, scored{id, score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	ids := make([]uint, 0, min(len(found), candidateRows))
	for _, f := range found {
		if len(ids) == candidateRows {
			break
		}
		ids = append(ids, f.id)
	}
	return ids, nil
}

// fuseRankings combines the ranking by meaning with the ranking by keyword (reciprocal rank fusion):
// a passage near the top of either list rises, and one near the top of both rises most. Meaning leads
// because it was clearly the better ranking on a test set; keyword mostly settles ties.
func fuseRankings(bySemantic, byKeyword []uint) []uint {
	score := map[uint]float64{}
	for pos, id := range bySemantic {
		score[id] += semanticWeight / float64(rankFusionK+pos+1)
	}
	for pos, id := range byKeyword {
		score[id] += keywordWeight / float64(rankFusionK+pos+1)
	}
	ids := make([]uint, 0, len(score))
	for id := range score {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if score[ids[i]] != score[ids[j]] {
			return score[ids[i]] > score[ids[j]]
		}
		return ids[i] < ids[j]
	})
	return ids
}
