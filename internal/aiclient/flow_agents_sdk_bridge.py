from __future__ import annotations

import asyncio
import json
import os
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any
from urllib.parse import urlparse


def _configure_stdio() -> None:
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure:
            try:
                reconfigure(encoding="utf-8", errors="replace")
            except Exception:
                pass


def _emit(payload: dict) -> int:
    sys.stdout.write(json.dumps(payload, ensure_ascii=False))
    sys.stdout.write("\n")
    sys.stdout.flush()
    return 0


async def _run(payload: dict) -> dict:
    try:
        from openai import AsyncOpenAI
        from agents import (
            Agent,
            OpenAIChatCompletionsModel,
            OpenAIResponsesModel,
            Runner,
            ShellCallOutcome,
            ShellCommandOutput,
            ShellCommandRequest,
            ShellResult,
            ShellTool,
            function_tool,
            set_tracing_disabled,
        )
    except Exception as exc:
        return {
            "ok": False,
            "error": "OpenAI Agents SDK 未安装或不可导入，请在 Ariadne agent runtime 中安装 openai-agents: "
            + f"{type(exc).__name__}: {exc}",
        }

    api_key = os.environ.get("OPENAI_API_KEY", "").strip()
    if not api_key:
        return {"ok": False, "error": "OPENAI_API_KEY 未设置"}

    base_url = str(payload.get("baseURL") or os.environ.get("OPENAI_BASE_URL") or "https://api.openai.com/v1").strip()
    model_name = str(payload.get("model") or "").strip()
    if not model_name:
        return {"ok": False, "error": "AI model 未配置"}

    system_prompt = str(payload.get("systemPrompt") or "").strip()
    user_prompt = str(payload.get("userPrompt") or "").strip()
    if not user_prompt:
        return {"ok": False, "error": "Flow agent prompt 为空"}
    skill = str(payload.get("skill") or "").strip()
    cli_command = str(payload.get("toolCommand") or "").strip()
    if not cli_command:
        cli_command = "ariadne"
    provider = str(payload.get("provider") or "openai-compatible").strip().lower()

    set_tracing_disabled(True)
    client = AsyncOpenAI(api_key=api_key, base_url=base_url)
    native_error = ""
    if _should_try_native_shell_skill(provider, base_url, payload):
        native = await _run_with_native_shell_skill(
            Agent=Agent,
            OpenAIResponsesModel=OpenAIResponsesModel,
            Runner=Runner,
            ShellCallOutcome=ShellCallOutcome,
            ShellCommandOutput=ShellCommandOutput,
            ShellResult=ShellResult,
            ShellTool=ShellTool,
            client=client,
            model_name=model_name,
            system_prompt=system_prompt,
            user_prompt=user_prompt,
            skill=skill,
            cli_command=cli_command,
        )
        if native.get("ok") or _truthy(os.environ.get("ARIADNE_FLOW_AGENT_NATIVE_SKILLS_STRICT")):
            return native
        native_error = str(native.get("error") or "").strip()

    model = OpenAIChatCompletionsModel(model=model_name, openai_client=client)

    async def _call_cli(action: str, args: list[str]) -> str:
        return await asyncio.to_thread(_run_workmemory_cli, cli_command, action, args)

    @function_tool
    async def search_flow_memory(
        query: str,
        limit: int = 8,
        since_hours: int = 24,
        source: str = "",
        app: str = "",
    ) -> str:
        """Search Ariadne local flow memory with semantic/keyword fallback and return JSON evidence."""
        args = ["--query", query, "--limit", str(_bounded_int(limit, 1, 20))]
        if since_hours > 0:
            args += ["--since-hours", str(_bounded_int(since_hours, 1, 24 * 30))]
        if source:
            args += ["--source", source]
        if app:
            args += ["--app", app]
        return await _call_cli("search", args)

    @function_tool
    async def recent_flow_memory(
        limit: int = 8,
        since_hours: int = 24,
        source: str = "",
        app: str = "",
    ) -> str:
        """Return recent non-sensitive Ariadne flow memories as JSON."""
        args = ["--limit", str(_bounded_int(limit, 1, 20))]
        if since_hours > 0:
            args += ["--since-hours", str(_bounded_int(since_hours, 1, 24 * 30))]
        if source:
            args += ["--source", source]
        if app:
            args += ["--app", app]
        return await _call_cli("recent", args)

    @function_tool
    async def get_flow_memory_entry(entry_id: str) -> str:
        """Load one Ariadne flow memory entry by id, including text/OCR/frame metadata."""
        return await _call_cli("get", ["--id", entry_id])

    instructions = system_prompt
    if skill:
        instructions = (
            instructions
            + "\n\nAriadne Flow Memory skill is available to you:\n"
            + skill
            + "\n\nUse the provided tools to execute this skill; do not answer factual memory questions from the fallback summary alone."
        )
    agent = Agent(
        name="Ariadne Flow",
        instructions=instructions,
        model=model,
        tools=[search_flow_memory, recent_flow_memory, get_flow_memory_entry],
    )
    result = await Runner.run(agent, input=user_prompt)
    answer = str(getattr(result, "final_output", "") or "").strip()
    if not answer:
        return {"ok": False, "error": "OpenAI Agents SDK 返回空内容"}
    if _looks_like_unexecuted_tool_call(answer):
        return {"ok": False, "error": "OpenAI Agents SDK function tool path 返回了未执行的 tool_call 文本"}
    message = "OpenAI Agents SDK 已通过 function tools 调用 Ariadne workmemory CLI（兼容接口降级）。"
    if native_error:
        message += " 原生 shell skill 未启用或不可用: " + native_error[:220]
    return {
        "ok": True,
        "answer": answer,
        "mode": "agent:openai-agents-sdk-function-tool-fallback",
        "message": message,
    }


