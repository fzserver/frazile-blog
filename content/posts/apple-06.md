---
title: What I learned running a Time Machine server for the whole house
slug: time-machine-server-lessons
summary: Backups are boring until they are not. Three years of hosting Time Machine for four Macs on a home server, and the settings that stopped the "backup failed" notifications for good.
tags: apple, backups, time machine, homelab, nas
days_ago: 1
---
Every Mac in the house backs up to the same server, over Wi-Fi, without anyone thinking about it. Getting there took longer than it should have. This is the short version.

![silver and black hard disk drive](/media/2026/09/38fb16e4b7020b73.jpg)
*Photo by [Nick](https://unsplash.com/@nkend?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## Use SMB, and advertise it properly

Time Machine over a network share needs the server to say "I support Time Machine" in its SMB configuration, and to be discoverable via Bonjour. On Linux with Samba, the share looks like this:

```ini
[TimeMachine]
   path = /srv/timemachine
   valid users = @tmusers
   read only = no
   fruit:time machine = yes
   fruit:time machine max size = 800G
   vfs objects = catia fruit streams_xattr
```

Plus, in the global section:

```ini
   fruit:aapl = yes
   fruit:model = MacSamba
   vfs objects = catia fruit streams_xattr
```

And an Avahi service file so the Macs find it in the Time Machine picker. Without `fruit:time machine = yes`, the Mac will refuse the share, or worse, accept it and then produce corrupted sparsebundles later.

If the server is itself a Mac, all of this is one checkbox in *Sharing → File Sharing → Advanced Options*.

## One share per Mac, with a quota

Time Machine grows until it fills whatever it is given, then starts thinning old backups. On a shared volume that means one laptop with a large video library can squeeze everyone else out. A **per-machine quota** (the `max size` line above, or a separate volume per Mac on ZFS) keeps the household fair.

A good rule: quota of **2 to 3 times the Mac's used disk space**. Enough for months of history, not so much that thinning never happens.

## The sparsebundle will eventually corrupt

Network Time Machine backups live inside a sparsebundle, a disk image made of thousands of 8 MB band files. A dropped Wi-Fi connection mid-write can leave it unmountable. Two defences:

1. **Periodic verification.** Hold Option while clicking the Time Machine menu icon and choose *Verify Backups*. I have a reminder for the first of each month.
2. **Server-side snapshots.** Snapshot the backup share nightly (ZFS or Btrfs). When a bundle goes bad, roll back to yesterday's snapshot instead of starting from zero.

Since adding snapshots, a corrupted bundle has cost me one lost day of history instead of a full re-seed.

## Seed the first backup over a cable

The first backup of a 500 GB Mac over Wi-Fi takes a day and is the most likely one to fail. Plug in Ethernet (or a Thunderbolt-to-Ethernet adapter) for the initial run. Everything after that is incremental and Wi-Fi is fine.

![server room aisle with metal equipment racks](/media/2026/09/d27582401777bf41.jpg)
*Photo by [İsmail Enes Ayhan](https://unsplash.com/@ismailenesayhan?utm_source=frazile_blog&utm_medium=referral) on [Unsplash](https://unsplash.com/?utm_source=frazile_blog&utm_medium=referral)*

## Do not rely on it alone

Time Machine is versioning, not disaster recovery. The server sits in the same house as the Macs; a fire, a flood, or ransomware that reaches the share takes both. The backup share itself is mirrored off-site nightly with restic to an object store. Restore tests happen twice a year, because a backup you have never restored from is a hypothesis.

## The settings checklist

- Share advertises Time Machine support and is visible via Bonjour
- Per-Mac quota set
- Server-side nightly snapshots of the backup volume
- First backup done over a cable
- Monthly verification reminder
- Off-site copy of the backup share, and a restore test on the calendar

Do these and the "Time Machine couldn't complete the backup" notification becomes something you read about on forums rather than see on your own screen.

---
*Cover photo by [Arina Krasnikova](https://www.pexels.com/@arina-krasnikova) on [Pexels](https://www.pexels.com).*
