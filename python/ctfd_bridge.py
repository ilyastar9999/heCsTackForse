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
