"""Chat notifier plugin for heCsTackForse bridge.

This notifier dispatches solve notifications to one of:
- Slack webhook
- Discord webhook
- Telegram bot chat

Configuration is read from event.data values populated by the Go backend.
"""

from __future__ import annotations

import json
from urllib import request


def _as_dict(value):
	if value is None:
		return {}
	if isinstance(value, dict):
		return value
	# Bridge maps to SimpleNamespace; vars() gives a dict for user-defined attrs.
	if hasattr(value, "__dict__"):
		return vars(value)
	return {}


def _as_bool(value):
	if isinstance(value, bool):
		return value
	if isinstance(value, str):
		return value.strip().lower() in {"1", "true", "yes", "on"}
	return bool(value)


def _as_int(value, default=0):
	try:
		return int(value)
	except (TypeError, ValueError):
		return default


def _post_json(url, payload):
	body = json.dumps(payload).encode("utf-8")
	req = request.Request(
		url,
		data=body,
		headers={"Content-Type": "application/json"},
		method="POST",
	)
	with request.urlopen(req, timeout=10) as resp:
		if resp.status < 200 or resp.status >= 300:
			raise RuntimeError(f"webhook returned HTTP {resp.status}")


class ChatNotifier:
	def name(self):
		return "chat_notifier"

	def Name(self):
		return self.name()

	def init(self, _cfg=None):
		return None

	def Init(self, _cfg=None):
		return self.init(_cfg)

	def notify(self, event):
		data = _as_dict(getattr(event, "data", None))
		event_type = getattr(event, "type", "")

		if not _as_bool(data.get("notifier_send_notifications", False)):
			return
		if event_type == "solve" and not _as_bool(data.get("notifier_send_solves", False)):
			return

		solve_num = _as_int(data.get("solve_num", 0), default=0)
		max_solve = _as_int(data.get("notifier_solve_count", 0), default=0)
		if event_type == "solve" and max_solve > 0 and solve_num > max_solve:
			return

		message = str(data.get("message") or "")
		if not message:
			solver = str(data.get("solver") or "unknown")
			challenge = str(data.get("challenge") or "unknown")
			message = f"{solver} solved {challenge} ({solve_num} solve)"

		notifier_type = str(data.get("notifier_type") or "slack").strip().lower()
		if notifier_type == "slack":
			url = str(data.get("notifier_slack_webhook_url") or "").strip()
			if not url:
				return
			_post_json(url, {"text": message})
			return

		if notifier_type == "discord":
			url = str(data.get("notifier_discord_webhook_url") or "").strip()
			if not url:
				return
			_post_json(url, {"content": message})
			return

		if notifier_type == "telegram":
			token = str(data.get("notifier_telegram_bot_token") or "").strip()
			chat_id = str(data.get("notifier_telegram_chat_id") or "").strip()
			if not token or not chat_id:
				return
			url = f"https://api.telegram.org/bot{token}/sendMessage"
			_post_json(url, {"chat_id": chat_id, "text": message})
			return

	def Notify(self, event):
		return self.notify(event)


def register(registry):
	registry.register_notifier(ChatNotifier())
