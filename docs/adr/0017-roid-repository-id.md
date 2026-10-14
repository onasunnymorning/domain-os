# ADR 0017 — ROID repository ID: one per installation, configured, stored with each object

- **Status:** Accepted
- **Date:** 2026-10-14
- **Deciders:** Platform / Registry
- **Composes with:** [ADR 0006](0006-tenancy-model.md) (the tenancy model), [ADR 0010](0010-database-migrations.md) (the migrate job), [ADR 0013](0013-epp-server-architecture.md) (the EPP server), [ADR 0015](0015-rst-conformance-test-suites.md) (RST epp-14)

## Context

Every contact, host and domain has a repository object identifier (ROID). RFC 5730 constrains its form to `(\w|_){1,80}-\w{1,8}`. The part after the `-` is the registry's repository ID, from IANA's [EPP Repository Identifiers](https://www.iana.org/assignments/epp-repository-ids) registry: 1 to 8 letters or digits, registered first come, first served. RST epp-14 checks that the ROIDs a registry hands out carry an ID registered there.

This registry minted `{snowflake}_{DOM|HOST|CONT}-APEX`, with `APEX` a code constant (`entities.EPP_REPOSITORY_ID`). `APEX` is not in the IANA registry (checked 2026-10-14).

Two findings shaped the decision:

1. **The suffix was never stored.** The `contacts`, `hosts` and `domains` tables keep only the number, in `ro_id`. The conversion from a row added `-APEX` on every read. Changing the constant would therefore have renamed every existing object, including objects already handed to registrars over EPP and written into escrow deposits.
2. **Contacts and hosts belong to no TLD.** They are scoped to their sponsoring registrar (ADR 0006). One host serves domains in several TLDs, and one contact can be used in several. Per TLD, the operator boundary, there is no single right repository ID for them.

## Decision

1. **One repository ID per installation, `ROID_REPOSITORY_ID`.** It is read once, by `config.LoadRepositoryID`, in each process that mints ROIDs:
   - the admin API and its `seed` command;
   - the EPP server;
   - the worker (its lifecycle and escrow-import activities);
   - the JISC import CLI.

   It is passed to the ROID service and the bulk importer through their constructors. Nothing reads a package global. A per-TLD or per-operator ID was rejected because of the second finding: it has no answer for contacts and hosts. If an operator ever needs its own ID, decision 3 means only minting has to change.

2. **No default.** A process that mints ROIDs refuses to start without a valid ID. The read-only processes (MCP, askg, whois) mint none and do not read it, and neither do `migrate` and `init-registrars`.

   The plan proposed allowing local development to run without it. There is no reliable signal that a process is local, and a default is exactly what would reach production unnoticed. Instead, the local configurations supply a value:
   - Docker Compose: `LOCAL`, overridable.
   - Tilt: keeps a value set in Doppler, else `LOCAL`.
   - CI: `CITEST`.

3. **The repository ID is stored with each object,** in a `ro_id_repository` column beside `ro_id`. A ROID is read back as it was minted.
   - **Existing rows.** A step before AutoMigrate adds the column to tables that predate it. Their rows get `APEX`, the ID they were minted with, and the column's default is then dropped. A writer that leaves the column out fails rather than silently getting `APEX`.
   - **Changing the ID later.** This affects only objects minted afterwards. An object keeps its ROID, because ROIDs are identifiers that registrars, escrow and (later) RDAP have already been given.

4. **Valid IDs are 1 to 8 ASCII letters or digits.** IANA allows letters and digits, and every registered ID is ASCII. The underscore that RFC 5730's `\w` would allow is refused, because IANA does not allow it.

5. **`IsIssuedHere` recognises this registry's ROIDs by shape,** whatever their suffix. Objects minted under an earlier ID are still this registry's own. Lookups by ROID go by its number, which is unique on its own.

## Consequences

- **Deploying this release.** Set `ROID_REPOSITORY_ID` for the admin API, EPP server and worker first, or they will not start. The `migrate` job does not need it, and it backfills `APEX` into the existing rows.
- **Before an RST or OT&E run,** register the ID with IANA and configure it. RST epp-14 checks the registration against IANA itself, so the suite treats it as operational. What the code can show, that new ROIDs end in the configured ID, is tested.
- **Objects minted before this change keep `-APEX`.** No migration renumbers them, because a ROID must not change. If a production registry needs them under its registered ID, renumbering is a deliberate operation of its own: the objects' escrow and registrar-facing history move with them.
- **Imported objects.** Escrow and JISC imports already give every imported object a new ROID. That ROID now carries the configured ID. An object imported from this registry's own deposit, whose ROID is kept, now keeps its suffix too, where before it was re-read as `APEX`.
- **Local development.** Under Doppler, `ROID_REPOSITORY_ID` can be set in `dev_personal`. Tilt falls back to `LOCAL` when it is not.
