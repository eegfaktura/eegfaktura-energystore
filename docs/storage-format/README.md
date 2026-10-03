# Storage-format concepts

One file per change that touches the data the energystore keeps on disk, named
`<YYYY-MM-DD>-<slug>.md`. Mandatory content: AGENTS.md section 8.1 and [TEMPLATE.md](TEMPLATE.md) —
what changes, data migration, compatibility, rollback, verification.

The persisted format is the directory layout `<persistence.path>/<tenant>/<ecId>`, the row ids
(`CP/yyyy/MM/dd/hh/mm/ss`, local wall-clock time), the meta record `cpmeta/0`, the value layout of a
raw line (consumer triples, producer pairs, QoV arrays) and its encoding: JSON through
`store/ebow/codec/json`, the default of `ebow.Open` (`db.go:179`; no caller sets another codec, the
`msgp` codec is unused), keys through `codec/key`.
There is no schema tool: a format change is code plus, where needed, an `estore` command that
converts existing data.

The rule starts on 2026-10-02. Today's format is described in [0000-current-format.md](0000-current-format.md)
(2026-10-03, `open-points.md` ES-7).
