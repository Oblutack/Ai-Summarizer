package mailer

import (
	"strings"
	"testing"
)

func TestSummaryEmailShowsTheSummaryAsText(t *testing.T) {
	msg := SummaryEmail("ana@example.com", "Lease\nnotes", "## Overview\n<script>alert(1)</script> & more")
	if msg.To != "ana@example.com" || msg.Subject != "Summary: Lease notes" {
		t.Errorf("addressing: %+v", msg)
	}
	if !strings.Contains(msg.Text, "## Overview") || !strings.Contains(msg.Text, "Lease notes") {
		t.Errorf("text: %q", msg.Text)
	}
	if strings.Contains(msg.HTML, "<script>") || !strings.Contains(msg.HTML, "&lt;script&gt;") || !strings.Contains(msg.HTML, "&amp; more") {
		t.Errorf("nothing in a summary may become markup: %q", msg.HTML)
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		t.Errorf("a subject is one line: %q", msg.Subject)
	}
}