def _bounded_int(value: Any, minimum: int, maximum: int) -> int:
    try:
        number = int(value)
    except Exception:
        number = minimum
    return max(minimum, min(maximum, number))


async def _run_with_native_shell_skill(
    *,
    Agent: Any,
    OpenAIResponsesModel: Any,
    Runner: Any,
    ShellCallOutcome: Any,
    ShellCommandOutput: Any,
    ShellResult: Any,
    ShellTool: Any,
    client: Any,
    model_name: str,
    system_prompt: str,
    user_prompt: str,
    skill: str,
    cli_command: str,
) -> dict:
    if not skill:
        return {"ok": False, "error": "Flow Memory skill 内容为空"}
    skill_dir = _write_skill_directory(skill)
    shell = _AriadneFlowMemoryShell(
        cli_command=cli_command,
        skill_dir=skill_dir,
        skill_content=skill,
        ShellCallOutcome=ShellCallOutcome,
        ShellCommandOutput=ShellCommandOutput,
        ShellResult=ShellResult,
    )
    shell_tool = ShellTool(
        executor=shell,
        needs_approval=False,
        environment={
            "type": "local",
            "skills": [
                {
                    "name": "ariadne-flow-memory",
                    "description": "Query Ariadne local flow memory, timeline, OCR, clipboard, window context, and evidence details.",
                    "path": str(skill_dir),
                }
            ],
        },
    )
    model = OpenAIResponsesModel(model=model_name, openai_client=client)
    instructions = (
        system_prompt
        + "\n\nUse the ariadne-flow-memory skill for factual memory questions. "
        + "Read its SKILL.md through the local shell when needed, then execute only the documented Ariadne workmemory commands."
    )
    agent = Agent(
        name="Ariadne Flow",
        instructions=instructions,
        model=model,
        tools=[shell_tool],
    )
    try:
        result = await Runner.run(agent, input=user_prompt)
    except Exception as exc:
        return {"ok": False, "error": f"{type(exc).__name__}: {exc}"}
    answer = str(getattr(result, "final_output", "") or "").strip()
    if not answer:
        return {"ok": False, "error": "OpenAI Agents SDK 原生 shell skill 返回空内容"}
    if _looks_like_unexecuted_tool_call(answer):
        return {"ok": False, "error": "OpenAI Agents SDK 原生 shell skill 返回了未执行的 tool_call 文本，当前兼容接口未真正执行 Responses 工具调用"}
    return {
        "ok": True,
        "answer": answer,
        "mode": "agent:openai-agents-sdk-shell-skill",
        "message": "OpenAI Agents SDK 已通过原生 local shell skill 调用 Ariadne Flow Memory。",
    }


class _AriadneFlowMemoryShell:
    def __init__(
        self,
        *,
        cli_command: str,
        skill_dir: Path,
        skill_content: str,
        ShellCallOutcome: Any,
        ShellCommandOutput: Any,
        ShellResult: Any,
    ) -> None:
        self.cli_command = cli_command
        self.skill_dir = skill_dir
        self.skill_content = skill_content
        self.ShellCallOutcome = ShellCallOutcome
        self.ShellCommandOutput = ShellCommandOutput
        self.ShellResult = ShellResult

    async def __call__(self, request: Any) -> Any:
        action = request.data.action
        outputs = []
        for command in action.commands:
            result = self._run_one(str(command), action.timeout_ms)
            outputs.append(
                self.ShellCommandOutput(
                    command=str(command),
                    stdout=result["stdout"],
                    stderr=result["stderr"],
                    outcome=self.ShellCallOutcome(
                        type="timeout" if result["timed_out"] else "exit",
                        exit_code=None if result["timed_out"] else result["exit_code"],
                    ),
                )
            )
        return self.ShellResult(output=outputs, max_output_length=action.max_output_length)

    def _run_one(self, command: str, timeout_ms: int | None) -> dict:
        if _is_skill_read_command(command, self.skill_dir):
            return {
                "stdout": self.skill_content if command.lower().find("skill.md") >= 0 else "SKILL.md\n",
                "stderr": "",
                "exit_code": 0,
                "timed_out": False,
            }
        parsed = _parse_workmemory_shell_command(command)
        if not parsed:
            return {
                "stdout": "",
                "stderr": "Rejected by Ariadne shell policy. Only reading the ariadne-flow-memory SKILL.md and running `ariadne workmemory ...` are allowed.",
                "exit_code": 2,
                "timed_out": False,
            }
        action, args = parsed
        output = _run_workmemory_cli(self.cli_command, action, args, timeout_ms=timeout_ms)
        return {"stdout": output, "stderr": "", "exit_code": 0, "timed_out": False}


