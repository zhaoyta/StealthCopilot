## 1. Configuration And Secrets

- [x] 1.1 Add Simli digital-human config fields for API Key, Face ID, provider, meeting virtual camera name, OBS Browser Source URL, and enabled state.
- [x] 1.2 Store Simli API Key in Keychain while keeping Face ID, provider, meeting camera name, and enabled state in local config.
- [x] 1.3 Add config validation helpers that report missing Simli credentials, Face ID, virtual microphone, and OBS setup separately.
- [x] 1.4 Add secret-safe Simli connection diagnostics without logging API Key plaintext.

## 2. Digital Human Provider Package

- [x] 2.1 Create `internal/digitalhuman` package with interfaces for session lifecycle, PCM driving, video receive/decode, and local video output.
- [x] 2.2 Implement Simli token/session startup, WebSocket connect, WebRTC SDP exchange, PCM sending, and cleanup.
- [x] 2.3 Implement PCM resampling and chunk pacing compatible with the speaking-chain TTS output.
- [x] 2.4 Implement Simli WebRTC video receive and ffmpeg VP8/H264 decode pipeline.
- [x] 2.5 Bridge decoded digital-human video frames to OBS Browser Source; keep audio on the local virtual microphone with delay compensation.
- [x] 2.6 Add unit tests for config validation, session behavior, resampling, and lifecycle cleanup on partial startup failure.

## 3. Speaking Chain Integration

- [x] 3.1 Replace the unconditional video-chain audio sink with an explicit speaking-chain output mode abstraction.
- [x] 3.2 Keep the existing direct virtual-audio output path when digital-human mode is disabled.
- [x] 3.3 Route TTS PCM to Simli for video while delaying local virtual-mic writes in Simli mode.
- [x] 3.4 Ensure Zero-PCM protection still works before the first direct-audio chunk becomes available.
- [x] 3.5 Stop and clean up digital-human WebSocket, WebRTC, ffmpeg, and OBS frame output resources when the speaking chain stops or startup fails.

## 4. Remove Old Video Chain And Self-Developed Virtual Camera Surface

- [x] 4.1 Delete self-developed virtual-camera driver implementation, tests, config fields, Wails bindings, and setup code.
- [x] 4.2 Remove old standalone video-chain references from dashboard preflight, setup status, settings pages, docs links, and i18n strings.
- [x] 4.3 Remove the homepage's separate video-chain start/stop control and present digital human only inside the speaking-chain card.
- [x] 4.4 Keep reusable video-writer abstractions, but output digital-human video through OBS Browser Source.

## 5. Frontend And UX

- [x] 5.1 Update Dashboard so it shows only hearing chain and speaking chain as top-level business chains.
- [x] 5.2 Add the speaking-chain digital-human switch/status inside the speaking card with clear labels for virtual audio vs digital-human audio/video output.
- [x] 5.3 Add settings UI for Simli digital-human credentials, Face ID, OBS URL, and meeting virtual camera name.
- [x] 5.4 Update setup completion and readiness indicators so digital human is optional unless the user enables its output mode.
- [x] 5.5 Add preflight messages that name the specific missing Simli setting, virtual microphone, or OBS setup required by digital-human mode.

## 6. Verification

- [x] 6.1 Run Go unit tests for config, digitalhuman, speaking, audio, and video packages.
- [x] 6.2 Run full Go test suite from `stealthcopilot`.
- [x] 6.3 Run frontend typecheck/build.
- [x] 6.4 Manually verify direct virtual-audio mode still starts without Simli configuration.
- [x] 6.5 Manually verify digital-human mode fails fast with incomplete Simli config and does not start partial local output.
- [ ] 6.6 Manually verify digital-human mode starts with valid Simli config and OBS setup, then writes delayed audio to virtual mic and video to OBS Virtual Camera.

## 7. Simli AI Migration (ZEGO enterprise-only, pivot to Simli)

- [x] 7.1 Add `SuppressDirectAudio() bool` to `Driver` and `DigitalHumanDriver` interfaces; ZEGO returns true, Simli and Null return false.
- [x] 7.2 Implement `SimliDriver` in `internal/digitalhuman/simli.go`: token fetch, WebSocket connect, WebRTC SDP exchange, PCM 24kHz→16kHz resampling, `SendAudio`, `Close`.
- [x] 7.3 Implement `simli_video.go`: WebRTC H.264 RTP receive via pion/webrtc, H.264 depacketization with pion/rtp, FFmpeg BGRA decode, VirtualCameraWriter output.
- [x] 7.4 Update speaking chain audio routing: when `SuppressDirectAudio()=false` (Simli), write TTS audio to virtual mic AND send to driver; EndTTS behavior also branched.
- [x] 7.5 Add `DigitalHumanProvider` (simli/zego), `SimliAPIKey` (Keychain), `SimaliFaceID` (local) to config; update validation to be provider-aware; default provider = simli.
- [x] 7.6 Update `app_bindings.go` to build `SimliDriver` or `ZegoDriver` based on `DigitalHumanProvider`; add `simliDigitalHumanConfigFromApp`.
- [x] 7.7 Frontend: add provider dropdown + Simli Face ID field to devices settings; add separate Simli help modal; add Simli AI to API Keys tab.
- [x] 7.8 Add i18n strings for Simli provider in zh-CN and en-US.
- [x] 7.9 Update `docs/guide/api-keys.md`: replace ZEGO-only section with Simli (recommended) + ZEGO (enterprise appendix).
- [x] 7.10 Add unit tests for SimliDriver: config validation, resample, SuppressDirectAudio, token error, mock WebSocket flow.
