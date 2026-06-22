## Why

The desired product behavior is a speaking-chain output mode: when digital human is enabled, translated TTS audio should drive a digital human and the resulting video should enter the meeting as the user's camera. The implementation has pivoted from ZEGO to Simli because the current workable path is Simli WebRTC video plus OBS Virtual Camera output, while meeting audio remains the local TTS stream.

## What Changes

- Add a Simli digital-human video output mode inside the speaking chain.
- When the speaking-chain digital-human switch is off, keep the current virtual-audio output path: translated TTS audio goes to the local virtual microphone.
- When the switch is on, route TTS PCM into Simli WebRTC driving, decode the returned video locally, and expose it through an OBS Browser Source.
- Keep local TTS audio on the virtual microphone, delayed by about 700ms in Simli mode to reduce audio/video desync.
- Require OBS App + OBS Virtual Camera for meeting camera output; do not register a custom virtual camera driver.
- Update the homepage so only hearing chain and speaking chain are presented as top-level business chains; digital human is a speaking-chain output switch/status, not a third chain.
- Keep Simli settings for API Key and Face ID.
- Remove the old self-developed virtual camera path and separate homepage video-chain control.

## Capabilities

### New Capabilities
- `digital-human-output`: Covers Simli session startup, TTS audio driving, returned video decoding, OBS Browser Source output, local audio delay compensation, and fallback behavior.

### Modified Capabilities
- `tts-output`: The speaking chain gains output-mode routing after TTS, where TTS can write directly to virtual audio and optionally drive Simli video.
- `settings-panel`: Settings must expose Simli digital-human configuration and the OBS output URL/status.
- `simli-provider`: Simli provider behavior is active as the default digital-human video provider.

## Impact

- Backend: `internal/speaking`, `internal/digitalhuman`, `internal/audio`, `internal/video`, config/keyring services, Wails bindings.
- Frontend: `Dashboard.vue`, settings API-key/device/advanced panels, setup completion/status views, i18n strings.
- External services: Simli AI, Xunfei ASR / translation / TTS, DeepSeek where configured.
- Local devices/apps: virtual microphone remains required for speaking output; OBS App and OBS Virtual Camera are required only when digital-human video is enabled.
- Migration: custom virtual-camera driver code and config are removed; users select OBS Virtual Camera in their meeting app.
