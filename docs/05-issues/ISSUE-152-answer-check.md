# ISSUE-152 — Kontroll av svaren

`epic: EPIC-14` · `priority: P2` · `state: done` · klar 2026-11-08

Idag klassificeras bara frågan. Kontrollera också svaret med samma fasta
regler: innehåller det hemligheter eller personuppgifter som inte fanns i
frågan? Först bara flagga (Inspector, request log, auditkedjan) — inte
blockera. Ger CISO:n "vi kontrollerar både in och ut". Strömmade svar kräver
att kontrollen görs i efterhand eller i block, utan att fördröja svaret.

## Levererat (2026-11-08)

- `internal/answercheck`: svaret läses med samma *värde*-detektorer som frågan
  (`secrets.Detect`, `pii.Detect`), inte med ämnesord, så ett svar som
  förklarar vad ett lösenord är flaggas inte; ett som innehåller ett gör det.
  Det frågan redan höll flaggas inte.
- Vanliga svar kontrolleras innan de skrivs (header `X-Router-Answer-Flags`);
  strömmade svar samlas och kontrolleras efter `[DONE]`, utan fördröjning.
- Flaggan hamnar i request-loggen (`answer_flags`, ny kolumn), auditkedjan
  (`response.flagged`, bara typer), `/metrics` (`router_answer_flags_total`)
  och chattens inspektor (*Answer check*). Blockerar aldrig.
- Kvar (backlogg): att *blockera* eller *maskera* ett flaggat svar; hör ihop
  med ISSUE-151 (mask and send).
