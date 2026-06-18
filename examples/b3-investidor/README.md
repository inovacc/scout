# B3 Investidor Extraction Pipeline

Capture your B3 *Área do Investidor* session once (headed, gov.br + MFA), then pull
all sections headless on every run to raw JSON + flattened CSV.

## One-time bootstrap
1. `task b3:bootstrap` — a headed Chrome opens; log in via gov.br (CPF + MFA) and
   walk Posição / Movimentação / Proventos. Close the window. Auth is sealed into
   vault profile **b3** (note the printed profile ID). You'll be asked for a vault
   passphrase — remember it; export it as `SCOUT_PASSPHRASE` for runs.
2. `task b3:recon` — records the API traffic to `capture.json`. From it, author
   `sections.yaml` (copy `sections.example.yaml`): one entry per section with its
   `endpoint` and the `record_path` to its record array, and set `auth.mode`
   (`bearer` if the JWT is in localStorage, `cookie` if it's an httpOnly cookie)
   and `token_storage_key` for bearer mode.

## Every run
```bash
export SCOUT_PASSPHRASE='…'
task b3:run PROFILE=<your-b3-profile-id>
```
Output lands in `$SCOUT_HOME/.scout-b3/b3-data/<timestamp>/` (and `latest/`):
`posicao.json` + `posicao.csv`, etc., plus `_run.json`.

## When it stops working
A `session expired` message means the **refresh token** died (not the short access
token). Re-run `task b3:bootstrap` (~1 min) and runs resume.

## Security
- Your gov.br password is never stored — you type it in the headed window.
- The session token lives only in the encrypted vault; `b3-data/`, `capture.json`,
  and the profile dir are gitignored and written outside the repo.
- After authoring `sections.yaml`, delete `capture.json` (it holds a live token).
