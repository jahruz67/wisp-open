# macOS installation (Apple Silicon preview)

The macOS build supports Apple Silicon Macs running macOS 13 or newer. Intel Macs are not supported.

## Install

1. Download `wis-free-v3_<version>_macos-arm64-preview.dmg` from the release.
2. Open the disk image and drag **WIS Free V3** to **Applications**.
3. Because the preview is not yet signed with an Apple Developer ID, Control-click the app, choose **Open**, then confirm **Open**. Do not bypass Gatekeeper for a copy obtained from anywhere except this repository's release page.
4. Follow the in-app setup card to allow Microphone and Accessibility access.

Accessibility is used to send Command-V to the application receiving the transcription. WIS does not request Input Monitoring, Screen Recording, or Automation permission.

## Local Whisper

No local model is included with the application and no model downloads automatically. Open Settings, choose Tiny, Base, Small, or Medium under **Offline Whisper**, and click **Install Offline Whisper**. Only that selected model is downloaded. Uninstalling the offline model does not remove the application.

The current preview uses an ad-hoc signature. macOS may ask you to grant privacy permissions again after installing a new preview build. A normal signed and notarized release requires an Apple Developer Program account and will replace this workflow when signing credentials become available.
