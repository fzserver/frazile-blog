---
title: Self-hosting image generation on an RTX 3090 with ComfyUI in Docker
slug: comfyui-rtx-3090-docker
summary: A used 24 GB card, the NVIDIA container toolkit, a compose file and a models directory on the NAS. Notes on getting ComfyUI running as a proper service, including the kernel-versus-driver problem on a rolling distribution.
tags: homelab, ai, comfyui, nvidia, docker
days_ago: 6
---
Cloud image generation is convenient until you want to iterate on a workflow for an evening, at which point per-image pricing starts to feel like a meter running. A second-hand **RTX 3090** with 24 GB of VRAM has become the sensible way to run this at home: it is old enough to be affordable and has more memory than most current consumer cards.

Here is how it is set up as a service on the homelab rather than a desktop app.

![A high-performance gaming PC with vibrant RGB lighting and visible internal components.](/media/2026/09/52a83a17664a3358.jpg)
*Photo by [Ivelin Donchev](https://www.pexels.com/@ivaivo) on [Pexels](https://www.pexels.com)*

## The driver problem on a rolling distribution

The server runs Kali, which tracks Debian testing and ships a very recent kernel. NVIDIA's packaged driver in the distribution repos did not build against it. What worked:

1. Add NVIDIA's own Debian repository (the current one for Debian 13).
2. Install the **open kernel modules** (`nvidia-open`) rather than the proprietary ones; they keep up with new kernels faster and support everything from Turing onwards.
3. Install `nvidia-container-toolkit` and run `nvidia-ctk runtime configure --runtime=docker`.
4. Reboot, then `nvidia-smi` on the host and `docker run --rm --gpus all nvidia/cuda:12.4.0-base-ubuntu22.04 nvidia-smi` in a container.

If the second command fails but the first works, the container toolkit is the problem; if both fail, it is the kernel module. That ordering saved me a lot of guessing.

## The compose file

```yaml
services:
  comfyui:
    image: ghcr.io/ai-dock/comfyui:latest
    container_name: fzcomfy
    restart: unless-stopped
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
    environment:
      CLI_ARGS: "--listen --preview-method auto"
    volumes:
      - /mnt/nas/models:/opt/ComfyUI/models
      - ./workflows:/opt/ComfyUI/user/default/workflows
      - ./output:/opt/ComfyUI/output
    ports:
      - "127.0.0.1:8188:8188"
```

Two decisions worth explaining:

- **Models live on the NAS, not in the container.** Checkpoints are 2 to 12 GB each and the collection grows fast. A NAS directory bind-mounted read-write means the image can be rebuilt freely and models are shared with any other tool that wants them.
- **The port binds to localhost only.** ComfyUI has no authentication. It is reachable through the tunnel with an access policy in front, never directly.

## VRAM budgeting

24 GB sounds like a lot until you stack things up. Rough numbers for what fits at once:

| Setup | VRAM |
|---|---|
| SDXL base, fp16 | ~7 GB |
| SDXL + refiner + a couple of ControlNets | ~14 GB |
| Flux dev, fp8 | ~13 GB |
| Flux dev, fp16 | ~24 GB (tight) |
| Any of the above + an upscaler pass | +2 to 4 GB |

The 3090's memory bandwidth is what makes it hold up against newer cards; it is not the fastest, but it rarely has to swap to system RAM, which is the thing that actually makes generation slow.

![Digital drawing setup with laptop and tablet for modern art creation.](/media/2026/09/20422d9e5ec44260.jpg)
*Photo by [Daniele](https://www.pexels.com/@daniele-2103096) on [Pexels](https://www.pexels.com)*

## Running it like a service

- `restart: unless-stopped` and a healthcheck on `/system_stats`.
- Output directory on the NAS so results survive a container rebuild.
- Workflows saved as JSON in git, which makes "the portrait workflow from last month" reproducible.
- Power limit set with `nvidia-smi -pl 280`. The card loses a few percent of speed and runs 15 °C cooler, which matters in a closet.

## What it is used for

Wallpaper sets for the wallpaper site, cover images for posts like this one, and a fair amount of experimenting. The nice part of self-hosting is that the experimenting is free after the card is paid for, which changes how willing you are to try the fiftieth variation.

---
*Cover photo by [Matheus Bertelli](https://www.pexels.com/@bertellifotografia) on [Pexels](https://www.pexels.com).*
