---
title: Shortcuts automations that run without me
slug: shortcuts-automations-hands-off
summary: Shortcuts is usually shown as a way to press a button that does five things. The interesting half is automations: shortcuts that fire on a time, a location, a Wi-Fi network or a device state, with no tap at all. These are the ones running on my devices right now, and how each is built.
tags: apple, shortcuts, ios, automation, productivity
days_ago: 0
---
For years I ignored Shortcuts because the examples were all "make a widget that texts your partner". The app became useful the day I found the Automation tab and realised the triggers are the feature. Since iOS 17 personal automations run without a confirmation prompt, which means a shortcut can react to the world instead of waiting for me. Here are the ones that have survived more than a month.

![Three smartphones connected to chargers on a wooden surface, showcasing modern technology](/media/2026/09/322bc19ba2a4a175.jpg)
*Photo by [Stanley Ng](https://www.pexels.com/@stanley-ng-2850879) on [Pexels](https://www.pexels.com)*

## Home network: unlock the homelab

**Trigger:** Wi-Fi, when joining the home network. **Run immediately.**

The shortcut sets a variable in a small file in iCloud Drive that other shortcuts read, turns Low Data Mode off, and opens nothing. Its only visible effect is that the Jellyfin and monitoring shortcuts below know they are on the LAN and use local addresses instead of going out through the tunnel and back in. The inverse automation, on leaving the network, flips the flag back.

This sounds like more work than it is worth until the day the tunnel is down and everything at home still works from the phone.

## Charger in, media server check

**Trigger:** Charger, when connected, between 22:00 and 02:00. **Run immediately.**

Calls the [Uptime Kuma](/p/uptime-kuma-ntfy-monitoring) status API with Get Contents of URL, parses the JSON with Get Dictionary Value, and if anything is down shows a notification with the list. Otherwise silent. The phone goes on the charger every night; this turns that into a scheduled health check I cannot forget, without a single notification on a good night. Time-of-day conditions in a Shortcuts automation are done with an If on the Current Date formatted as hour, which is clumsy but works.

## Camera roll to the NAS

**Trigger:** Time of day, 03:00, daily. **Run immediately.**

Find Photos where Date Taken is in the last day, then for each, Save File to a folder that is a Files-app location backed by SMB to the NAS. Photos still go to iCloud too; this is the second copy that [Time Machine](/p/time-machine-server-lessons) is not responsible for. The SMB connection in Files has to be set up once by hand and stays. Videos over a size limit are skipped with a Filter action, since a 4K minute at 3 a.m. over Wi-Fi is a good way to make the automation fail its time budget.

## Focus modes as the switchboard

Focus modes are the closest thing iOS has to a global state, and automations can both set them and react to them.

**Trigger:** When Work Focus turns on. Sets the Watch face to the plain one, turns on Do Not Disturb on the Mac via Focus sync, and opens nothing. **Trigger:** When Sleep Focus turns on. Sets brightness to minimum, turns on Night Shift, enables Low Power Mode, and starts a rain sound in the background with a timer to stop playback after 40 minutes.

The reason to route through Focus rather than time is that the Focus itself is already scheduled, already syncs across devices, and already knows about exceptions like a calendar event. The automation only handles what Focus cannot do on its own.

## Leaving home: what did I forget

**Trigger:** Location, when I leave home. **Run immediately.**

Checks HomeKit for any lights on and the desk lamp's smart plug, turns them off, and sends a single notification if any door sensor is open. It uses Get the State of Home actions rather than blind "turn off" commands, so it stays quiet when there is nothing to do. Location triggers have a delay of a minute or two and occasionally misfire on a large property, but for a flat they are reliable.

![Illuminated LED smart bulb with accessories on a vibrant orange background.](/media/2026/09/9be3f4e940da001a.jpg)
*Photo by [Jakub Zerdzicki](https://www.pexels.com/@jakubzerdzicki) on [Pexels](https://www.pexels.com)*

## The ones that did not survive

- **Battery below 20 percent, enable Low Power Mode.** iOS asks about this itself; the automation added nothing.
- **Open the parking app when disconnecting from CarPlay.** Fired every time the car was turned off in the garage too. A location condition fixed it and then it fired late. Deleted.
- **Log every app open to a spreadsheet.** Worked, and produced data I never looked at.

## Tips that apply to all of them

- **Run Immediately** is the setting that makes an automation an automation. With it off, the phone asks first, and you will say no out of habit.
- Put shared logic in a normal shortcut and call it with **Run Shortcut** from each automation. Editing one place beats editing six.
- A **Show Notification** action with a variable in it is the whole debugging toolkit. Add one, run the automation, read the value, remove it.
- Automations that call URLs on the home network need the Wi-Fi flag above, or a Tailscale-style VPN that is always on, or they will fail outside the house and iOS will not tell you why.
- Keep an eye on the **Automation** list itself. Disabled ones and duplicates from restoring a backup accumulate, and two copies of the same location trigger are a mystery you do not want to debug at a car park.

None of these is clever. Every one of them removes a small thing I used to do by hand, and the total is a phone that handles the boring parts of the day on its own, which is what a computer in your pocket was supposed to be for.

---
*Cover photo by [Castorly Stock](https://www.pexels.com/@castorlystock) on [Pexels](https://www.pexels.com).*
