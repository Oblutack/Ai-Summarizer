package server

import (
	"ai-summarizer/go-api/controllers"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// Searching by meaning: the fake AI service puts "cat", "dog" and "pet" in one bucket, so a question
// about pets is close to a passage about cats and dogs although they share no word.

const (
	petsPassage = "Cats and dogs are not allowed without written permission from the landlord."
	rentPassage = "The monthly rent is 950 euros and is due on the first of every month."
)

func embeddedCount(t *testing.T, a *app, model string) int {
	t.Helper()
	return count(t, a.db, "SELECT count(*) FROM document_passages WHERE embedding IS NOT NULL AND embedding_model = ?", model)
}

func TestSavedDocumentsAreEmbedded(t *testing.T) {
	a := newApp(t)
	a.ai.embedOn.Store(true)
	cl, _ := a.newUser()
	saveText(t, cl, petsPassage+"\n\n"+rentPassage)

	if n := embeddedCount(t, a, "fake-model"); n != 2 {
		t.Errorf("both passages should be embedded when saved, found %d", n)
	}
}

func TestAQuestionFindsAPassageByMeaning(t *testing.T) {
	a := newApp(t)
	a.ai.embedOn.Store(true)
	cl, _ := a.newUser()
	saveText(t, cl, petsPassage)
	saveText(t, cl, rentPassage)
	saveText(t, cl, "The garden has two apple trees and a compost heap in the corner.")

	r := askLibrary(cl, "Can I keep a pet?") // the document says cats and dogs, never "pet"
	if r.Status != http.StatusOK {
		t.Fatalf("ask: %d %s", r.Status, r.Body)
	}
	passages := sentPassages(t, a)
	if len(passages) == 0 || passages[0]["text"] != petsPassage {
		t.Fatalf("the pets passage should come first: %v", passages)
	}
	for _, p := range passages {
		if p["text"] == "The garden has two apple trees and a compost heap in the corner." {
			t.Errorf("an unrelated passage must not be sent: %v", passages)
		}
	}
}

func TestWithoutEmbeddingsThatQuestionFindsNothing(t *testing.T) {
	// The same library and question as above, to show it is the embeddings that find the passage.
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, petsPassage)
	saveText(t, cl, rentPassage)

	r := askLibrary(cl, "Can I keep a pet?")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 0 {
		t.Fatalf("keyword search cannot know that pets means cats and dogs: %d %s", r.Status, r.Body)
	}
}

func TestKeywordSearchStillAnswersWhenEmbeddingsAreDown(t *testing.T) {
	a := newApp(t) // embeddings answer 503
	cl, _ := a.newUser()
	saveText(t, cl, rentPassage)
	saveText(t, cl, "Apples are crisp and grown in orchards across the valley.")

	r := askLibrary(cl, "How much is the monthly rent?")
	if r.Status != http.StatusOK {
		t.Fatalf("ask: %d %s", r.Status, r.Body)
	}
	passages := sentPassages(t, a)
	if len(passages) != 1 || passages[0]["text"] != rentPassage {
		t.Fatalf("keyword search should still find the rent: %v", passages)
	}
}

func TestEmbeddingsAreNotRetriedForAWhileAfterAFailure(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, rentPassage) // the save tries once and fails
	before := a.ai.embedCalls.Load()
	askLibrary(cl, "How much is the monthly rent?")
	askLibrary(cl, "And when is the rent due?")
	if after := a.ai.embedCalls.Load(); after != before {
		t.Errorf("a failed service should be left alone for a while, but it was asked %d more times", after-before)
	}

	controllers.ResetEmbeddingBackoff()
	a.ai.embedOn.Store(true)
	askLibrary(cl, "How much is the monthly rent?")
	if a.ai.embedCalls.Load() == before {
		t.Error("after the pause the service should be asked again")
	}
}

func TestOlderPassagesGetEmbeddedWhenAQuestionIsAsked(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, petsPassage) // saved while embeddings were off
	if n := embeddedCount(t, a, "fake-model"); n != 0 {
		t.Fatalf("setup: %d", n)
	}

	controllers.ResetEmbeddingBackoff()
	a.ai.embedOn.Store(true)
	askLibrary(cl, "Can I keep a pet?")
	if n := embeddedCount(t, a, "fake-model"); n != 1 {
		t.Errorf("the passage should be embedded by the question, found %d", n)
	}
	if passages := sentPassages(t, a); len(passages) != 1 || passages[0]["text"] != petsPassage {
		t.Errorf("and then found by meaning: %v", passages)
	}
}

func TestVectorsOfAReplacedModelAreMadeAgain(t *testing.T) {
	a := newApp(t)
	a.ai.embedOn.Store(true)
	cl, _ := a.newUser()
	saveText(t, cl, petsPassage)
	if n := embeddedCount(t, a, "fake-model"); n != 1 {
		t.Fatalf("setup: %d", n)
	}

	a.ai.embedModel.Store("better-model")
	askLibrary(cl, "Can I keep a pet?")
	if n := embeddedCount(t, a, "better-model"); n != 1 {
		t.Errorf("the passage should be embedded again with the new model, found %d", n)
	}
	if n := embeddedCount(t, a, "fake-model"); n != 0 {
		t.Errorf("no vector of the old model should remain, found %d", n)
	}
	if passages := sentPassages(t, a); len(passages) != 1 {
		t.Errorf("and search keeps working: %v", passages)
	}
}

func TestAQuestionAboutNothingInTheLibraryFindsNothing(t *testing.T) {
	a := newApp(t)
	a.ai.embedOn.Store(true)
	cl, _ := a.newUser()
	saveText(t, cl, petsPassage)
	saveText(t, cl, rentPassage)

	r := askLibrary(cl, "Explain quantum chromodynamics")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 0 {
		t.Fatalf("nothing is close enough: %d %s", r.Status, r.Body)
	}
	if a.ai.lastAsk.Load() != nil {
		t.Error("no passages means the model is never called")
	}
}

func TestSearchByMeaningNeverReachesAnotherUsersPassages(t *testing.T) {
	a := newApp(t)
	a.ai.embedOn.Store(true)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	saveText(t, alice, petsPassage)
	saveText(t, bob, "Bob keeps notes about the quarterly budget and the office plants.")

	r := askLibrary(bob, "Can I keep a pet?")
	if r.Status != http.StatusOK {
		t.Fatalf("ask: %d", r.Status)
	}
	for _, source := range r.JSON()["sources"].([]any) {
		if strings.Contains(source.(map[string]any)["text"].(string), "Cats and dogs") {
			t.Errorf("Bob must never see Alice's passage: %v", source)
		}
	}
	if raw, _ := a.ai.lastAsk.Load().([]byte); strings.Contains(string(raw), "Cats and dogs") {
		t.Error("Alice's passage must not be sent to the model on Bob's behalf")
	}
}

func TestDeletedDocumentsAreNotFoundByMeaning(t *testing.T) {
	a := newApp(t)
	a.ai.embedOn.Store(true)
	cl, _ := a.newUser()
	id := saveText(t, cl, petsPassage)
	if r := cl.delete("/documents/"+strconv.Itoa(id), nil); r.Status != http.StatusOK {
		t.Fatalf("delete: %d %s", r.Status, r.Body)
	}

	r := askLibrary(cl, "Can I keep a pet?")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 0 {
		t.Fatalf("a deleted document must not come back: %d %s", r.Status, r.Body)
	}
}
