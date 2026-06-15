import argparse
import contextlib
import importlib.util
import inspect
import json
import os
import re
import sys


def emit(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False, default=str))


def listify(value):
    if value is None:
        return []
    if isinstance(value, (list, tuple, set)):
        return [str(item) for item in value]
    return [str(value)]


def safe_call(instance, name, default=None):
    method = getattr(instance, name, None)
    if not callable(method):
        return default
    try:
        return method()
    except Exception as exc:
        return default if default is not None else str(exc)


def plugin_id(instance, module_name):
    keywords = listify(safe_call(instance, "get_keywords", []))
    if keywords:
        return normalize_id(keywords[0])
    return normalize_id(module_name)


def normalize_id(value):
    value = re.sub(r"[^0-9a-zA-Z_]+", "_", str(value)).strip("_").lower()
    return value or "legacy_plugin"


def load_plugins(workspace_root, plugin_dir):
    if workspace_root not in sys.path:
        sys.path.insert(0, workspace_root)
    try:
        from src.core.plugin_base import PluginBase
    except Exception as exc:
        return [], [{"module": "plugin_base", "error": str(exc)}]

    loaded = []
    errors = []
    if not os.path.isdir(plugin_dir):
        return [], [{"module": plugin_dir, "error": "plugin directory not found"}]

    for filename in sorted(os.listdir(plugin_dir)):
        if not filename.endswith(".py") or filename.startswith("__"):
            continue
        module_name = os.path.splitext(filename)[0]
        module_path = os.path.join(plugin_dir, filename)
        spec_name = "_ariadne_legacy_" + normalize_id(module_name)
        try:
            spec = importlib.util.spec_from_file_location(spec_name, module_path)
            if spec is None or spec.loader is None:
                raise RuntimeError("cannot create import spec")
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
        except Exception as exc:
            errors.append({"module": module_name, "error": str(exc)})
            continue

        for _, cls in inspect.getmembers(module, inspect.isclass):
            try:
                if cls is PluginBase or not issubclass(cls, PluginBase):
                    continue
                loaded.append((module_name, cls()))
            except Exception as exc:
                errors.append({"module": module_name, "error": str(exc)})
    return loaded, errors


def manifest_for(module_name, instance):
    name = str(safe_call(instance, "get_name", instance.__class__.__name__))
    keywords = listify(safe_call(instance, "get_keywords", []))
    command_schema = safe_call(instance, "get_command_schema", {})
    if not isinstance(command_schema, dict):
        command_schema = {}
    return {
        "id": plugin_id(instance, module_name),
        "name": name,
        "description": str(safe_call(instance, "get_description", "")),
        "keywords": keywords,
        "supportedPlatforms": listify(safe_call(instance, "get_supported_platforms", ["windows"])),
        "requiredCapabilities": listify(safe_call(instance, "get_required_capabilities", [])),
        "commandSchema": command_schema,
        "directAction": bool(safe_call(instance, "is_direct_action", False)),
    }


def normalize_results(results):
    if results is None:
        return []
    if isinstance(results, dict):
        results = [results]
    if not isinstance(results, list):
        results = [{"name": "Python legacy result", "path": str(results), "type": "legacy_result"}]
    normalized = []
    for item in results:
        if isinstance(item, dict):
            normalized.append(item)
        else:
            normalized.append({"name": "Python legacy result", "path": str(item), "type": "legacy_result"})
    return normalized


def command_list(args):
    with contextlib.redirect_stdout(sys.stderr):
        plugins, errors = load_plugins(args.workspace_root, args.plugin_dir)
        manifests = [manifest_for(module_name, instance) for module_name, instance in plugins]
    for error in errors:
        manifests.append({
            "id": "load_error_" + normalize_id(error.get("module", "unknown")),
            "name": "旧插件加载失败",
            "description": error.get("error", ""),
            "keywords": [],
            "supportedPlatforms": ["windows"],
            "requiredCapabilities": ["python_legacy_bridge"],
            "loadError": error.get("error", ""),
        })
    emit({"ok": True, "manifests": manifests})


def command_execute(args):
    with contextlib.redirect_stdout(sys.stderr):
        plugins, _ = load_plugins(args.workspace_root, args.plugin_dir)
    requested = args.keyword.strip().lower()
    for module_name, instance in plugins:
        with contextlib.redirect_stdout(sys.stderr):
            manifest = manifest_for(module_name, instance)
        names = [manifest["id"], module_name] + [item.lower() for item in manifest.get("keywords", [])]
        if requested not in names:
            continue
        try:
            with contextlib.redirect_stdout(sys.stderr):
                results = instance.execute(args.query)
        except Exception as exc:
            emit({"ok": True, "results": [{"name": "旧插件执行失败", "path": str(exc), "type": "legacy_error"}]})
            return
        emit({"ok": True, "results": normalize_results(results)})
        return
    emit({"ok": False, "error": "legacy plugin not found: " + args.keyword})


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--workspace-root", required=True)
    parser.add_argument("--plugin-dir", required=True)
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("list")
    execute_parser = subparsers.add_parser("execute")
    execute_parser.add_argument("--keyword", required=True)
    execute_parser.add_argument("--query", default="")
    args = parser.parse_args()

    if args.command == "list":
        command_list(args)
    elif args.command == "execute":
        command_execute(args)


if __name__ == "__main__":
    main()
