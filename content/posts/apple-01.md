---
title: Why Apple Silicon changed how I think about a homelab
slug: apple-silicon-homelab
summary: A Mac mini idles at a few watts, runs 24/7 without a fan you can hear, and still has the memory bandwidth to run local AI models. That combination quietly rewrote my server plans.
tags: apple, apple silicon, homelab, mac mini
days_ago: 21
---
For years my homelab was a pile of second-hand x86 boxes: an old desktop as the NAS, a mini PC for containers, and a gaming card doing double duty for anything AI-shaped. It worked. It also pulled around 200 W at idle, sounded like a hair dryer under load, and made the room noticeably warmer in summer.

Then I put a Mac mini on the shelf next to it and stopped noticing it existed.

![black and gray laptop computer](/media/2026/09/a11dd3194b3fcb93.jpg)
*Photo by [Roger Cai](https://unsplash.com/@hi_roger?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## The numbers that matter

The headline benchmarks are not the interesting part. What matters for a machine that runs all day is the shape of the power curve:

| State | Typical draw |
|---|---|
| Idle, display off | ~4–7 W |
| Light containers (DNS, reverse proxy, monitoring) | ~8–12 W |
| Transcoding a 4K stream | ~20–30 W |
| Sustained CPU + GPU load | ~40–65 W |

An x86 box that idles at 40 W and my Mac mini idling at 5 W differ by about **300 kWh a year**. That is real money in most countries and a lot less heat in a small room.

## Unified memory is the sleeper feature

The thing that surprised me was not efficiency, it was memory bandwidth. Apple's unified memory means the GPU cores see the same pool as the CPU, and that pool is fast. A 32 GB or 64 GB machine can hold a mid-sized language model or a Stable Diffusion checkpoint entirely in memory that the GPU can address directly.

That does not make it a replacement for a dedicated NVIDIA card. My RTX 3090 still wins comfortably on raw throughput for image generation, and CUDA remains the path of least resistance for most ML tooling. But for *always-on* inference, things like a local embedding service, a small chat model behind a private API, or Whisper transcription of voice notes, the Mac does it in the background at a power budget that a discrete GPU cannot approach.

## What I actually run on it

- **Time Machine and SMB shares** for the household Macs, which is the one job a Linux box never did quite as cleanly.
- **Docker via Colima** for the light, stateless services. macOS is not a great container host, but for a dozen small containers it is fine.
- **A local LLM endpoint** for the tooling that does not need to leave the house.
- **Media transcoding** through hardware video encoders, which are excellent for H.264 and HEVC.

The heavy lifting, big storage, torrents behind a VPN, and GPU image generation stays on Linux. The Mac took over the things that need to be quiet, reliable, and cheap to leave on.

![A wi-fi router and a network switch sit side-by-side](/media/2026/09/e00934297431c020.jpg)
*Photo by [User_Pascal](https://unsplash.com/@user_pascal?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## The trade-offs, honestly

- macOS wants to be a desktop. Auto-updates, login windows, and sleep settings all need taming before it behaves like a server.
- No ECC memory, no hot-swap storage, no IPMI. If it dies, you drive to a store.
- Docker performance on macOS lags Linux for anything I/O heavy.
- Storage upgrades are external, so a tidy setup needs a Thunderbolt enclosure.

None of those are deal-breakers for a home setup. They are just things to know before you start.

## Would I do it again?

Yes, and the follow-up post walks through exactly how I set the mini up headless, including the settings that stop it from ever showing a login screen you cannot see.

---
*Cover photo by [Arthur Lambillotte](https://unsplash.com/@artlambi?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral).*
