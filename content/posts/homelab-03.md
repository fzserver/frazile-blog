---
title: The crash-loop that only happened after a restart
slug: protected-regular-docker-crash-loop
summary: A Tor container ran fine for a day, then restarted itself 995 times with "Permission denied" on a file in /tmp. The cause was a kernel hardening sysctl, and the fix was one line of compose. A debugging story.
tags: homelab, docker, linux, debugging
days_ago: 25
---
Uptime monitoring paged me about a container that had been perfectly healthy for a day. `docker ps` showed it restarting. The log was one line, repeating:

```
entrypoint.sh: line 12: can't create /tmp/torrc-defaults: Permission denied
```

The container runs its entrypoint as root. Root cannot create a file in `/tmp`. That should be impossible, which is usually a sign the bug is somewhere you are not looking.

![Woman using a laptop in a server room, showcasing modern technology and work environment.](/media/2026/09/00e111775a1cfe37.jpg)
*Photo by [Christina Morillo](https://www.pexels.com/@divinetechygirl) on [Pexels](https://www.pexels.com)*

## Reproducing it was the hard part

A fresh `docker run` of the same image worked perfectly. Deleting the container and recreating it also worked. Only *restarting the existing container* failed. So the state that mattered lived in the container's writable layer, and every clean-slate attempt was throwing away the evidence.

The trick that cracked it: **snapshot the broken container into an image** and run that.

```sh
docker commit fztor debug:snap
docker run --rm -it --entrypoint sh debug:snap
```

Inside, `/tmp/torrc-defaults` already existed, owned by the `tor` user rather than root. The entrypoint writes the file as root and then `chown`s it to the service user. On first start the file does not exist and everything is fine. On restart, the writable layer still has yesterday's file, owned by someone else, and root's attempt to rewrite it fails.

But root has `CAP_DAC_OVERRIDE`. Why would it fail?

## The sysctl

```
fs.protected_regular = 2
```

This kernel setting (on by default in Kali and increasingly in other distributions) blocks `O_CREAT` opens of a file in a **world-writable sticky directory** when the file is owned by a different user than the one opening it. It applies to root as well. It exists to stop `/tmp` file-squatting attacks, and it does exactly what it says: it stops the entrypoint from opening a file in `/tmp` that belongs to `tor`.

`/tmp` inside the container is a sticky, world-writable directory like any other, and the host's sysctl applies to it because it is the host's kernel.

A plain `touch /tmp/newfile` still worked, which is what sent me in circles for a while. The rule only blocks *existing* files owned by *another* user.

## The fix

Not disabling the sysctl. It protects the whole host, and `/tmp` in a container should be ephemeral anyway. Instead:

```yaml
services:
  tor:
    image: dockurr/tor
    tmpfs:
      - /tmp
```

With `/tmp` on tmpfs it is wiped on every start, the file never pre-exists, and the entrypoint's write-then-chown dance works every time.

![A close-up of a person typing on a keyboard in a modern tech workspace with gadgets and a monitor.](/media/2026/09/3ff741eaf3b82dd2.jpg)
*Photo by [Jakub Zerdzicki](https://www.pexels.com/@jakubzerdzicki) on [Pexels](https://www.pexels.com)*

## Auditing the rest

Once I knew the pattern I checked every other container for the same shape: an entrypoint that creates something in `/tmp` and hands it to a different user. Only the one matched. A Nextcloud container had non-root files in `/tmp`, but they were PHP sessions written by the same user that reads them, so no conflict.

## The lessons

1. "Works on a fresh start, fails on restart" means the writable layer is the suspect. `docker commit` lets you inspect it.
2. When root gets `EACCES` on something root should be able to do, look for LSMs and hardening sysctls before anything else.
3. Ephemeral directories should be declared ephemeral. `tmpfs: [/tmp]` costs nothing and removes a whole class of surprises.

---
*Cover photo by [Tima Miroshnichenko](https://www.pexels.com/@tima-miroshnichenko) on [Pexels](https://www.pexels.com).*
