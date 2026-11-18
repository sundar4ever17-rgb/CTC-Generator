# Chromium sandbox profile

`chromium-seccomp.json` is derived from Microsoft Playwright v1.64.1's [Docker seccomp profile](https://github.com/microsoft/playwright/blob/v1.64.1/utils/docker/seccomp_profile.json), used under the adjacent Apache 2.0 license. The upstream file's SHA-256 before modification was `cc3e61cabda6bbc1e53e54d27ba4d55a9d3be829b6dd1a596f4a7b31b1cc7849`.

The single local change adds `chroot` to the unconditional allow rule for Chromium's namespace sandbox. With all outer container capabilities dropped, the upstream conditional `CAP_SYS_CHROOT` rule does not allow this operation inside the new user namespace. Kernel capability checks still apply; this does not grant a host capability.

The profile defaults to denying unlisted system calls, permits the Chromium user namespace sandbox and retains the other upstream restrictions. Production runs as uid 10001 with all container capabilities dropped, no privilege escalation, a read-only root filesystem and writable temporary memory. It does not use privileged mode, SYS_ADMIN, unconfined seccomp, or `--no-sandbox`.

The host must permit unprivileged user namespaces for this application. Run `scripts/test-browser-sandbox.ps1` on the actual host before deployment. Re-run after kernel, Docker, AppArmor, Chromium or seccomp-profile upgrades. The profile is a reviewed snapshot, so review upstream changes when upgrading rather than automatically downloading a replacement at startup. [Official container guidance](https://playwright.dev/docs/docker) describes why the extra namespace calls are needed.
