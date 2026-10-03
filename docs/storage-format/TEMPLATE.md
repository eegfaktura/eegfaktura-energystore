# <YYYY-MM-DD> — <title>

<!-- Copy to docs/storage-format/<YYYY-MM-DD>-<slug>.md. Answer every heading (AGENTS.md section 8.1). -->

## 1. What changes

The keys, buckets, value layout, meta record (`cpmeta/0`) or directories the change creates or
alters, and why. Say whether the REST/GraphQL output or the Excel export changes with it.

## 2. Data migration

What happens to existing data (conversion with an `estore` command, lazy conversion on read,
nothing), how long it takes at production size (about 1.2 TB, all communities), and whether the
MQTT ingest must stop meanwhile.

## 3. Compatibility

Does the previous version still read the new data, and the new version the old? What must be
deployed together (eda-xp, eegfaktura-web, eegfaktura-v3, the backend's master data)?

## 4. Rollback

The steps, starting from the backup taken before the migration. Name the data that cannot be
restored (values imported after the migration).

## 5. Verification

The tests that prove it (package and test names, run against `t.TempDir()` data) and the check the
operator runs on a copy of production data.
