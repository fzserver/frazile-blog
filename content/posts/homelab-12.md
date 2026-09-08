---
title: Jellyfin, properly: users, transcoding, clients and the settings that matter
slug: jellyfin-settings-that-matter
summary: Jellyfin works out of the box and works much better with about a dozen deliberate changes. Users and parental controls, hardware transcoding, the clients worth installing, metadata and library layout, remote access through the tunnel, and what to back up.
tags: homelab, jellyfin, media server, self-hosting
days_ago: 0
---
The [media server overview](/p/frazile-media-server) explained where Jellyfin sits in the stack. This is the post about Jellyfin itself: the choices that make the difference between "it plays files" and "the family stopped asking how to use it".

![A close-up of a TV remote on a textured sofa in a cozy living room setting.](/media/2026/09/77c752dd28dc83cb.jpg)
*Photo by [Kamil Čičila](https://www.pexels.com/@cicosvk) on [Pexels](https://www.pexels.com)*

## Libraries: fewer, cleaner

Jellyfin's library types drive everything else, so start there. Mine:

| Library | Type | Folder |
|---|---|---|
| Films | Movies | `/media/movies` |
| Series | Shows | `/media/tv` |
| Music | Music | `/media/music` + `/media/music-archive` |
| Kids | Movies + Shows | a separate folder Radarr/Sonarr write to via a second root |
| Home videos | Home Videos & Photos | `/media/home` |

Two rules. **One content type per library**, so metadata providers are not guessing. **Media mounted read-only** (`:ro` in the compose file), so nothing in Jellyfin can ever move or delete a file.

In each library's settings: *Real time monitoring* on, because the arr apps also trigger a refresh and the double coverage is harmless. **Metadata savers off**: Jellyfin does not need to write `.nfo` files next to the media, and on a read-only mount it cannot.

## Metadata that stays put

Jellyfin fetches artwork and descriptions from TMDB, TheTVDB and MusicBrainz. Two settings prevent surprises:

- **Lock** metadata on anything you edit by hand, or the next refresh will overwrite it.
- Set the **preferred language and country** per library. A library that mixes languages will otherwise pick metadata in whichever language the provider returned first.

The `.ignore` file trick from [the database post](/p/jellyfin-database-ssd) keeps the scanner out of caches and thumbnail folders that live inside the media tree.

## Users, profiles and parental controls

Every person gets their own account. That is what makes watch progress, "continue watching" and recommendations personal. On each user:

- **Library access**: choose exactly which libraries they see. The kids' profile sees the Kids library and nothing else.
- **Max parental rating** as a second line of defence, using the ratings that came with the metadata.
- **Allow media deletion: off** for everyone, including me. Deletion is a job for the arr apps or a shell, not a remote control.
- **Remote connections** on only for the accounts that need them.
- A **PIN** on the TV apps for the adult profiles, since the living-room TV is shared.

An "auto-login" profile on the TV with a limited library is the option that finally made guests stop asking for the password.

## Transcoding: use the hardware

Direct play is always the goal: the client gets the original file and Jellyfin just serves bytes. Transcoding happens when a client cannot play the codec, container or subtitle format, or when bandwidth is limited. On a small server that is where CPU goes to die.

*Dashboard → Playback → Transcoding*:

- **Hardware acceleration**: VA-API on an Intel or AMD iGPU, NVENC on an NVIDIA card. Pass the device through in compose (`/dev/dri` for VA-API, the NVIDIA runtime for NVENC).
- Enable **hardware decoding** for H.264, HEVC and VP9, and **hardware encoding**.
- **Tone mapping** on, so HDR content looks right when transcoded to SDR for a phone.
- **Transcoding path** on fast storage, ideally tmpfs, since segments are written and deleted constantly.
- Cap the **encoder preset** at *fast* or *veryfast*; the quality difference on a phone is invisible and the CPU difference is not.

Verify it with the **Dashboard → Active Devices** panel during playback. It shows whether a stream is direct playing, direct streaming (remuxing) or transcoding, and why. Most transcodes turn out to be caused by subtitles: burning in PGS subtitles forces a full video re-encode. Switching those releases to SRT subtitles, or telling the client to prefer text subtitles, removes most of them.

## Clients worth installing

- **Jellyfin Media Player** on desktops: it bundles a proper player and direct-plays nearly everything.
- **The official app on Android TV and Fire TV**: plays most formats natively, handles the PIN profiles.
- **Swiftfin on Apple TV and iOS**: the best option on Apple platforms, and it can use the native player for HEVC and Dolby Vision.
- **Findroid** on Android phones for offline downloads.
- **Finamp** for music, since the main apps treat music as an afterthought.

On each client, the one setting to check is **maximum bitrate**. The defaults are conservative and cause transcodes on a home network that could direct play.

## Remote access through the tunnel

Jellyfin sits behind the Cloudflare Tunnel with the origin invisible. Two settings to match that:

- *Dashboard → Networking*: set **Published server URI** to the public address, and add the tunnel's host to **Known proxies** so client IPs are logged correctly.
- Leave **automatic port mapping (UPnP) off**. Nothing should be opening router ports.

Cloudflare's free tier limits streaming through the tunnel in practice, so remote playback works but heavy 4K use from outside the house goes over a VPN into the home network instead. That is a trade I am happy with.

## Plugins, sparingly

- **Intro Skipper** detects and skips opening credits on series. The one plugin everyone in the house notices.
- **Trakt** to sync watch history.
- **Playback Reporting** for a "what actually gets watched" dashboard, which is what unmonitors films in Radarr later.

Every plugin is a scan-time cost and a potential breakage on upgrade. Three is enough.

![Man relaxing on sofa using tablet for video call at home.](/media/2026/09/1b08cc027e206ad0.jpg)
*Photo by [Vitaly Gariev](https://www.pexels.com/@silverkblack) on [Pexels](https://www.pexels.com)*

## What to back up

- `config/` (the database, users, settings, plugin config) nightly. It is small and it is everything.
- `metadata/` if you have hand-edited a lot; otherwise it can be regenerated.
- Not `cache/` or the transcoding folder.
- Not the media, which has its own backup story.

Restore is: new container, same compose file, restore `config/`, point at the same mounts. It has been tested. It took eleven minutes.

## The order to do things in

Libraries → users → transcoding → clients → remote access. Each step is easier when the one before it is right, and the first two are the ones that change how the server feels to everyone who uses it.

---
*Cover photo by [Ron Lach](https://www.pexels.com/@ron-lach) on [Pexels](https://www.pexels.com).*
