---
title: Jellyfin on Apple devices: Swiftfin, Infuse and the browser
slug: jellyfin-apple-clients
summary: The server side of Jellyfin is a solved problem here. The client side on Apple TV, iPhone, iPad and Mac is where the choices are. Which app does what, how to avoid transcoding on the server entirely, and the settings that make a household stop asking why the film is buffering.
tags: apple, jellyfin, apple tv, media server, ios
days_ago: 0
---
The [media server](/p/frazile-media-server) has been stable for a long time. What changed the experience most in the past year was not on the server; it was picking the right client for each Apple device and configuring it so the server never has to transcode. This post is that map.

![iPad and iPhone showing a scenic landscape on a wooden table, highlighting portability and technology.](/media/2026/09/1854296e0df99027.jpg)
*Photo by [Alexander Pöllinger](https://www.pexels.com/@alexander-pollinger-137430820) on [Pexels](https://www.pexels.com)*

## Three clients, three jobs

**Swiftfin** is the official Jellyfin app for iOS, iPadOS and tvOS. It is free, open source, and has caught up with most of what people used to want from third-party apps: a native tvOS interface, direct play of nearly everything, subtitle styling, and per-user settings that follow the server's. It is the default on the Apple TV.

**Infuse** is a paid third-party player that connects to Jellyfin as one of many sources. Its strength is the player itself: it decodes everything on device, including formats Apple's own player will not touch, with excellent subtitle rendering and Dolby Vision profile 7 handling that Swiftfin still cannot match on some files. It is what runs on the living room Apple TV where the 4K remuxes get watched.

**The browser** is the fallback on the Mac. Jellyfin's web client in Safari handles direct play of H.264 and HEVC and is fine for a laptop on the couch. For anything more demanding on the Mac, Infuse has a macOS version and it behaves exactly like the tvOS one.

Phones and iPads get Swiftfin. They are mostly used for downloads and for finishing an episode in bed, and the official app's download manager is the best of the three.

## Stop transcoding at the source

The server has hardware transcoding, but the goal is to never use it. Transcoding costs CPU or GPU on the server, adds seconds of startup delay, and lowers quality. Every case where an Apple client asks for a transcode has a fix on one side or the other.

**Audio is the usual culprit.** Apple devices direct play H.264 and HEVC video without complaint; what triggers a transcode is DTS or TrueHD audio that the Apple TV's own player does not decode. Options, in order of preference:

1. Use Infuse, which decodes DTS and TrueHD on device and outputs PCM or passes through to the receiver.
2. In the arr apps' quality profiles, prefer releases with AC3 or EAC3 audio. Most releases carry an AC3 track alongside the lossless one anyway.
3. As a last resort, let Jellyfin transcode audio only. In the server's playback settings, audio-only transcoding is cheap and does not touch the video stream.

**Subtitles are the second.** Image-based subtitles (PGS from Blu-ray) force a video transcode when the client cannot render them. Infuse and Swiftfin both render PGS on device. The web client in Safari does not, so on the Mac it is Infuse or an SRT track.

**Bitrate limits are the third.** Every client has a maximum streaming bitrate setting, and a low default silently forces transcodes of high-bitrate 4K files. On the home network set it to the maximum. On mobile data set it to something honest, and Jellyfin will transcode, which is the one case where that is the right answer.

## Settings that made a difference

On the **Apple TV**, in Settings, Video and Audio: **Match Content** for both frame rate and dynamic range. Without it every 24 fps film is judder-converted to 60 Hz and every HDR film plays in whatever the TV was last set to. Infuse and Swiftfin both trigger the switch.

In **Infuse**: set Jellyfin as the only library source, turn on **Direct Play** with "Never" for transcoding, and let it fetch metadata from the server rather than its own scrapers so the artwork matches what everyone else sees. Its audio setting of "Auto" with passthrough enabled sends bitstream audio to the receiver and PCM to a TV without one.

In **Swiftfin**: the native player, not the compatibility one, and subtitle size at 20 or so on the TV. Enable **Auto-play next episode** with a ten-second delay; the default off setting is the source of half the "can you make it play the next one" requests.

On the **server**, per-user: set the household accounts to "Allow media playback" with transcoding disabled for the ones on the same network, so a misconfigured client fails loudly with an unsupported-format message rather than silently eating the GPU.

![Comfortable settee with colorful cushions placed near big windows against contemporary television set in spacious room of modern stylish apartment](/media/2026/09/10c36f043b2082f2.jpg)
*Photo by [Max Vakhtbovych](https://www.pexels.com/@artbovich) on [Pexels](https://www.pexels.com)*

## A note on Plex

Plex's Apple apps are polished and it is what most people compare Jellyfin to. The thing that keeps this house on Jellyfin is that nothing depends on an account with someone else's company: the Apple TV app talks to the server on the local network, and if the internet goes down, so does exactly nothing. With Infuse in front, the player quality argument disappears too.

## What is still rough

Swiftfin's tvOS interface is good and improving quickly, but its handling of the Siri Remote's swipe gestures during playback still catches people out. Infuse's Jellyfin integration occasionally loses watched state when two devices are playing the same profile. And there is no truly good Jellyfin client for watchOS, though it is hard to say what one would be for. Everything else works, every day, with the server barely noticing.

---
*Cover photo by [Image Hunter](https://www.pexels.com/@image-hunter-281453274) on [Pexels](https://www.pexels.com).*
