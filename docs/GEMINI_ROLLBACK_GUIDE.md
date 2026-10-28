# Rollback preparation
No deployment or DB schema change occurred in this checkpoint.
Gemini must retain a verified previous application artifact and a tested DB backup before deployment. Rehearse restoration on a disposable instance; record restore time and integrity checks. Do not assume schema downgrade is safe with newer writes.
For these source-only security fixes, reintroducing the old handlers also reintroduces vulnerabilities. Prefer disabling affected routes while correcting a regression.
A production-specific rollback command sequence and recovery objectives remain pending infrastructure selection.
