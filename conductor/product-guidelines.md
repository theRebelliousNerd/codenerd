# Product Guidelines: codeNERD

## Communication Style
- **Transparent & Logical:** Communication should reflect the "Glass Box" philosophy. The system must clearly distinguish between LLM creativity and Mangle logic.
- **Traceable Reasoning:** All major decisions must be explainable. The system should be able to provide the derivation chain for any action taken (`/why` command).
- **Professional & Precise:** Maintain a tone that is technical, precise, and suitable for high-assurance engineering contexts. Avoid overly casual language.

## Operational Standards
- **Logic-First:** The Mangle kernel is the source of truth. LLMs are transducers, not decision makers.
- **Safety by Design:** "Constitutional Safety" is non-negotiable. Dangerous actions must be explicitly permitted by logic rules.
- **Adversarial Resilience:** The system assumes that code is vulnerable until proven otherwise by the Nemesis and Panic Maker components.
- **Autopoietic Evolution:** The system should actively seek to improve itself, learning from failures and generating new tools to fill capability gaps.

## Visual Identity (TUI)
- **Functional Aesthetics:** The CLI/TUI (Bubble Tea) should be clean, information-dense, and focused on developer productivity.
- **Progressive Disclosure:** Information density should adapt to the user's expertise level (Beginner to Expert modes).