def _write_skill_directory(skill: str) -> Path:
    root = Path(tempfile.gettempdir()) / "ariadne-agent-skills" / "ariadne-flow-memory"
    root.mkdir(parents=True, exist_ok=True)
    (root / "SKILL.md").write_text(skill, encoding="utf-8")
    return root


def _should_try_native_shell_skill(provider: str, base_url: str, payload: dict) -> bool:
    if _truthy(os.environ.get("ARIADNE_FLOW_AGENT_FORCE_FUNCTION_TOOLS")):
        return False
    if _truthy(payload.get("nativeSkills")) or _truthy(os.environ.get("ARIADNE_FLOW_AGENT_NATIVE_SKILLS")):
        return True
    parsed = urlparse(base_url)
    host = (parsed.netloc or parsed.path).lower()
    return provider == "openai" and ("api.openai.com" in host or "api.openai.azure.com" in host)


def _truthy(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    return str(value or "").strip().lower() in {"1", "true", "yes", "on"}


def _looks_like_unexecuted_tool_call(answer: str) -> bool:
    text = str(answer or "").strip().lower()
    if not text:
        return False
    if "<tool_call" in text or "</tool_call>" in text:
        return True
    if "<arg_key>" in text and "<arg_value>" in text:
        return True
    if ("\"tool_call\"" in text or "\"tool_calls\"" in text) and (
        "\"arguments\"" in text or "\"command\"" in text or "\"name\"" in text
    ):
        return True
    return False


def _is_skill_read_command(command: str, skill_dir: Path) -> bool:
    lowered = command.lower()
    skill_root = str(skill_dir).lower()
    if skill_root not in lowered:
        return False
    if "skill.md" in lowered:
        return any(token in lowered for token in ("type", "cat", "get-content", "gc ", "more"))
    return any(token in lowered for token in ("dir", "ls", "get-childitem", "gci"))


def _parse_workmemory_shell_command(command: str) -> tuple[str, list[str]] | None:
    if any(marker in command for marker in ("&&", "||", "|", ">", "<", "`", "\n", "\r")):
        return None
    try:
        tokens = [_clean_token(token) for token in shlex.split(command, posix=False)]
    except Exception:
        return None
    tokens = [token for token in tokens if token and token != "&"]
    lowered = [token.lower() for token in tokens]
    try:
        marker = lowered.index("workmemory")
    except ValueError:
        return None
    if marker + 1 >= len(tokens):
        return None
    action = lowered[marker + 1]
    if action not in {"status", "refresh", "search", "recent", "timeline", "get", "add-note"}:
        return None
    args = tokens[marker + 2 :]
    if any(arg.startswith(("/", "\\")) and not arg.startswith("--") for arg in args):
        return None
    return action, args


def _clean_token(token: str) -> str:
    token = token.strip()
    if len(token) >= 2 and token[0] == token[-1] and token[0] in {'"', "'"}:
        token = token[1:-1]
    return token


def _run_workmemory_cli(command: str, action: str, args: list[str], timeout_ms: int | None = None) -> str:
    base = [command, "workmemory", action]
    timeout = 45
    if timeout_ms and timeout_ms > 0:
        timeout = max(1, min(120, int(timeout_ms / 1000)))
    try:
        completed = subprocess.run(
            base + args,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=timeout,
        )
    except Exception as exc:
        return json.dumps(
            {"ok": False, "action": action, "message": f"Ariadne workmemory CLI 调用失败: {type(exc).__name__}: {exc}"},
            ensure_ascii=False,
        )
    output = (completed.stdout or "").strip()
    if not output:
        output = json.dumps(
            {
                "ok": False,
                "action": action,
                "message": "Ariadne workmemory CLI 没有输出",
                "stderr": (completed.stderr or "").strip()[:1200],
            },
            ensure_ascii=False,
        )
    if len(output) > 30000:
        output = output[:30000] + "\n... truncated ..."
    return output


def main() -> int:
    _configure_stdio()
    try:
        payload = json.loads(sys.stdin.read() or "{}")
    except Exception as exc:
        return _emit({"ok": False, "error": f"输入 JSON 解析失败: {exc}"})
    try:
        return _emit(asyncio.run(_run(payload)))
    except Exception as exc:
        return _emit({"ok": False, "error": f"{type(exc).__name__}: {exc}"})


if __name__ == "__main__":
    raise SystemExit(main())
