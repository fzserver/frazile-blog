---
title: Self-hosting e-mail from a home connection: what actually stops you
slug: self-hosting-email-home-connection
summary: Running a mail server at home is not hard because of the software. It is hard because of reverse DNS, a dynamic IP, and the reputation systems that decide whether anyone will accept your messages. What I found by testing rather than assuming, and the shape of the setup that works.
tags: homelab, email, networking, self-hosting, docker
days_ago: 0
---
"Do not self-host e-mail" is the most repeated advice in self-hosting, and it is repeated by people who mean well. It is also mostly about one thing, delivery, and delivery is a problem you can measure before you build anything. This post is what I measured from a home connection, what it ruled out, and the layout that came out the other side.

![Detailed image of Ethernet cables and connectors, ideal for tech themes.](/media/2026/09/cc886965f0ea809c.jpg)
*Photo by [Dmitry Sidorov](https://www.pexels.com/@dmitry-sidorov-2775764) on [Pexels](https://www.pexels.com)*

## Test the connection before choosing software

Three facts decide whether direct sending is even possible, and each takes a minute to check.

**Is outbound port 25 open?** Many ISPs block it. Mine does not:

```sh
nc -vz gmail-smtp-in.l.google.com 25
```

A `220` banner means the server on the other side is willing to talk. Try two or three large providers, since some ISPs block only some destinations.

**Does the public IP have a PTR record?**

```sh
dig -x $(curl -s https://ipinfo.io/ip) +short
```

Mine returns nothing. No reverse DNS at all. Gmail and Outlook treat mail from an address with no PTR as spam at best and reject it at worst, and there is no way to set a PTR on a residential line; only the ISP can.

**Is the IP on a blocklist?** Check Spamhaus ZEN. Mine was clean, not even on the policy list that covers most residential ranges, which is unusual and did not change the conclusion.

**Is the IP static?** No. It changes every few weeks, which is why a small container already exists here to update DNS records when it does. A mail server on a dynamic IP can receive fine; it cannot build a sending reputation, because reputation is attached to the address.

Two out of four went against direct sending, and the two that did are the ones that matter. Deciding this on day one saved the weeks people spend fighting spam folders.

## The layout that works: receive direct, send through a relay

Inbound mail does not care about any of the above. Other servers look up the MX record, connect to port 25, and deliver. Dynamic IP is handled by DNS updates; PTR is irrelevant for receiving. So the server at home holds the mailboxes, runs IMAP for the clients, and accepts mail on 25.

Outbound goes through a **smarthost**: a transactional mail provider (Resend, SMTP2GO, Brevo, Mailgun, any of them) that accepts authenticated submissions and delivers from its own well-kept IPs. The domain's SPF record lists the relay, DKIM signing happens at home with the public key in DNS, DMARC is set to quarantine. To the receiving side the mail comes from an established provider; to me it is my server with my archive and my rules.

The cost is a dependency on the relay for sending and a free tier that is more than enough for personal volume. The benefit is that "will this land" stops being a question.

## The software

docker-mailserver, in one container: Postfix, Dovecot, rspamd for filtering and DKIM, fail2ban. Compose is short:

```yaml
services:
  mailserver:
    image: ghcr.io/docker-mailserver/docker-mailserver:latest
    hostname: mail.example.org
    ports: ["25:25", "143:143", "465:465", "587:587", "993:993"]
    environment:
      ENABLE_RSPAMD: 1
      ENABLE_OPENDKIM: 0
      ENABLE_OPENDMARC: 0
      ENABLE_CLAMAV: 0
      ENABLE_FAIL2BAN: 1
      SSL_TYPE: letsencrypt
      RELAY_HOST: smtp.relay.example
      RELAY_PORT: 587
    volumes:
      - ./mail-data:/var/mail
      - ./mail-state:/var/mail-state
      - ./config:/tmp/docker-mailserver
      - ./certs:/etc/letsencrypt:ro
    cap_add: [NET_ADMIN]
    restart: always
```

Mailcow was the other candidate and is excellent, but it is a dozen containers and its own way of doing everything. For one domain and a few mailboxes, one container with files I can read in the config directory won.

![A detailed view of neatly stacked brown envelopes showcasing organization.](/media/2026/09/02687908108dac5a.jpg)
*Photo by [serdar barış](https://www.pexels.com/@serdar-baris-2093847029) on [Pexels](https://www.pexels.com)*

## Things the tunnel cannot do for you

Everything else on this server reaches the internet through a Cloudflare Tunnel and gets its TLS from Cloudflare. Mail is the first service here that needs real ports and a real certificate:

- The mail hostname must be a **DNS-only** A record. A proxied record breaks SMTP and IMAP entirely, since Cloudflare only proxies HTTP.
- Port 25, 465, 587, 143 and 993 need forwarding from the router to the box. This is the one place in the homelab with open ports.
- The certificate has to come from a DNS-01 challenge, because inbound port 80 is not available. Certbot with the Cloudflare plugin and a scoped token that can edit this one zone's DNS handles it, and the container reloads when the files change.

## Would I recommend it

For a domain that receives real mail today, no, not without a migration plan and a week of watching logs. For a spare domain, as a way to own the archive and learn what every header means, yes, with the relay from the start. The software takes an afternoon. The decisions above are what take the time, and they are all decidable before the first `docker compose up`.

---
*Cover photo by [Vika Glitter](https://www.pexels.com/@vika-glitter-392079) on [Pexels](https://www.pexels.com).*
