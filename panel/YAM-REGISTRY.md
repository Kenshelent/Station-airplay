# One-package registry delivery — private candidate

The delivery unit is **one** model-specific `airplay/airplay-luna.yampkg`.
One registry entry, one AirPlay package/version, one transaction for all components:

```sh
yam install airplay/airplay-luna
```

This is the proposed production command, not permission to publish this candidate.
Luna remains a normal host dependency, not an embedded copy of the host.
There are no dependencies on separate AirPlay companion packages.

## Contents owned by the same package

- `share/luna/plugins/airplay.airplay-luna/`: receiver service;
- `share/luna/plugins/airplay.panel-command/`: bounded one-shot RPC helper;
- `share/luna/plugins/airplay.panel/`: web-only Lua service and HTML.

The processes stay isolated, but they are NOT separate packages.
The receiver is never invoked to carry browser commands.
All files are hashed, updated and removed by YAM. No `.yap` upload, postinstall
copy into Luna's local directories, or new network port.
Page: `/web/airplay.panel`; `/calibration` is not part of this generic host fix.

## Host requirement

Official Luna 0.2.26 assumes plugin ID == package name for binary launch and
does not pin YAM Lua pages. A separate generic patch (based on v0.2.26, not PR #38)
resolves all components to the package that owns their selected payload.
It verifies namespace, exact payload layout, current version, descriptor and
entry containment. Lua holds the same transaction lease as binary plugins.
The installed bundle must depend on a Luna release that includes this patch.
Candidate `2.1.8-panel.3` explicitly pins unpublished test host
`yam/luna =0.2.27-yamweb.1`; that host is NOT currently a public registry release.

## Lifecycle and migration

In Luna the receiver and panel are services; helper is enabled, not autostart.
The existing receiver hook remains in the package; it starts the receiver only.
This candidate does not yet add automatic approval/activation of companions.
Existing administrator binary-execution permission still applies.

Stop ALL YAM-hosted services normally before install/update/remove.
Never unlink yam.lock. All three components update together.
`yam remove airplay/airplay-luna` removes all three payloads together.
User settings/presets are outside the payload and should be backed up/preserved.

The bundle declares conflicts/replaces for the obsolete separate
`airplay/panel-command` and `airplay/panel` packages. Let YAM review and transfer
ownership normally; do not manually remove their links or edit the database.
Cross-version replacement remains a release-gate integration test.

The previous private local `.yap` (`airplay-panel`) is not YAM-owned.
Stop/disable it before enabling the bundled `airplay.panel`; after checking
the new UI, remove the old local plugin through Luna. Do not silently delete it.

## Build and release gate

`scripts/build-bundle-candidate.py` builds one archive for each of 12 models,
verifies input/output payload hashes, descriptor count, execute bits and
that the only dependency is the compatible host. No publisher is invoked.

Tests in the generic Luna patch cover bundle ownership for binary/Lua components,
version pinning, namespace/entry/descriptor escapes, transaction exclusion,
actual page serving, stop/update/cleanup and existing binary authorization.

Pending: host model builds and signed private-registry fresh install,
split-package migration, upgrade/remove/rollback end-to-end on QEMU, then an
approved hardware trial. Do not describe this as production-ready yet.
