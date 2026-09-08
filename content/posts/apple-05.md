---
title: Continuity features I actually use every day
slug: apple-continuity-daily
summary: Apple's cross-device features range from party tricks to genuinely load-bearing. After a few years of a mixed Mac, iPhone and iPad desk, these are the ones that stuck and the settings that make them reliable.
tags: apple, macos, ipad, productivity
days_ago: 4
---
Apple demos Continuity as a lifestyle: a photo taken on an iPhone floating onto a Mac, a phone call answered from a laptop. In practice some of it is delightful and some of it you try once. This is the list that survived contact with real work.

![Apple Macbook Air and iPad on table](/media/2026/09/313090f0d58ee30d.jpg)
*Photo by [Jonathan Francisca](https://unsplash.com/@jonathan_francisca?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## Universal Clipboard

Copy on one device, paste on another. That is it, and it is the feature I would miss most. Copy a two-factor code from Messages on the iPhone and paste it in a Mac browser; copy a URL on the Mac and paste it into a note on the iPad. It just works, provided both devices are on the same Apple ID with Bluetooth and Wi-Fi on.

*Tip:* it times out after a couple of minutes, so paste promptly.

## Universal Control

One keyboard and trackpad across a Mac and an iPad sitting next to it, with the cursor sliding between screens. I use an iPad as a second display for reference material, but unlike Sidecar the iPad keeps running iPadOS, so I can drag a file from the Mac straight into an iPad app.

Enable it in *System Settings → Displays → Advanced* on the Mac and *Settings → General → AirPlay & Continuity* on the iPad.

## iPhone Mirroring

Your iPhone appears as a window on the Mac, controllable with the Mac's keyboard and mouse, while the phone stays locked in your bag. It is the sane way to deal with apps that exist only on the phone: banking apps, a smart-home app, that one delivery tracker. Notifications from the phone show up in the Mac's Notification Centre and open the mirror when clicked.

## Continuity Camera

The iPhone as a Mac webcam, wirelessly or over USB. The image quality embarrasses every built-in laptop camera, and **Desk View** (a top-down view of your desk from the same wide lens) is oddly useful for showing a sketch on a call. Mounts are cheap; the feature is free.

## Handoff, selectively

Handoff moves an in-progress task between devices: a Safari page, a Mail draft, a Note. It is excellent for reading (start an article on the phone, finish on the Mac) and I have turned it off for most other apps to keep the Dock icon from twitching all day. Per-app control is not available, but disabling it on the iPad alone removes most of the noise.

## AirDrop, still

The oldest feature on the list and still the fastest way to move a large video between two Apple devices in the same room. Set receiving to *Contacts Only* and forget about it.

![A smartphone, a smartwatch, and wireless earbuds on a wooden surface](/media/2026/09/ccf1ada29bb68c00.jpg)
*Photo by [Dennis Brendel](https://unsplash.com/@dnnsbrndl?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## Things I turned off

- **Phone calls on Mac.** Ringing everywhere is worse than ringing in one place.
- **Text Message Forwarding to the iPad.** Same reason.
- **Handoff for Messages.** The phone is already in reach.

## Making it reliable

Nearly every Continuity failure I have had came down to one of three things:

1. **Bluetooth off** on one device. Everything except iPhone Mirroring over USB needs it.
2. **Different Apple IDs**, usually a work account signed in on one machine.
3. **Firewall or VPN** on the Mac blocking local discovery. Continuity uses peer-to-peer Wi-Fi, so a VPN client that forces all traffic through a tunnel can break it. Split tunnelling fixes that.

If all three check out and it still misbehaves, toggling Wi-Fi off and on both devices resolves it more often than I would like to admit.

---
*Cover photo by [Julian Steenbergen](https://unsplash.com/@mamanofficial?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral).*
