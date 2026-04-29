#!/usr/bin/env python3
"""Lightweight bridge for Python-native challenge plugins.

The bridge loads plugin modules from configured paths and exposes them to the Go
server over a small JSON protocol on stdin/stdout.
"""

from __future__ import annotations

import argparse
import importlib
import importlib.util
import inspect
import json
import pathlib
import sys
import traceback
import types
from types import SimpleNamespace
from typing import Any


def flag_checker(name: str | None = None):
    def decorator(obj):
        obj.__ctfd_kind__ = "flag_checker"
        obj.__ctfd_name__ = name
        return obj

    return decorator


def scorer(name: str | None = None):
    def decorator(obj):
        obj.__ctfd_kind__ = "scorer"
        obj.__ctfd_name__ = name
        return obj

    return decorator


def notifier(name: str | None = None):
    def decorator(obj):
        obj.__ctfd_kind__ = "notifier"
        obj.__ctfd_name__ = name
        return obj

    return decorator


def challenge_type(name: str | None = None):
    def decorator(obj):
        obj.__ctfd_kind__ = "challenge_type"
        obj.__ctfd_name__ = name
        return obj

    return decorator


class Registry:
    def __init__(self) -> None:
        self.flag_checkers: dict[str, Any] = {}
        self.scorers: dict[str, Any] = {}
        self.notifiers: dict[str, Any] = {}
        self.challenge_types: dict[str, Any] = {}
        self.assets: list[dict[str, Any]] = []
        self.admin_menu: list[dict[str, Any]] = []
        self.user_menu: list[dict[str, Any]] = []
        self.home_widgets: list[dict[str, Any]] = []

    def register_flag_checker(self, plugin: Any) -> None:
        self.flag_checkers[self._name_for(plugin)] = self._init_plugin(plugin)

    def register_scorer(self, plugin: Any) -> None:
        self.scorers[self._name_for(plugin)] = self._init_plugin(plugin)

    def register_notifier(self, plugin: Any) -> None:
        self.notifiers[self._name_for(plugin)] = self._init_plugin(plugin)

    def register_challenge_type(self, plugin: Any) -> None:
        self.challenge_types[self._name_for(plugin)] = self._init_plugin(plugin)

    def _name_for(self, plugin: Any) -> str:
        for attr in ("name", "Name", "type_id", "TypeID"):
            value = getattr(plugin, attr, None)
            if callable(value):
                try:
                    resolved = value()
                    if resolved:
                        return str(resolved)
                except Exception:
                    pass
        fallback = getattr(plugin, "__ctfd_name__", None)
        if fallback:
            return str(fallback)
        return plugin.__class__.__name__.lower()

    def _init_plugin(self, plugin: Any) -> Any:
        init_fn = None
        for attr in ("init", "Init"):
            value = getattr(plugin, attr, None)
            if callable(value):
                init_fn = value
                break
        if init_fn is not None:
            try:
                init_fn({})
            except TypeError:
                init_fn()
        return plugin


class BridgeApp:
    """Small CTFd-compatible app shim passed to plugin load(app) functions."""

    def __init__(self, registry: Registry) -> None:
        self.registry = registry
        self.config: dict[str, Any] = {}
        self.routes: list[dict[str, Any]] = []

    def route(self, rule: str, **options):
        def decorator(func):
            self.routes.append({"rule": rule, "endpoint": getattr(func, "__name__", rule), "options": options})
            return func

        return decorator

    def register_flag_checker(self, plugin: Any) -> None:
        self.registry.register_flag_checker(plugin)

    def register_scorer(self, plugin: Any) -> None:
        self.registry.register_scorer(plugin)

    def register_notifier(self, plugin: Any) -> None:
        self.registry.register_notifier(plugin)

    def register_challenge_type(self, plugin: Any) -> None:
        self.registry.register_challenge_type(plugin)

    def register_home_widget(self, name: str, title: str, body: str = "", url: str = "") -> None:
        self.registry.home_widgets.append({"name": name, "title": title, "body": body, "url": url})

    def register_admin_menu(self, name: str, title: str, route: str) -> None:
        self.registry.admin_menu.append({"name": name, "title": title, "route": route})


def install_ctfd_shims(registry: Registry) -> None:
    """Install lightweight modules for common CTFd plugin imports.

    The bridge does not run Flask. These shims let simple plugins load and
    expose metadata/assets while platform integrations register explicit bridge
    adapters for runtime behaviour.
    """

    ctfd = sys.modules.setdefault("CTFd", types.ModuleType("CTFd"))

    plugins_mod = types.ModuleType("CTFd.plugins")

    def register_plugin_assets_directory(_app=None, base_path="", endpoint=None, **_kwargs):
        registry.assets.append({"type": "directory", "base_path": base_path, "endpoint": endpoint})

    def register_plugin_asset(_app=None, asset_path="", **_kwargs):
        registry.assets.append({"type": "file", "asset_path": asset_path})

    def register_admin_plugin_menu_bar(title, route):
        key = str(title).strip().lower().replace(" ", "_")
        registry.admin_menu.append({"name": key, "title": title, "route": route})

    def register_user_page_menu_bar(title, route):
        registry.user_menu.append({"title": title, "route": route})

    plugins_mod.register_plugin_assets_directory = register_plugin_assets_directory
    plugins_mod.register_plugin_asset = register_plugin_asset
    plugins_mod.register_admin_plugin_menu_bar = register_admin_plugin_menu_bar
    plugins_mod.register_user_page_menu_bar = register_user_page_menu_bar
    sys.modules["CTFd.plugins"] = plugins_mod
    setattr(ctfd, "plugins", plugins_mod)

    utils_mod = types.ModuleType("CTFd.utils")
    config_store: dict[str, Any] = {}
    utils_mod.get_config = lambda key, default=None: config_store.get(key, default)
    utils_mod.set_config = lambda key, value: config_store.__setitem__(key, value)
    sys.modules["CTFd.utils"] = utils_mod
    setattr(ctfd, "utils", utils_mod)

    decorators_mod = types.ModuleType("CTFd.utils.decorators")
    decorators_mod.admins_only = lambda f: f
    decorators_mod.authed_only = lambda f: f
    decorators_mod.during_ctf_time_only = lambda f: f
    decorators_mod.require_verified_emails = lambda f: f
    decorators_mod.bypass_csrf_protection = lambda f: f
    sys.modules["CTFd.utils.decorators"] = decorators_mod

    models_mod = types.ModuleType("CTFd.models")
    models_mod.db = SimpleNamespace(session=SimpleNamespace(add=lambda *_: None, commit=lambda: None))
    sys.modules["CTFd.models"] = models_mod
    setattr(ctfd, "models", models_mod)


