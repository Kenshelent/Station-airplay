# AirPlay-owned panel for official Luna 0.2.26

**Private test candidate, not a release.** No modifications to Luna, no new
LAN listener, no browser runtime token. General Luna LED calibration is not
part of this panel.

## Components

1. `airplay/airplay-luna 2.1.8-panel.2`: receiver and private Unix-socket
   API, including bounded idempotent requests, device-local presets and the
   per-point artwork colour table.
2. `airplay/panel-command 0.1.0-test.1`: optional one-shot binary helper.
3. `airplay-panel-0.1.0-test.2.yap`: web-only Lua companion. Install this
   through Luna's `.yap` upload, not through YAM v2.

The HTML registers at `/web/airplay-panel` and appears in Luna's custom tabs.
The Lua companion requests only `web`; it does not execute programs.
The separate helper requires `exec` and Luna's existing administrator
`allow_plugin_binaries` setting, already needed by the receiver.

Why `.yap` for the UI: official Luna's custom-page file check expects the
canonical HTML to remain inside the plugin root. YAM installs individual
file symlinks to another version directory; the existing Lua code does not
pin that root as binary launch does. A normal local Lua install keeps real
files inside its root. Do not bypass or weaken the path check.

## Installation (test hardware only)

Back up the current AirPlay settings first. **Stop all YAM binary plugins**
through Luna before installing either `.yampkg`; never unlink `yam.lock`.
Use packages for the actual model, not another speaker's archive.

```sh
yam --platform <model> --profiles /data/yam/platforms install -y /data/local/tmp/station-airplay-2.1.8-panel.2-<model>.yampkg
yam --platform <model> --profiles /data/yam/platforms install -y /data/local/tmp/airplay-panel-command-0.1.0-test.1-<model>.yampkg
```

AirPlay's existing postinstall may start services. Stop them normally again
before the next YAM transaction if necessary. Local prerelease Luna may fail
the `>=0.2.26` resolver check; do not override it. This candidate targets
official Luna 0.2.26.

Upload `airplay-panel-0.1.0-test.2.yap` using Luna's plugin installation UI.
Enable:

- AirPlay receiver as usual;
- **AirPlay Panel Command** as an enabled plugin, **not** an autostart service;
- **AirPlay Panel** as an enabled autostart service.

Do not invoke or restart the receiver to transport settings commands.
Only restart it deliberately after saving configuration, with playback
stopped. Settings and EQ are applied on receiver restart, not live.

## Transport and ownership

- The page uses only existing same-origin Luna invoke/logs endpoints.
- Only the helper ID is invoked; the receiver PID is not affected.
- Each logical request gets a random ID and the receiver's boot epoch.
- Retries reuse that request; the receiver caches up to 128 results for
  60 seconds. UI retries are bounded to 12 seconds. This is retry safety
  within that window, not a durable transaction log across restarts.
- Each helper invocation also has a separate attempt ID. Old or partial
  log records cannot be confused with the latest attempt.
- Two tabs may cancel each other's short-lived helper, but committed
  operations remain cached. Tests cover eventual retries and conflicting
  payloads. Sustained many-client fairness is not qualified.
- A receiver restart rejects old mutations: reload the page.
- Timeout means **unknown outcome**, not rollback. Reload settings before
  repeating a save.
- Preview still belongs to AirPlay's existing four-second lease and
  speech/volume gates. If command renewal is too slow it expires safely.
  Smooth live preview on physical ARM hardware is not yet qualified.
- Do not expose new tokens, shell execution or arbitrary paths/URLs.

## UI features

AirPlay artwork gains, brightness, saturation, black-floor override,
timing, experimental EQ, per-zone preview, image sampling, named presets
and JSON import/export. The nine editable colour points (grey, red, green,
blue, yellow, cyan, magenta, orange, white) form a 3x3x3 table with
trilinear interpolation, so one hue can be retuned without moving its
neighbours. Black keeps the existing per-model black-cover colour and
floor because those models have their own off/fallback behaviour.
Presets live in AirPlay's state directory.
Changes affect AirPlay only, not system music/Alice or other Luna plugins.
EQ is off by default; keep it off for timing/video checks.

The page is derived from the original calibration UI authored for Luna.
The MIT notice is in `LUNA-LICENSE`; it is included in the UI archive.

## Validation / limits

Run `node panel/test-transport.cjs panel/transport.js` and Go race/vet.
The mocked browser transport tests cancellations, two tabs, partial output,
read coalescing and epoch changes. The QEMU test installs the exact YAM
receiver/helper, uploads the Lua `.yap` to official Luna, saves/restores
EQ/timing, checks replay/conflict/epoch responses, simultaneous clients,
presets, lease expiry and unchanged receiver PID.

Chromium rendering and start/stop/save/preset interactions passed against
the same-origin browser fixture, with no JavaScript errors. That is not a
real receiver backend. Long-running renewal under playback, hardware
colour confirmation, power-cycle persistence and EQ CPU remain pending.
Do not call this production-ready on the strength of QEMU alone.

This branch is based on v2.1.7, separate from `2.1.8-mini.1`; it does not
include that other candidate's Mini volume/colour fixes. Integrate and
qualify both before offering a combined Mini update.

Nothing is published to GitHub releases or the registry by these scripts.
