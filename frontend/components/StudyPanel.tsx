"use client";
import { useCallback, useState } from "react";
import axios from "axios";
import type { QuizQuestion, StudyCard } from "../types";
import { API_URL, apiError } from "../lib/api";
import { shuffled } from "../lib/study";
import { useT } from "./I18nProvider";
import { actionButton, textButton } from "./styles";

type Kind = "flashcards" | "quiz";

interface StudyPanelProps {
  documentId: number;
}

interface Loaded {
  cards?: StudyCard[];
  questions?: QuizQuestion[];
}

// Flashcards and a quiz made from a document. Each is written once and kept, so opening it again is free.
export default function StudyPanel({ documentId }: StudyPanelProps) {
  const t = useT();
  const [kind, setKind] = useState<Kind | null>(null);
  const [loaded, setLoaded] = useState<Loaded>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(
    async (which: Kind, regenerate = false) => {
      setKind(which);
      setError("");
      if (!regenerate && (which === "flashcards" ? loaded.cards : loaded.questions)) return;
      setLoading(true);
      try {
        const response = await axios.post<{ cards?: StudyCard[]; questions?: QuizQuestion[] }>(
          `${API_URL}/documents/${documentId}/study`,
          { kind: which, regenerate }
        );
        setLoaded((prev) =>
          which === "flashcards" ? { ...prev, cards: response.data.cards } : { ...prev, questions: response.data.questions }
        );
      } catch (err) {
        setError(apiError(err, t("study.failed")));
      } finally {
        setLoading(false);
      }
    },
    [documentId, loaded, t]
  );

  return (
    <div className="mt-4 rounded-md border-2 border-ink p-3" data-testid="study">
      <div className="flex flex-wrap items-center gap-3">
        <button type="button" onClick={() => load("flashcards")} aria-pressed={kind === "flashcards"} className={actionButton}>
          {t("study.flashcards")}
        </button>
        <button type="button" onClick={() => load("quiz")} aria-pressed={kind === "quiz"} className={actionButton}>
          {t("study.quiz")}
        </button>
        {kind && !loading && (
          <button type="button" onClick={() => load(kind, true)} className={textButton}>
            {t("study.newOnes")}
          </button>
        )}
      </div>

      {!kind && <p className="mt-3 text-lg text-ink/70">{t("study.intro")}</p>}
      {loading && (
        <p className="mt-3 text-xl tracking-wider uppercase text-ink/70" role="status">
          {t("study.writing")}
        </p>
      )}
      {error && (
        <p className="mt-3 text-red-500 text-lg" role="alert">
          {error}
        </p>
      )}
      {!loading && kind === "flashcards" && loaded.cards && <Flashcards key={loaded.cards[0]?.front} cards={loaded.cards} />}
      {!loading && kind === "quiz" && loaded.questions && <Quiz key={loaded.questions[0]?.question} questions={loaded.questions} />}
    </div>
  );
}

// ---- flashcards ----------------------------------------------------------------------------------------

function Flashcards({ cards }: { cards: StudyCard[] }) {
  const t = useT();
  const [order, setOrder] = useState(() => cards.map((_, i) => i));
  const [position, setPosition] = useState(0);
  const [flipped, setFlipped] = useState(false);
  const card = cards[order[position]];

  const go = (delta: number) => {
    setPosition((p) => Math.min(order.length - 1, Math.max(0, p + delta)));
    setFlipped(false);
  };

  return (
    <div className="mt-3" data-testid="flashcards">
      <button
        type="button"
        onClick={() => setFlipped((f) => !f)}
        aria-label={flipped ? t("study.showQuestion") : t("study.showAnswer")}
        className="block w-full min-h-40 rounded-md border-2 border-ink p-6 text-center text-2xl hover:bg-ink/5"
        data-testid="flashcard"
      >
        <span className="block text-base uppercase tracking-widest text-ink/60">{flipped ? t("study.answer") : t("study.question")}</span>
        <span className="mt-2 block">{flipped ? card.back : card.front}</span>
      </button>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
        <span className="text-lg tracking-widest" data-testid="flashcard-position">
          {position + 1} / {order.length}
        </span>
        <div className="flex flex-wrap gap-3">
          <button type="button" onClick={() => go(-1)} disabled={position === 0} className={actionButton}>
            {t("study.back")}
          </button>
          <button type="button" onClick={() => go(1)} disabled={position === order.length - 1} className={actionButton}>
            {t("study.next")}
          </button>
          <button
            type="button"
            onClick={() => {
              setOrder(shuffled(order));
              setPosition(0);
              setFlipped(false);
            }}
            className={textButton}
          >
            {t("study.shuffle")}
          </button>
        </div>
      </div>
    </div>
  );
}

// ---- quiz ----------------------------------------------------------------------------------------------------

function Quiz({ questions }: { questions: QuizQuestion[] }) {
  const t = useT();
  const [index, setIndex] = useState(0);
  const [chosen, setChosen] = useState<number | null>(null);
  const [score, setScore] = useState(0);
  const [done, setDone] = useState(false);

  if (done) {
    return (
      <div className="mt-3" data-testid="quiz-result">
        <p className="text-2xl">
          {t("quiz.score", { score, total: questions.length })}
        </p>
        <button
          type="button"
          onClick={() => {
            setIndex(0);
            setChosen(null);
            setScore(0);
            setDone(false);
          }}
          className={`${actionButton} mt-3`}
        >
          {t("quiz.again")}
        </button>
      </div>
    );
  }

  const q = questions[index];
  const answered = chosen !== null;
  const choose = (i: number) => {
    if (answered) return;
    setChosen(i);
    if (i === q.answer) setScore((s) => s + 1);
  };

  return (
    <div className="mt-3" data-testid="quiz">
      <p className="text-base uppercase tracking-widest text-ink/60">
        {t("quiz.progress", { n: index + 1, total: questions.length })}
      </p>
      <p className="mt-1 text-2xl">{q.question}</p>
      <ul className="mt-3 space-y-2">
        {q.options.map((option, i) => {
          const right = answered && i === q.answer;
          const wrong = answered && i === chosen && i !== q.answer;
          return (
            <li key={i}>
              <button
                type="button"
                onClick={() => choose(i)}
                disabled={answered}
                className={`w-full rounded-md border-2 px-4 py-2 text-left text-xl ${
                  right ? "border-ink bg-ink text-canvas" : wrong ? "border-red-600 text-red-600" : "border-ink hover:bg-ink/5"
                } disabled:cursor-default`}
              >
                {right && <span className="sr-only">{t("quiz.correctSr")}</span>}
                {wrong && <span className="sr-only">{t("quiz.wrongSr")}</span>}
                {option}
              </button>
            </li>
          );
        })}
      </ul>
      {answered && (
        <div className="mt-3" role="status">
          <p className="text-xl">{chosen === q.answer ? t("quiz.correct") : t("quiz.notQuite", { answer: q.options[q.answer] })}</p>
          {q.explanation && <p className="text-lg text-ink/70">{q.explanation}</p>}
          <button
            type="button"
            onClick={() => (index + 1 < questions.length ? (setIndex(index + 1), setChosen(null)) : setDone(true))}
            className={`${actionButton} mt-3`}
          >
            {index + 1 < questions.length ? t("quiz.next") : t("quiz.seeScore")}
          </button>
        </div>
      )}
    </div>
  );
}
