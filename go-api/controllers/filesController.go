package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

// storedFile is an uploaded PDF on its way into the database.
type storedFile struct {
	Name string
	Data []byte
}

// defaultStoredFilesMB is how much original data (PDFs and recordings) one user may keep. The summaries and chat
// keep working past it; new documents simply are not given a viewable original.
const defaultStoredFilesMB = 100

// storedFilesLimit is the per-user cap in bytes. STORED_FILES_MB_PER_USER=0 turns storing off.
func storedFilesLimit() int64 {
	mb := defaultStoredFilesMB
	if v, err := strconv.Atoi(os.Getenv("STORED_FILES_MB_PER_USER")); err == nil && v >= 0 {
		mb = v
	}
	return int64(mb) << 20
}

// storeFiles keeps the original PDFs of a saved document, unless that would take the user past
// their storage cap. Failures are logged and never fail the request: the summary is already made.
func storeFiles(userID, documentID uint, files []storedFile) {
	if len(files) == 0 {
		return
	}
	var incoming int64
	for _, f := range files {
		incoming += int64(len(f.Data))
	}

	var used int64
	if err := initializers.DB.Raw("SELECT COALESCE(SUM(size_bytes), 0) FROM document_files WHERE user_id = ?", userID).Scan(&used).Error; err != nil {
		slog.Error("checking stored file usage failed", "user_id", userID, "error", err)
		return
	}
	if used+incoming > storedFilesLimit() {
		slog.Info("original PDFs not kept: storage cap reached", "user_id", userID, "used_bytes", used, "incoming_bytes", incoming)
		return
	}

	rows := make([]models.DocumentFile, 0, len(files))
	for _, f := range files {
		rows = append(rows, models.DocumentFile{
			DocumentID: documentID, UserID: userID, Filename: f.Name, SizeBytes: int64(len(f.Data)), Content: f.Data,
		})
	}
	if err := initializers.DB.Create(&rows).Error; err != nil {
		slog.Error("storing original PDFs failed", "user_id", userID, "document_id", documentID, "error", err)
	}
}

// attachFiles fills in the Files list of each document with one query (never the file bytes).
func attachFiles(docs []models.Document) error {
	if len(docs) == 0 {
		return nil
	}
	ids := make([]uint, len(docs))
	index := make(map[uint]int, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
		index[d.ID] = i
		docs[i].Files = []models.FileInfo{} // [] in the JSON rather than null
	}
	var rows []struct {
		ID         uint
		DocumentID uint
		Filename   string
		SizeBytes  int64
	}
	if err := initializers.DB.Raw(
		"SELECT id, document_id, filename, size_bytes FROM document_files WHERE document_id IN ? ORDER BY id", ids,
	).Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		i := index[r.DocumentID]
		docs[i].Files = append(docs[i].Files, models.FileInfo{ID: r.ID, Name: r.Filename, Size: r.SizeBytes})
	}
	return nil
}

// DocumentFile sends back one original PDF of one of the signed-in user's documents. Looking it up
// by user as well as id means other people's files look like they do not exist.
func DocumentFile(c *gin.Context) {
	user := middleware.CurrentUser(c)
	documentID, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	fileID, err2 := strconv.ParseUint(c.Param("fileId"), 10, 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}

	var file models.DocumentFile
	err := initializers.DB.
		Where("id = ? AND document_id = ? AND user_id = ?", fileID, documentID, user.ID).
		First(&file).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// The bytes are shown by the app's own viewer or player, so there is nothing for a browser to execute:
	// served as what they are (a PDF or a recording), never sniffed (SecurityHeaders sets nosniff), and not
	// cached by proxies.
	c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": file.Filename}))
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, contentTypeOf(file.Filename), file.Content)
}
