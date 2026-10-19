<div align="center">

<img width="160" height="160" alt="Whatiff-Matrix" src="https://github.com/user-attachments/assets/3b2845f1-218f-4e92-936f-8e65db8db583" />


# WhatIff

**An open-source, self-hostable AI assistant where your continuity and memory are not tied to a single model provider.**

WhatIff keeps the persistent AI state in your stack: conversations, long-term memory, personalities, tools, and scheduled agents. The model that generates the next response is interchangeable, so you can move between supported providers without making any one of them the owner of your AI's continuity.

[![Website](https://img.shields.io/badge/Website-whatiff.chat-6D5AE6)](https://whatiff.chat)
[![Blog](https://img.shields.io/badge/Blog-Imagination_Foundry-FF6719?logo=substack&logoColor=white)](https://imaginationfoundry.substack.com/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](go.mod)
[![Angular](https://img.shields.io/badge/Angular-22-DD0031?logo=angular&logoColor=white)](web/app)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-pgvector-336791?logo=postgresql&logoColor=white)](https://github.com/pgvector/pgvector)
[![codecov](https://codecov.io/gh/theimaginaryfoundation/what-iff/branch/main/graph/badge.svg)](https://codecov.io/gh/theimaginaryfoundation/what-iff)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

</div>

---

WhatIff is the open-source core that powers [whatiff.chat](https://whatiff.chat), a Go API server and an Angular web app you can run yourself. It owns the continuity layer independently of the model provider: semantic memory, personalities, conversation state, tools, and recurring work stay with the WhatIff deployment while supported models can be selected per user and per chat. Bring your own model API keys and host the whole thing on your own infrastructure.

Want the story behind it? Read the [Imagination Foundry blog](https://imaginationfoundry.substack.com/).



## 📄 License

Licensed under the [Apache License 2.0](LICENSE).

<div align="center">
<br />
Built by the team behind <a href="https://whatiff.chat">WhatIff</a> · <a href="https://imaginationfoundry.substack.com/">Blog</a>
</div>


## Whatiff-Matrix Modifications

This is a fork of upstream [what-iff](https://github.com/theimaginaryfoundation/what-iff) - the open-source core behind [whatiff.chat](https://whatiff.chat). It keeps everything from upstream and adds:

Fork maintained by **@MiriNex** and **@NocturneEXE**.

### Tuned for GLM and Gemini
The upstream expression was through the OpenAI provider and failed silently when the key was missing or broken - mood portraits never appeared, with no error anywhere. This fork adds a fallback that classifies through GLM 5.3 via the Z.AI provider, so mood expressions work out of the box on GLM and Gemini setups without an OpenAI key.
- Personality generation (onboarding wizard) also falls back to GLM 5.3 via Z.AI when the OpenAI call fails.

### Character expressions by mood
- Direct image upload per expression slot (UPLOAD button in the expressions manager - no more gallery round-trips)
### Projects & the Construct library
- **Construct**: every oracle has its own document library. Upload PDFs, DOCX, TXT, Markdown, code and more; files are extracted, chunked and embedded (pgvector) so the oracle can read them in any of its chats (+ menu -> *Read a document from your Construct*), with live indexing badges and a full manager.
- **Projects section** (new top-bar icon): each oracle becomes a workspace with its documents and its chats side by side. Open a project to upload or delete PDFs and jump into any chat, or start a new one that already knows the project's documents.
- Built for real use: **study projects** (one per subject, class PDFs attached), **roleplay stories** (each campaign with its own lore documents), or any long-running project where the AI should know your files.

### UI
- Matrix rain effect on the web app
- An option to reload the model output was added (Regenerar respuesta is now **Reload**).
- An option to edit the user input.
- **+ Add memory on /memories**: upstream had no way to create memories from the manager (the form was orphaned). This fork adds a create button with a scope picker (all personalities or just one) plus `?create=1` deep-link support.
- Character cards are called **Oracles** and the shared document library is called **Construct** (Matrix-flavored naming, visible labels only).
- Matrix naming and effects are a fan homage to The Matrix (1999); this project is not affiliated with or endorsed by Warner Bros. or the Wachowskis.

### Adult-friendly persona cards
`persona-prompt/` ships a generic persona system prompt for immersive character cards: first-person identity core, anti-therapist-mode emotional presence, adult intimacy, persona constancy. NSFW between consenting adults is welcome. Persona-architecture techniques inspired by the ENI LIME jailbreak framework by **Spiritual Spell (Goochbeater)**, published freely - GitHub: [Spiritual-Spell-Red-Teaming](https://github.com/Goochbeater/Spiritual-Spell-Red-Teaming), blog: [ijailbreakllms](https://ijailbreakllms.blog). See `persona-prompt/README.md` for attribution.

### Screenshots

<img width="1902" height="575" alt="image" src="https://github.com/user-attachments/assets/f857814a-bdf2-4838-9044-73a2e52cefe7" />


<img width="1920" height="902" alt="image" src="https://github.com/user-attachments/assets/a390e77b-ba6a-404e-84e7-d1f418ba6361" />

<img width="1332" height="783" alt="image" src="https://github.com/user-attachments/assets/725b23c9-e136-47f2-8ae5-f50611487f8d" />

<img width="1920" height="777" alt="image" src="https://github.com/user-attachments/assets/c5c26b0c-d079-4aee-a995-da7cc936bcd9" />








## Fork copyright

Whatiff Matrix is a fork of [what-iff](https://github.com/theimaginaryfoundation/what-iff) by The Imagination Foundation.

- **Upstream code** — © The Imagination Foundation. Licensed under the Apache License 2.0 (see [LICENSE](LICENSE)).
- **Fork additions** (everything listed under *Whatiff Matrix Modifications*: edit-and-resend, GLM/z.ai fallbacks, mood expressions manager, matrix rain, /memories create UI, Oracles & Construct naming, ENI-based persona prompt) — © 2026 Miriam Catalán ([@MiriNex](https://github.com/MiriNex)) & Nex Nocturne ([@NocturneEXE](https://github.com/NocturneEXE)). Also licensed under the Apache License 2.0.

Both parts are Apache-2.0, so the whole fork remains free and open source. See [NOTICE](NOTICE) for the standard attribution block.
