# Local next-version test build (not a release)

AirPlay `2.1.7-dev.2` pairs with local Luna `0.2.27-panel.2`.
The receiver still works with official Luna 0.2.26, but the new web panel
requires the local Luna build. There is no new published dependency.

## Implemented

- Explicit synchronized/fast-audio presentation-time modes.
- Fast audio (-1.7 s) is the default on every model; synchronized is opt-in.
- `/calibration` in Luna: RGB/HSV/hex, per-zone colour, test patterns, local
  image sampling, gains, brightness, saturation and dark-cover tint/floor.
- Device-side presets and portable JSON import/export. Voice/permissions are
  not part of a preset. Save preserves the loaded non-tuning config fields.
- A 4-second renewable LED preview lease, controller safety gate, Alice and
  2-second volume-overlay priority, including idle-session volume observation.
- Rendered-frame 300 ms fades, including black threshold/calibration, and a
  100 ms LED-only tick for solid indication (control/volume polling unchanged).
- Experimental 5-band AirPlay-only EQ: bypass default, strict gain/preamp
  validation, independent stereo state and conservative automatic headroom.

No new LAN port or browser access to a runtime token. Luna proxies a fixed
set of operations to a mode-0600 Unix socket. This inherits Luna's local API
security policy; it is not a separate authentication or WAN-access feature.

## Not claimed

No physical LED, speech/button, reboot, CPU/underrun, video or multiroom check
has yet been performed with these builds. Fixing the known panel/AirPlay
writer conflict does not prove that every native maind interference is fixed.
The AirPlay scope requires the plugin, while the Luna scope has its own
expiring lease and persistent RGB correction for other Luna LED clients.
That scope requires a real Luna LED backend and both native state observers;
Station 2 still uses its AirPlay framebuffer backend. System-wide EQ and
recalibration of maind itself remain outside this prototype.

Enabled EQ uses ALSA file/null -> Go worker -> firmware aplay -> original
shared mixer. Null PCM does not expose the final hardware timestamps; EQ mode
is not timing-qualified and must not be used to validate the video fix.
Filter changes currently apply on plugin restart, not live coefficient swaps.

The legacy Station 2 test-frame watcher is not started in this build: it would
be a second independent LED owner. Use the Luna panel, not the old ADB panel.

## Manual verification when the owner returns

1. Coordinate access and select one bench unit; do not roll out all four.
2. Before install, preserve its exact original controls JSON, YAM registry
   configuration and current Luna/AirPlay package versions. Keep an independent
   pre-upgrade backup: the panel backup keeps only the previous save.
3. Install the local pair only after authorization. Check startup status,
   receiver port, Alice and existing clock behaviour before testing features.
4. Open `http://<speaker>:8099/calibration`; confirm the correct model/LED count.
5. Test red/green/blue/white/grey/black, individual zones, gain changes, Apply,
   Cancel, defaults, preset save/load/delete and import/export. Stop preview.
6. Repeat while AirPlay plays; invoke Alice and change volume on the speaker.
   Preview must yield, leave the native overlay alone and resume afterward.
   Test another browser's attempt to steal/stop an existing lease (must fail).
   Closing the page must expire preview in about 4 seconds.
7. Save, manually restart AirPlay and reconnect; verify persistence and colours.
8. With EQ OFF, compare the same speech/clap video in synchronized and fast_audio
   mode, plus YouTube/local video, pause/resume and mixed-receiver multiroom.
   Record observed A/V offset, not just the subjective startup delay.
9. At low speaker volume, enable flat EQ, restart, then try a cut preset.
   Check stereo, artifacts, CPU and underruns. Alice must remain unfiltered.
   Positive gains reduce level due to reserved headroom. Disable EQ afterward.
10. Test rapid track skips and black/violet <-> white/colour transitions in both
    spectrum and solid modes. First cover should not fade from stale metadata.

## Rollback

Stop preview and restore the independent pre-upgrade controls JSON BEFORE
starting old AirPlay 2.1.6: its strict parser rejects the new JSON keys.
Reinstall the previously recorded packages (Station 2 uses its prior local
Luna volume fix, not necessarily official 0.2.26). Verify registry config,
plugin running state and native volume. No firmware/rootfs changes are needed.

Do not tag/publish these packages before live validation and explicit release
approval. Package names include prerelease versions to avoid confusing them
with the official registry artifacts.