def _load_module(path: pathlib.Path):
    module_name = f"ctfd_bridge_{path.stem}_{abs(hash(str(path)))}"
    spec = importlib.util.spec_from_file_location(module_name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load module from {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = module
    spec.loader.exec_module(module)
    return module


def _load_registry_from_module(module, registry: Registry) -> None:
    if hasattr(module, "load") and callable(module.load):
        module.load(BridgeApp(registry))
        # Continue scanning decorators too; hybrid plugins are cheap to support.

    if hasattr(module, "register") and callable(module.register):
        module.register(registry)
        return

    for _, obj in inspect.getmembers(module):
        kind = getattr(obj, "__ctfd_kind__", None)
        if kind is None:
            continue
        instance = obj() if inspect.isclass(obj) else obj
        if kind == "flag_checker":
            registry.register_flag_checker(instance)
        elif kind == "scorer":
            registry.register_scorer(instance)
        elif kind == "notifier":
            registry.register_notifier(instance)
        elif kind == "challenge_type":
            registry.register_challenge_type(instance)


def load_plugins(plugin_dirs: list[str], modules: list[str]) -> Registry:
    registry = Registry()
    install_ctfd_shims(registry)
    for entry in modules:
        module_path = pathlib.Path(entry)
        if module_path.exists():
            _load_registry_from_module(_load_module(module_path), registry)
        else:
            module = importlib.import_module(entry)
            _load_registry_from_module(module, registry)
    for plugin_dir in plugin_dirs:
        root = pathlib.Path(plugin_dir)
        if not root.exists():
            continue
        for path in sorted(root.rglob("*.py")):
            if path.name.startswith("_") or path.name == "ctfd_bridge.py":
                continue
            _load_registry_from_module(_load_module(path), registry)
    return registry


def _as_namespace(value: Any) -> Any:
    if isinstance(value, dict):
        return SimpleNamespace(**{k: _as_namespace(v) for k, v in value.items()})
    if isinstance(value, list):
        return [_as_namespace(item) for item in value]
    return value


def serve(registry: Registry) -> None:
    for raw_line in sys.stdin:
        line = raw_line.strip()
        if not line:
            continue
        try:
            request = json.loads(line)
            op = request.get("op")
            params = request.get("params") or {}
            if op == "list":
                result = {
                    "flag_checkers": sorted(registry.flag_checkers.keys()),
                    "scorers": sorted(registry.scorers.keys()),
                    "notifiers": sorted(registry.notifiers.keys()),
                    "challenge_types": sorted(registry.challenge_types.keys()),
                    "assets": registry.assets,
                    "admin_menu": registry.admin_menu,
                    "user_menu": registry.user_menu,
                    "home_widgets": registry.home_widgets,
                }
            elif op == "check_flag":
                plugin = registry.flag_checkers[params["name"]]
                result = bool(plugin.check(params["correct_flag"], params["submitted"]))
            elif op == "calculate_score":
                plugin = registry.scorers[params["name"]]
                challenge = _as_namespace(params.get("challenge") or {})
                result = int(plugin.calculate_score(challenge, int(params.get("solve_count", 0))))
            elif op == "notify":
                plugin = registry.notifiers[params["name"]]
                plugin.notify(_as_namespace(params.get("event") or {}))
                result = True
            elif op == "verify_challenge_type":
                plugin = registry.challenge_types[params["name"]]
                result = bool(plugin.verify(params.get("challenge_data", ""), params.get("submitted", "")))
            elif op == "shutdown":
                sys.stdout.write(json.dumps({"ok": True, "result": True}) + "\n")
                sys.stdout.flush()
                break
            else:
                raise RuntimeError(f"unknown op: {op}")
            sys.stdout.write(json.dumps({"ok": True, "result": result}) + "\n")
            sys.stdout.flush()
        except Exception as exc:
            sys.stdout.write(json.dumps({"ok": False, "error": f"{exc}"}) + "\n")
            sys.stdout.flush()
            traceback.print_exc(file=sys.stderr)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--plugin-dir", action="append", default=[])
    parser.add_argument("--module", action="append", default=[])
    args = parser.parse_args()

    registry = load_plugins(args.plugin_dir, args.module)
    serve(registry)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
