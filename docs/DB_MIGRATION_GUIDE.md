# DB migration checkpoint
Authoritative SQL: database/001_all_phases.sql, 002_shared_ws_tickets.sql, 003_polls_and_reactions.sql. No migration was added by the security fixes in this checkpoint.
The local SQLite adapter applies/translates the initial schema; this is not proof that a live PostgreSQL upgrade works.
Before Gemini applies changes: inspect the current schema, determine which migrations are already represented, back up, test on a copy, add migration version tracking and validate constraints/concurrency. Do not blindly replay the initial SQL on a populated production DB.
Rollback is restore-based until reviewed reversible migrations and data compatibility are established. Production execution was not attempted.
