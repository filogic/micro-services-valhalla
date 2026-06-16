#!/usr/bin/env python3
"""Send the daily toll-benchmark summary email.

Credential-agnostic: reads Secret Manager `benchmark-smtp` (a small JSON blob)
and sends via either Postmark or SMTP, so the delivery method can be chosen by
populating the secret — no code change needed.

Secret formats (one of):
  {"type":"postmark","token":"<server-token>","from":"ops@filogic.nl","stream":"outbound"}
  {"type":"smtp","host":"smtp.example.com","port":587,"user":"...","password":"...",
   "from":"ops@filogic.nl","ssl":false,"starttls":true}

Usage:
  python3 scripts/send_report.py --subject "Toll benchmark 2026-06-16" \
      [--to marinus@filogic.nl] [--html-file report.html]
  # HTML body may also be piped on stdin.
"""
import argparse
import json
import os
import subprocess
import sys

SECRET = os.environ.get("SMTP_SECRET", "benchmark-smtp")
DEFAULT_TO = "marinus@filogic.nl"


def load_cfg():
    raw = os.environ.get("BENCHMARK_SMTP_JSON")
    if not raw:
        r = subprocess.run(
            ["gcloud", "secrets", "versions", "access", "latest", "--secret=" + SECRET],
            capture_output=True, text=True, timeout=60,
        )
        if r.returncode != 0:
            sys.exit(f"cannot read secret {SECRET}: {r.stderr.strip()}")
        raw = r.stdout
    return json.loads(raw)


def send_postmark(cfg, to, subject, html):
    payload = {
        "From": cfg["from"], "To": to, "Subject": subject, "HtmlBody": html,
        "MessageStream": cfg.get("stream", "outbound"),
    }
    r = subprocess.run(
        ["curl", "-s", "-w", "\n%{http_code}", "-X", "POST", "https://api.postmarkapp.com/email",
         "-H", "Accept: application/json", "-H", "Content-Type: application/json",
         "-H", "X-Postmark-Server-Token: " + cfg["token"], "-d", json.dumps(payload)],
        capture_output=True, text=True, timeout=60,
    )
    code = r.stdout.strip().rsplit("\n", 1)[-1]
    if code != "200":
        sys.exit("postmark send failed: " + r.stdout[-400:])


def send_smtp(cfg, to, subject, html):
    import smtplib
    import ssl
    from email.mime.text import MIMEText

    msg = MIMEText(html, "html")
    msg["Subject"] = subject
    msg["From"] = cfg["from"]
    msg["To"] = to
    ctx = ssl.create_default_context()
    if cfg.get("ssl"):
        server = smtplib.SMTP_SSL(cfg["host"], int(cfg.get("port", 465)), context=ctx, timeout=60)
    else:
        server = smtplib.SMTP(cfg["host"], int(cfg.get("port", 587)), timeout=60)
        if cfg.get("starttls", True):
            server.starttls(context=ctx)
    if cfg.get("user"):
        server.login(cfg["user"], cfg["password"])
    server.sendmail(cfg["from"], [to], msg.as_string())
    server.quit()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--subject", required=True)
    ap.add_argument("--to", default=DEFAULT_TO)
    ap.add_argument("--html-file")
    args = ap.parse_args()

    html = open(args.html_file).read() if args.html_file else sys.stdin.read()
    cfg = load_cfg()
    if cfg.get("type", "smtp") == "postmark":
        send_postmark(cfg, args.to, args.subject, html)
    else:
        send_smtp(cfg, args.to, args.subject, html)
    print("sent to", args.to)


if __name__ == "__main__":
    main()
