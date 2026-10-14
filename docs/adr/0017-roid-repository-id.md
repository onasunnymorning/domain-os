# ADR 0017 — ROID repository ID: one per installation, configured, guarded

- **Status:** Accepted
- **Date:** 2026-10-14
- **Deciders:** Platform / Registry
- **Composes with:** [ADR 0006](0006-tenancy-model.md) (the tenancy model), [ADR 0010](0010-database-migrations.md) (the migrate job), [ADR 0013](0013-epp-server-architecture.md) (the EPP server), [ADR 0015](0015-rst-conformance-test-suites.md) (RST epp-14)

## Context

Every contact, host and domain has a repository object identifier (ROID). RFC 5730 constrains its form to `(\w|_){1,80}-\w{1,8}`. The part after the `-` is the registry's repository ID, from IANA's [EPP Repository Identifiers](https://www.iana.org/assignments/epp-repository-ids) registry: 1 to 8 letters or digits, registered first come, first served. RST epp-14 checks that the ROIDs a registry hands out carry an ID registered there.

This registry minted `{snowflake}_{DOM|HOST|CONT}-{APEX}`, with `APEX` a code constant (`entities.EPP_REPOSITORY_ID`). `APEX` is not in the IANA registry (checked 2026-10-14).

The tables store only the snowflake, in `ro_id`, as a number, so ROIDs sort by creation time. The suffix was added on every read.

## Decision

1. **One repository ID per installation, `ROID_REPOSITORY_ID`.**
   - Contacts and hosts are scoped to their sponsoring registrar (ADR 0006), not to a TLD. One host serves domains in several TLDs, and one contact can be used in several. A per-TLD or per-operator ID would have no answer for them.
   - IANA assigns repository IDs per registry.

2. **Only the number is stored.** `ro_id` stays a number. The contact, host and domain repositories take the installation's repository ID through their constructors and append it whenever they read a ROID. That makes every output agree: EPP, the admin API and so the UI, whois and MCP today, escrow export and RDAP when they are built. Nothing reads a package global.

3. **Every process that mints or shows ROIDs requires the ID.**
   - It is read once by `config.LoadRepositoryID`, with no default. A process without a valid ID does not start. A default is exactly what would reach production unnoticed.
   - The processes that need it: the admin API and `seed`, the EPP server, the worker, whois, MCP, askg, and the JISC import.
   - `migrate` and `init-registrars` do not need it.
   - The local configurations supply a value: Docker Compose `LOCAL` (overridable), Tilt the value in Doppler or else `LOCAL`, and CI `CITEST`.

4. **The installation records its ID, and refuses another.** On start, each of those processes (except the JISC import, which writes only numbers) calls `postgres.CheckRepositoryID`:
   - The first start records the configured ID in `installation_settings`.
   - A later start with a different ID fails with `ErrRepositoryIDChanged`.

   Because only the number is stored, a changed ID would silently rename every object. The check turns that into a startup error.

5. **ROIDs are always generated on create.**
   - The admin API's create commands no longer accept a ROID.
   - Escrow imports, through both the bulk importer and the per-object path, give every imported object a new ROID. A deposit's ROID belongs to another system, and its number may collide with ours.

6. **Valid IDs are 1 to 8 ASCII letters or digits.** IANA allows letters and digits, and every registered ID is ASCII. The underscore that RFC 5730's `\w` would allow is refused, because IANA does not allow it.

## Consequences

- **Set the ID once, before the first start.** Changing it later needs a deliberate operation: update the recorded row and accept that every ROID changes. Before go-live this is harmless. After it, ROIDs have been given to registrars, escrow and RDAP.
- **Existing installations show their objects with the configured ID** from the first start of this release. No `-APEX` ROID has left the system yet: there is no escrow export or RDAP, and the EPP server is not live.
- **Deploying this release.** Set `ROID_REPOSITORY_ID` for admin-api, epp-server, unified-worker, whois and mcp-server first, or they will not start. The `migrate` job creates `installation_settings`.
- **Before an RST or OT&E run,** register the ID with IANA and configure it. RST epp-14 checks the registration with IANA, so the suite treats it as operational. What the code can show, that ROIDs end in the configured ID, is tested.
- **Local development.** A process started under Doppler outside Tilt needs `ROID_REPOSITORY_ID` in `dev_personal`. A local database records the first ID it is started with.
