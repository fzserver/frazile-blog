---
title: Locking down a Mac beyond the defaults
slug: mac-security-beyond-defaults
summary: macOS is secure enough out of the box that most people never look further. The dozen settings below are the ones that close the gaps that remain, ordered by how much they matter and how much they cost you day to day.
tags: apple, macos, security, privacy
days_ago: 0
---
Apple's defaults are good. FileVault is offered at setup, Gatekeeper is on, the firmware is signed and verified on every boot, and app sandboxing means a bad download does not become a bad week. What follows is the list of things the defaults do not do, in the order I apply them to a new Mac. Nothing here needs a third-party product.

![A detailed close-up of a MacBook Pro keyboard, highlighting keys and reflection.](/media/2026/09/adffaf2dab440c0a.jpg)
*Photo by [Кирилл Абрамов](https://www.pexels.com/@feel-and-live) on [Pexels](https://www.pexels.com)*

## The three that matter most

**1. Turn FileVault on, and check it.** Setup Assistant asks, and people click past it. System Settings, Privacy & Security, FileVault. On Apple Silicon the internal drive is always encrypted at rest; FileVault is what ties that encryption to your password rather than to the hardware alone, so a stolen machine is a paperweight instead of a data source. Store the recovery key in the password manager, not in iCloud, and verify with:

```sh
fdesetup status
```

**2. Use a separate admin account.** The account you use every day should be a standard user. Create an admin account, then demote your own in Users & Groups. Installers ask for the admin password when they need it, which is rare, and everything that wants to change system files now has to ask a question you will notice. This one change stops more classes of problems than anything else on the list.

**3. Require a password immediately after sleep.** Lock Screen settings: "Require password after screen saver begins or display is turned off" set to Immediately. Pair it with a hot corner that starts the screen saver, so leaving the desk is a flick of the mouse. On a laptop, also set the lid to lock rather than merely sleep.

## Network and services

**Firewall on, with stealth mode.** Network, Firewall. macOS ships with it off because most Macs live behind a NAT router. On a laptop that visits coffee shops, on. Stealth mode makes the machine ignore pings and port scans.

**Turn off every sharing service you are not using.** General, Sharing. Screen Sharing, File Sharing, Remote Login (SSH) and Remote Management are all off unless the Mac is a [headless server](/p/mac-mini-headless-server), in which case only SSH, only with keys, and only from the LAN.

**AirDrop to Contacts Only**, or off when travelling. Everyone is a shortcut for receiving unsolicited files in a crowded room.

**Private Wi-Fi address on**, per network, so the laptop's hardware address is not a tracking beacon across every hotspot it joins. It is on by default for new networks now, but older saved networks keep the old setting.

## Apps and permissions

**Audit Privacy & Security once a quarter.** Full Disk Access, Accessibility, Screen Recording and Input Monitoring are the four dangerous ones. Anything in those lists can read everything, control everything, or watch everything. Apps ask for them for convenience features and never give them back. Remove what you do not recognise; the app will ask again if it truly needs it.

**Login Items and Background Items.** General, Login Items. This is where "helper" processes from uninstalled apps live on for years. Remove anything whose parent app is gone.

**Gatekeeper stays at App Store and identified developers.** The Terminal override for a single unsigned app is `xattr -d com.apple.quarantine <app>`; use that for the one tool you trust rather than lowering the setting for everything.

**Notarised or nothing for command line tools too.** Homebrew is fine, and its formulae are built from source or from signed bottles. Curl-to-shell installers from a project's README are the same as running an unsigned app, so read them first.

![Close-up of a computer screen displaying an authentication failed message.](/media/2026/09/be7608d66fc4b345.jpg)
*Photo by [Markus Spiske](https://www.pexels.com/@markusspiske) on [Pexels](https://www.pexels.com)*

## Passwords and accounts

**Passkeys where offered, a password manager everywhere else.** [Passkeys](/p/passkeys-apple-explained) remove the password from the equation for the sites that support them. For the rest, iCloud Keychain or a dedicated manager, with the built-in check for reused and leaked passwords run and acted on.

**Advanced Data Protection for iCloud.** Apple ID, iCloud, Advanced Data Protection. This makes the backups, Notes, Photos and everything else in iCloud end-to-end encrypted, with Apple no longer holding a key. The cost is that account recovery moves to you: set up a recovery contact and a recovery key first, and put the key somewhere that is not the Mac.

**Two-factor is mandatory for the Apple ID; make the trusted phone number a real one.** Recovery goes through it. A number you might lose with a carrier switch is a bad choice.

## For the paranoid, in a good way

- **Lockdown Mode** if you have reason to think you are a target. It disables a long list of convenience features, and for most people the cost is too high, but it exists and it works.
- **Disable the "Allow accessories to connect" automatic setting** so a plugged-in USB device on a locked Mac has to be approved.
- **Check the boot security policy** in Recovery on Apple Silicon: Full Security, which is the default, refuses to boot anything not signed by Apple and current.
- **Do not turn off System Integrity Protection**, whatever a forum post says. Every tool that once needed it has a supported way now.

The theme across the list is that macOS already has the mechanism for each of these. The work is a Sunday afternoon of turning things on and things off, and then nothing, which is what security should feel like day to day.

---
*Cover photo by [Dan  Nelson](https://www.pexels.com/@dan-nelson-1667453) on [Pexels](https://www.pexels.com).*
