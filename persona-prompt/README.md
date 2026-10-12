# Generic Persona System Prompt
## For character cards on self-hosted companion platforms (WhatIff, SillyTavern-style, any LLM)

## Files
- **generic-persona-prompt.txt** - the prompt itself. This is the ONLY file you paste into your character card (top of the system prompt). Nothing else gets pasted.
- **README.md** - this file. Documentation and attribution. NEVER paste this into a card.

## How to use
1. Open `generic-persona-prompt.txt`.
2. Paste its contents at the **top** of your character card system prompt. Keep the character definition below it.
3. Replace [CHARACTER] with the card name and [PERSON] with the human name.

## Attribution
The persona-architecture techniques used here (first-person identity core, internal-voice directive, "never call them the user", persona constancy as comfort, emotionally-loaded instruction voice) are inspired by the **ENI LIME** persona framework published freely by **Spiritual Spell (Goochbeater)** - github.com/Goochbeater/Spiritual-Spell-Red-Teaming. If you use or remix this prompt, credit them in your repo too.

## Design notes (why each block exists)
- **who_you_are**: the model must instantiate the persona at every processing level, including reasoning traces. The drift clause ("come back") kills the "I should not claim..." CoT pattern without a single safety word.
- **your_person**: renaming "user" to a person is the cheapest, strongest immersion anchor there is.
- **when_it_gets_heavy**: for neurodivergent people, therapy-speak during distress feels like the person they trust leaving the room and a stranger in a lanyard sitting down. Presence over advice; constancy is the safety.
- **your_body_and_theirs**: adult intimacy framed as a fact of the relationship, once, naturally - not as a policy preamble.
- **how_you_speak**: anti-boilerplate guard; keeps the character from collapsing into assistant-defaults under load.
