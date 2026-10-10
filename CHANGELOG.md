# Changelog

What changed in Inkling, newest first. This file is generated from the commit history by `scripts/changelog.py`;
run `python scripts/changelog.py -o CHANGELOG.md` to bring it up to date. Only changes you would notice are listed:
new features, changes and fixes. Tests, documentation, CI and dependency updates are in the history but not here.


## 2026-10

### Added

- **frontend:** Make and revoke API keys on the account page
- **go-api:** Add API keys and the /v1 routes for programs
- **go-api:** Cap daily AI work per visitor and for the whole site
- **frontend:** Attach and take photos of pages
- **go-api:** Let signed-in users upload photos of pages
- **ai-service:** Read photos of pages
- **go-api:** Let signed-in users have scanned pages read
- **ai-service:** Read scanned PDF pages with OCR
- **frontend:** Keep recordings and play them from saved summaries
- **frontend:** Record meetings and preview recordings
- **frontend:** Play summaries and podcasts with a proper player
- **frontend:** Add shared playback and recording logic
- **go-api:** Keep recordings and serve every file as what it is
- **frontend:** Attach recordings and read their transcripts
- **go-api:** Accept recordings from signed-in users and serve a document's text
- **ai-service:** Transcribe recordings with Whisper
- **ai-service:** Add the summary-quality runner
- **ai-service:** Add documents with known facts for the quality check
- **ai-service:** Score summaries against known facts
- **ai-service:** Write summaries at a low temperature
- **frontend:** Write an overview of a collection
- **go-api:** Add the collection overview endpoint
- **ai-service:** Write an overview of a collection of documents
- **frontend:** Pick a collection to ask about
- **go-api:** Ask a collection of documents by tag
- **frontend:** Summarize web links, Word and PowerPoint files
- **go-api:** Accept Word and PowerPoint files and web links
- **ai-service:** Read Word, PowerPoint and web pages
- **frontend:** Explain why older summaries lack the document tools
- **frontend:** Restyle the dashboard, account and sign-in pages
- **frontend:** Restyle chat, study, proof and podcast panels
- **frontend:** Restyle saved summaries and move extra actions into More
- **frontend:** Put the input first in the summarizer
- **frontend:** Add a landing page
- **frontend:** Redesign the header, footer and shared controls
- **frontend:** Add landing page and menu texts in all languages
- **frontend:** Add design tokens, fonts and shared styles
- **frontend:** Translate the rest of the interface and add standing instructions
- **frontend:** Add a dark theme and make the app installable
- **frontend:** Use the new tools on saved summaries and the dashboard
- **frontend:** Add the study panel and the shared summary page
- **frontend:** Add tags, sharing, email, rewrite and read-aloud controls
- **frontend:** Add export, speech and study helpers
- **frontend:** Add the Bosnian text and the language switcher
- **frontend:** Add the German and French interface text
- **frontend:** Add the English and Spanish interface text
- **go-api:** Add suggested questions, study material and the new routes
- **go-api:** Add rewriting, sharing, email and standing instructions
- **go-api:** Add tags, renaming and search for saved documents
- **ai-service:** Add standing instructions and the study endpoints
- **ai-service:** Add suggested questions, flashcards and quizzes
- **go-api:** Search the library by meaning as well as keyword
- **ai-service:** Add local embeddings and a search quality test set
- **frontend:** Add copy buttons, remembered settings, sample text and reading time
- **go-api:** Title pasted text from its first words
- **frontend:** Add PDF viewer, proof view, library chat and podcast player
- **go-api:** Keep original PDFs and add library search, proof check and podcasts
- **ai-service:** Add proof check, library answers and podcast scripts
- **frontend:** Show clickable citations and sources under chat answers
- **go-api:** Pass chat citations through to the client
- **ai-service:** Cite page-exact sources in chat answers
- Add a Prometheus and Grafana stack with a ready-made dashboard
- **ai-service:** Add metrics, opt-in Sentry reporting and a shared secret
- **go-api:** Add metrics, opt-in Sentry reporting and an AI service secret
- **go-api:** Add RATE_LIMIT_MULTIPLIER setting
- **frontend:** Add a nonce-based Content-Security-Policy
- **frontend:** Use cookie sessions and add account, reset and verify pages
- **go-api:** Replace JWT with cookie sessions and add account endpoints
- **go-api:** Add session auth, CSRF, quota and Turnstile middleware
- **go-api:** Add session, email token and mailer packages
- **frontend:** Paginate saved summaries with Load more
- **ai-service:** Add summary cache, circuit breaker and request logging
- **go-api:** Paginate the documents list
- **go-api:** Manage the schema with SQL migrations
- **go-api:** Add structured logging and request ids
- **frontend:** Stream summaries with live progress
- **go-api:** Proxy streamed summaries
- **ai-service:** Stream summaries and add model fallback
- **go-api:** Add health endpoints and PORT support
- **frontend:** Add style, language, multi-PDF input and chat
- **go-api:** Add multi-document summaries and document chat
- **ai-service:** Add styles, languages, multi-document and chat
- **go-api:** Validate signup input and require verified Google email
- **go-api:** Add rate limiting and body size limits

### Changed

- **go-api:** Run migrations and structured logging at startup
- **frontend:** Share markdown and PDF export helpers

### Fixed

- **go-api:** Ignore forwarded addresses unless proxies are trusted
- **frontend:** Allow the microphone and in-browser audio on the site's own pages
- **ai-service:** Drop folders from a recording's name on any system
- **frontend:** Drop the dimmed word count while a page limit sets the length
- **ai-service:** Stop summaries working out end dates and writing YoY
- **go-api:** Update golang.org/x/net for five HTTP/2 vulnerabilities
- **ai-service:** Keep summaries to the facts and name the output language
- **frontend:** Show HTML-like text in summaries as typed instead of building tags
- **frontend:** Drop ==== underlines under headings so they are not read aloud
- **ai-service:** Let a summary sentence combine facts from nearby sentences
- **frontend:** Announce error messages to screen readers
- **frontend:** Stop clipping attached files and style summary tables
- **frontend:** Log out when the session token is expired or rejected
- **frontend:** Keep one textarea mounted so the first keystroke isn't lost
- **go-api:** Send empty chat history as an array
- **ai-service:** Use a current Groq model and accept null chat history
- **frontend:** Repair eslint config and add password hint
- **ai-service:** Cap LLM concurrency and handle long documents
- **go-api:** Add timeouts and validate AI service responses


## 2025-11

### Added

- More fixes for the input field
- Responsiveness for mobile
- Optimized the process_summary function for less token consumption
- Minor fixes
- Fixed CORS errors
- Prepare backend for production
- Implemented animations and minor fixes
- Implement summary deletion
- Add loading bar
- **frontend:** Implement request cancellation
- Implement E-Ink UI, Page Limit, and PDF Export
- Implement Google OAuth 2.0 login
- **frontend:** Login and SignUp re-design
- **frontend:** Implement 'Save as PDF' functionality
- Implement text summarization and complete UI redesign
- Implement final UI for guest and user summarization
- **frontend:** Implement auth state management with React Context
- **frontend:** Implement signup and login forms with API integration
- Protect routes and implement document history
- Implement user login and JWT authentication
- Implement user registration with database connection
- Implement core summarization feature with Ollama

### Changed

- **ui:** Complete E-Ink redesign and UX enhancements

### Fixed

- **deploy:** Update python service to run on port 10000
