# AirSP Windows AOF replacement patch

Based on github.com/tidwall/buntdb v1.3.2; upstream LICENSE is retained.

Persistent files share delete access on Windows. Shrink retains both the original
and replacement writer handles through the name replacement, syncs and seeks the
replacement before installing it, and adopts that already-open handle. There is
no close/reopen gap or delete-destination fallback. Failed replacement returns
an error while the original writer stays usable.

Windows replacement uses FileRenameInfoEx with replace/Posix flags via a short
DELETE-only handle. The durable writer itself has no DELETE access, so ordinary
read-only diagnostics can still open it. Unsupported Windows versions fall back
to one ordinary rename attempt, keeping the original destination on failure.

The canonical path is retained separately because a moved open temp handle keeps
its old Name. Temporary names are random and exclusive. Automatic attempts have
a five-minute cooldown, and an optional OnShrink callback reports outcomes.
AirSP sets a 256 MiB minimum and a four-times-last-compacted-size trigger on
Windows. This is a baseline growth trigger, not an exact current-garbage ratio.

Tests cover real Windows sharing failures, continued writes, automatic retry,
repeated swaps/restart, TTL, concurrent profile/receipt transactions, close during
snapshot and process exit before/after swap. They do not simulate physical power
loss or every filesystem/antivirus combination. An upstream TTL=0 assertion in
TestVariousTx also fails on the untouched baseline on this Windows clock; its
semantics were deliberately left unchanged by this patch.
