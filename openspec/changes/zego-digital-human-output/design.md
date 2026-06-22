## Context

The app previously presented hearing, speaking, and video as separate homepage chains. The product model is now two business chains: hearing and speaking. Digital human is a speaking-chain output mode because it starts after translated TTS is generated.

This change was initially explored with ZEGO, but the current product path uses Simli. Simli receives TTS PCM over its realtime session and returns a digital-human video track. Meeting audio remains the local TTS stream written to the virtual microphone; meeting video is exposed through OBS Browser Source and OBS Virtual Camera.

The product model should therefore be two business chains:

```text
Hearing chain:
meeting audio -> ASR -> translation -> answer suggestions / private monitor

Speaking chain:
physical mic -> ASR/translation/polish -> TTS -> output mode

Output mode: Virtual audio
TTS PCM -> local virtual microphone

Output mode: Digital human
TTS PCM -> Simli WebRTC -> ffmpeg decode video
         -> local OBS Browser Source -> OBS Virtual Camera
TTS PCM -> delayed local virtual microphone
```

Digital human is not a third top-level chain. It is a speaking-chain output mode that replaces the final output segment after TTS.

ZEGO remains a possible enterprise-only provider later, but it is not the default path documented for the current product. The current path is Simli video plus local TTS audio.

## Goals / Non-Goals

**Goals:**
- Keep the homepage at two business chains: hearing chain and speaking chain.
- Add a speaking-chain digital-human switch that changes output routing.
- Preserve the existing virtual-audio path when digital human is off.
- When digital human is on, use Simli as the authoritative video source and use a delayed local TTS stream as the meeting audio source.
- Move Simli-specific session and video decode concerns into `internal/digitalhuman` so `internal/speaking` stays focused on speaking-chain orchestration.
- Remove old self-developed virtual-camera code paths, configuration, preflight checks, and user-facing copy.

**Non-Goals:**
- Do not create a third homepage chain for digital human.
- Do not use the old physical-camera direct-through path as part of digital-human mode.
- Do not expose advanced provider tuning in the first version unless required for a reliable default.
- Do not require voice cloning; the speaking chain can use the currently selected TTS provider before entering the output mode.

## Decisions

### Decision: Model digital human as a speaking-chain output mode

The speaking chain owns user speech capture, ASR, translation, optional polish, TTS, and final output. Digital human starts after TTS, so it belongs at the output boundary of `speaking.Chain`, not as a separate `video.Chain`.

Alternatives considered:
- Separate video chain toggle: rejected because users would need to reason about three chains, and digital-human video cannot work independently from speaking-chain TTS audio.
- Replace the whole speaking chain: rejected because ASR/translation/TTS remain the same; only the output stage changes.

### Decision: Treat Simli video as authoritative when enabled

When digital human is enabled, the app sends TTS PCM to Simli to drive the returned digital-human video. Since Simli does not provide the local meeting audio path, the app continues writing TTS PCM to the virtual microphone but delays those writes by about 700ms so audio better aligns with the video returned from Simli.

Alternatives considered:
- Writing local TTS immediately while Simli video arrives later: rejected because it creates obvious audio/video desync.
- Provider video only with no audio: rejected because the interviewer still needs to hear the translated TTS through the virtual microphone.

### Decision: Add `internal/digitalhuman` as a provider package

`internal/digitalhuman` should own:
- Simli token/session startup.
- WebSocket / WebRTC lifecycle.
- PCM sending to drive the digital human.
- Video-track decode and frame delivery to `internal/video`.

`internal/speaking` should only select the output mode and feed synthesized PCM into an output sink.

Alternatives considered:
- Put Simli code directly in `internal/speaking`: rejected because it would mix provider protocol, WebRTC lifecycle, and frame decoding into business orchestration.
- Reuse the old video-chain abstraction: rejected because digital human is driven by speaking-chain TTS, not by a separate camera chain.

### Decision: Keep configuration split by sensitivity

Sensitive values go to Keychain:
- Simli API Key

Non-sensitive local config:
- Simli Face ID
- digital-human enabled flag
- meeting virtual camera name

The OBS browser-source URL is generated locally and shown in settings/help copy.

### Decision: Use OBS Virtual Camera instead of a self-developed driver

The app exposes digital-human video at `http://127.0.0.1:18765/` for OBS Browser Source. Users start OBS Virtual Camera and select `OBS Virtual Camera` in the meeting app. The app does not install, register, or emulate a virtual camera driver.

## Risks / Trade-offs

- [Risk] Simli video can arrive later than local TTS audio. -> Mitigation: delay local virtual-mic output by about 700ms in Simli mode and keep the delay centralized in the speaking chain.
- [Risk] WebRTC video decode can flicker or block if backpressure builds. -> Mitigation: keep decode and OBS frame publishing decoupled, publish the latest frame at a stable cadence, and log ffmpeg stderr for diagnosis.
- [Risk] OBS Virtual Camera may be unavailable until OBS is opened and macOS permissions are granted. -> Mitigation: show explicit setup guidance and do not block on meeting-app device enumeration before OBS starts.
- [Risk] Digital-human output can introduce more latency than direct virtual audio. -> Mitigation: UI should label the active output mode; diagnostics should log TTS-to-WS, WS connect, RTC pull, and virtual-device write milestones.
- [Risk] Stale self-developed virtual-camera docs or code can mislead setup. -> Mitigation: remove the old driver code path and keep README / guide / OpenSpec docs synchronized around OBS.

## Migration Plan

1. Keep digital human behind the speaking-chain output mode.
2. Keep Simli settings and digital-human toggle in the active UI.
3. Replace old `StartVideoChain` homepage usage with speaking-chain output-mode handling.
4. Remove custom virtual-camera driver implementation/tests and stale device config.
5. Verify direct virtual-audio mode still works without Simli config.
6. Verify digital-human mode fails fast with incomplete Simli config and gives OBS setup guidance when camera output is unavailable.

Rollback strategy: keep the direct virtual-audio speaking path independent of `internal/digitalhuman` so disabling the digital-human flag restores the previous speaking output behavior without needing Simli or OBS.

## Open Questions

- Should the Simli audio-delay offset remain a fixed default or become an advanced setting?
- Should OBS Browser Source dimensions remain fixed at 512x512 or follow the selected Simli avatar resolution?
- Should the app add an OBS WebSocket integration later to auto-create the browser source, or keep manual setup for reliability?
