---
title: Setting up a Mac mini as a headless home server
slug: mac-mini-headless-server
summary: The exact checklist I use to turn a fresh Mac mini into a machine that reboots on its own, never sleeps, and can be reached over SSH and Screen Sharing without a monitor attached.
tags: apple, mac mini, homelab, macos, tutorial
days_ago: 17
---
A Mac makes an excellent low-power server, but out of the box it behaves like a laptop that happens to lack a battery. This is the list I work through on every fresh install. Do it once, in this order, and the machine will happily disappear onto a shelf.

![a bunch of blue wires connected to each other](/media/2026/09/ee5ce7182b500b80.jpg)
*Photo by [Scott Rodgerson](https://unsplash.com/@scottrodgerson?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## 1. Create the admin account and enable remote access

Finish Setup Assistant with a display attached the first time. Then in **System Settings → General → Sharing**:

- Turn on **Remote Login** (SSH). Restrict it to your admin user.
- Turn on **Screen Sharing** so you can still reach the GUI when something needs clicking.
- Optionally turn on **File Sharing** if this will be your household NAS.

Copy your SSH key across before you unplug anything:

```sh
ssh-copy-id admin@macmini.local
```

## 2. Stop it from sleeping, ever

In **System Settings → Energy** (or *Displays → Advanced* on some versions):

- Prevent automatic sleeping when the display is off: **on**
- Wake for network access: **on**
- Start up automatically after a power failure: **on**

The command-line equivalent is more reliable across versions:

```sh
sudo pmset -a sleep 0 disksleep 0 displaysleep 5 womp 1 autorestart 1
```

## 3. Auto-login and the FileVault question

A headless Mac that reboots into the login window is a headless Mac you cannot reach until someone types a password. You have two choices:

- **Turn FileVault off** and enable auto-login for the admin user. Simplest, and fine if the machine lives in your home.
- **Keep FileVault on** and accept that after a power cut you will need physical access, or set up a *bootstrap token* through MDM. For most home setups this is not worth the hassle.

I keep FileVault off on the server and encrypt the external volumes instead, which keeps the sensitive data protected while letting the OS boot unattended.

## 4. Tame the updates

Automatic macOS updates will reboot your server at 2 AM while you are asleep and something is running. In **System Settings → General → Software Update → Automatic Updates**, keep *security responses and system files* on but turn off *install macOS updates*. Apply major updates on your own schedule.

## 5. Install a package manager and the basics

```sh
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
brew install colima docker docker-compose tmux htop
colima start --cpu 4 --memory 8 --vm-type vz --mount-type virtiofs
```

Colima gives you a Docker daemon backed by Apple's Virtualization framework. It is noticeably lighter than Docker Desktop and has no licence question attached.

## 6. Make services survive a reboot

Homebrew's `brew services` wraps `launchd`, which is what macOS uses instead of systemd:

```sh
brew services start colima
```

For your own scripts, write a small `launchd` plist in `~/Library/LaunchAgents` with `RunAtLoad` and `KeepAlive` set to true. It is verbose, but it is dependable.

![Green computer code text scrolling on a dark screen during a software installation](/media/2026/09/b424f0aa303b77d5.jpg)
*Photo by [Jake Walker](https://unsplash.com/@jakewalker?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## 7. Monitoring and alerts

Point your existing uptime monitor at the box and add a heartbeat: a cron job (yes, cron still works on macOS) that pings a URL every five minutes. When the ping stops, you know before the family does.

## 8. Unplug the monitor

One catch: some Mac models throttle the GPU or refuse to render a Screen Sharing session at a useful resolution with no display connected. A cheap **HDMI dummy plug** fixes it and costs less than a coffee.

That is the whole list. From here on the machine is just another host on the network, except it draws about as much power as the router.

---
*Cover photo by [Amanz](https://unsplash.com/@amanz?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral).*
