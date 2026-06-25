from __future__ import annotations

import asyncio
import html
import json
import os
import re
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
    if provider != "openai":
        _install_chat_tool_argument_normalizer(client)
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

    if provider != "openai":
        return await _run_with_compatible_chat_tools(
            client=client,
            model_name=model_name,
            system_prompt=system_prompt,
            user_prompt=user_prompt,
            skill=skill,
            cli_command=cli_command,
            native_error=native_error,
        )

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

    @function_tool
    async def list_flow_todos(
        status: str = "",
        query: str = "",
        scope: str = "",
        include_done: bool = False,
        limit: int = 20,
    ) -> str:
        """List Ariadne todos for unfinished work, follow-ups, reminders, and pending actions."""
        args = ["--limit", str(_bounded_int(limit, 1, 50))]
        if status:
            args += ["--status", status]
        if query:
            args += ["--query", query]
        if scope:
            args += ["--scope", scope]
        if include_done:
            args += ["--include-done"]
        return await _call_cli("todos", args)

    @function_tool
    async def add_flow_todo(
        title: str,
        note: str = "",
        priority: str = "normal",
        status: str = "open",
        scope: str = "",
        evidence: str = "",
    ) -> str:
        """Add one Ariadne todo when the task and owner are clear."""
        args = ["--title", title, "--priority", priority, "--status", status]
        if note:
            args += ["--text", note]
        if scope:
            args += ["--scope", scope]
        if evidence:
            args += ["--evidence", evidence]
        return await _call_cli("todo-add", args)

    @function_tool
    async def update_flow_todo(
        todo_id: str,
        status: str = "",
        title: str = "",
        note: str = "",
        priority: str = "",
        scope: str = "",
    ) -> str:
        """Update one Ariadne todo status or metadata by id."""
        args = ["--id", todo_id]
        if status:
            args += ["--status", status]
        if title:
            args += ["--title", title]
        if note:
            args += ["--text", note]
        if priority:
            args += ["--priority", priority]
        if scope:
            args += ["--scope", scope]
        return await _call_cli("todo-update", args)

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
        tools=[search_flow_memory, recent_flow_memory, get_flow_memory_entry, list_flow_todos, add_flow_todo, update_flow_todo],
    )
    result = await Runner.run(agent, input=user_prompt)
    answer = str(getattr(result, "final_output", "") or "").strip()
    if not answer:
        return {"ok": False, "error": "OpenAI Agents SDK 返回空内容"}
    if _looks_like_unexecuted_tool_call(answer):
        return {"ok": False, "error": "OpenAI Agents SDK function tool path 返回了未执行的 tool_call 文本"}
    message = "OpenAI Agents SDK 已通过 Chat Completions function tools 调用 Ariadne workmemory CLI。"
    if native_error:
        message += " 原生 Responses Skill 不可用: " + native_error[:220]
    return {
        "ok": True,
        "answer": answer,
        "mode": "agent:openai-agents-sdk-chat-tools",
        "message": message,
    }


def _bounded_int(value: Any, minimum: int, maximum: int) -> int:
    try:
        number = int(value)
    except Exception:
        number = minimum
    return max(minimum, min(maximum, number))


async def _run_with_compatible_chat_tools(
    *,
    client: Any,
    model_name: str,
    system_prompt: str,
    user_prompt: str,
    skill: str,
    cli_command: str,
    native_error: str = "",
) -> dict:
    instructions = system_prompt
    if skill:
        instructions = (
            instructions
            + "\n\nAriadne Flow Memory skill is available to you:\n"
            + skill
            + "\n\nUse the provided tools to execute this skill; do not answer factual memory questions from the fallback summary alone."
        )
    messages: list[dict[str, Any]] = [
        {"role": "system", "content": instructions},
        {"role": "user", "content": user_prompt},
    ]
    tools = _compatible_chat_tool_schemas()
    todo_requirement = _todo_tool_requirement(user_prompt)
    used_tools = False
    used_tool_names: list[str] = []
    todo_mutation_succeeded = False
    preloaded_todo = _preload_readonly_todo_tool(cli_command, user_prompt, todo_requirement)
    if preloaded_todo:
        used_tools = True
        used_tool_names.append("list_flow_todos")
        messages.append(
            {
                "role": "user",
                "content": "Ariadne Todo 工具已查询结果：\n"
                + preloaded_todo[:18000]
                + "\n\n请基于这个待办结果回答；如需补充上下文，可继续自动调用其他 Ariadne memory 工具。",
            }
        )
    for _ in range(6):
        tool_choice: Any = "auto"
        if not used_tools and todo_requirement.get("required_tool"):
            tool_choice = _chat_tool_choice(str(todo_requirement["required_tool"]))
        try:
            response = await _create_chat_completion_with_retries(
                client,
                model=model_name,
                messages=messages,
                tools=tools,
                tool_choice=tool_choice,
                temperature=0.2,
                max_tokens=1800,
            )
        except Exception as exc:
            if _should_retry_chat_completion_error(exc):
                fallback = await _answer_from_cli_retrieval_fallback(
                    client=client,
                    model_name=model_name,
                    user_prompt=user_prompt,
                    cli_command=cli_command,
                    reason=f"{type(exc).__name__}: {exc}",
                )
                if fallback.get("ok"):
                    return fallback
            return {"ok": False, "error": f"OpenAI-compatible Chat Tools 调用失败: {type(exc).__name__}: {exc}"}
        choices = getattr(response, "choices", None) or []
        if not choices:
            return {"ok": False, "error": "OpenAI-compatible Chat Tools 未返回 choices"}
        message = getattr(choices[0], "message", None)
        if message is None:
            return {"ok": False, "error": "OpenAI-compatible Chat Tools 未返回 message"}
        tool_calls = list(getattr(message, "tool_calls", None) or [])
        if tool_calls:
            used_tools = True
            messages.append(_assistant_message_from_tool_calls(message, tool_calls))
            for tool_call in tool_calls:
                tool_name = str(getattr(getattr(tool_call, "function", None), "name", "") or "").strip()
                tool_args = _tool_arguments_dict(getattr(getattr(tool_call, "function", None), "arguments", None))
                tool_output = await asyncio.to_thread(_run_compatible_tool, cli_command, user_prompt, tool_name, tool_args)
                used_tool_names.append(tool_name)
                if tool_name in {"add_flow_todo", "update_flow_todo"} and _tool_output_ok(tool_output):
                    todo_mutation_succeeded = True
                messages.append(
                    {
                        "role": "tool",
                        "tool_call_id": str(getattr(tool_call, "id", "") or ""),
                        "name": tool_name,
                        "content": tool_output[:18000],
                    }
                )
            continue
        answer = str(getattr(message, "content", "") or "").strip()
        if not answer:
            if used_tools:
                answer, retry_detail = await _retry_compatible_chat_final_answer(
                    client=client,
                    model_name=model_name,
                    messages=messages,
                )
                if answer:
                    missing_required_tool = _missing_required_todo_tool(
                        todo_requirement, used_tool_names, todo_mutation_succeeded
                    )
                    if missing_required_tool:
                        return {"ok": False, "error": missing_required_tool}
                    if _looks_like_unexecuted_tool_call(answer):
                        return {"ok": False, "error": "OpenAI-compatible Chat Tools 返回了未执行的 tool_call 文本"}
                    message_text = "OpenAI-compatible Chat Completions 已通过 Ariadne 兼容工具调用本地 workmemory CLI。"
                    if native_error:
                        message_text += " 原生 Responses Skill 不可用: " + native_error[:220]
                    message_text += " 已基于工具结果生成最终回答。"
                    return {
                        "ok": True,
                        "answer": answer,
                        "mode": "agent:openai-compatible-chat-tools",
                        "message": message_text,
                    }
                detail = f"；{retry_detail}" if retry_detail else ""
                return {"ok": False, "error": "OpenAI-compatible Chat Tools 返回空内容" + detail}
            return {"ok": False, "error": "OpenAI-compatible Chat Tools 返回空内容"}
        missing_required_tool = _missing_required_todo_tool(todo_requirement, used_tool_names, todo_mutation_succeeded)
        if missing_required_tool:
            return {"ok": False, "error": missing_required_tool}
        if _looks_like_unexecuted_tool_call(answer):
            return {"ok": False, "error": "OpenAI-compatible Chat Tools 返回了未执行的 tool_call 文本"}
        message_text = "OpenAI-compatible Chat Completions 已通过 Ariadne 兼容工具调用本地 workmemory CLI。"
        if native_error:
            message_text += " 原生 Responses Skill 不可用: " + native_error[:220]
        if not used_tools:
            message_text += " 本轮模型未请求工具。"
        return {
            "ok": True,
            "answer": answer,
            "mode": "agent:openai-compatible-chat-tools",
            "message": message_text,
        }
    return {"ok": False, "error": "OpenAI-compatible Chat Tools 超过最大工具调用轮次"}


async def _retry_compatible_chat_final_answer(
    *,
    client: Any,
    model_name: str,
    messages: list[dict[str, Any]],
) -> tuple[str, str]:
    details: list[str] = []
    try:
        response = await _create_chat_completion_with_retries(
            client,
            model=model_name,
            messages=messages
            + [
                {
                    "role": "user",
                    "content": "请只根据上面的工具结果给出最终中文回答。不要再调用工具，不要输出工具调用文本。",
                }
            ],
            temperature=0.2,
            max_tokens=1800,
        )
        answer = _chat_completion_answer(response)
        if answer:
            return answer, ""
        details.append("原消息链最终回答仍为空")
    except Exception as exc:
        details.append(f"原消息链最终回答失败: {type(exc).__name__}: {exc}")

    try:
        response = await _create_chat_completion_with_retries(
            client,
            model=model_name,
            messages=[
                {
                    "role": "system",
                    "content": "你是 Ariadne 心流 Agent。只根据用户问题和工具结果生成中文最终回答；不要输出工具调用文本。",
                },
                {
                    "role": "user",
                    "content": _plain_final_answer_prompt(messages),
                },
            ],
            temperature=0.2,
            max_tokens=1800,
        )
        answer = _chat_completion_answer(response)
        if answer:
            return answer, ""
        details.append("纯文本工具结果最终回答仍为空")
    except Exception as exc:
        details.append(f"纯文本工具结果最终回答失败: {type(exc).__name__}: {exc}")
    return "", "；".join(details)


async def _create_chat_completion_with_retries(client: Any, attempts: int = 3, **kwargs: Any) -> Any:
    last_exc: Exception | None = None
    for attempt in range(max(1, attempts)):
        try:
            return await client.chat.completions.create(**kwargs)
        except Exception as exc:
            last_exc = exc
            if attempt >= attempts - 1 or not _should_retry_chat_completion_error(exc):
                raise
            await asyncio.sleep(min(2.5, 0.8 * (attempt + 1)))
    if last_exc:
        raise last_exc
    raise RuntimeError("chat completion retry exhausted")


def _should_retry_chat_completion_error(exc: Exception) -> bool:
    status = getattr(exc, "status_code", None)
    if isinstance(status, int):
        if status in {408, 409, 425, 429} or status >= 500:
            return True
        if status == 400 and "upstream error" in str(exc).lower():
            return True
        return False
    text = str(exc).lower()
    return any(
        marker in text
        for marker in (
            "upstream error",
            "timeout",
            "timed out",
            "temporarily",
            "connection reset",
            "connection aborted",
            "bad gateway",
            "service unavailable",
        )
    )


def _chat_completion_answer(response: Any) -> str:
    choices = getattr(response, "choices", None) or []
    if not choices:
        return ""
    message = getattr(choices[0], "message", None)
    if message is None:
        return ""
    return str(getattr(message, "content", "") or "").strip()


def _plain_final_answer_prompt(messages: list[dict[str, Any]]) -> str:
    user_messages = [str(message.get("content") or "").strip() for message in messages if message.get("role") == "user"]
    original_user = user_messages[0] if user_messages else ""
    additional_user_context = "\n\n".join(message for message in user_messages[1:] if message)
    tool_blocks = []
    for message in messages:
        if message.get("role") != "tool":
            continue
        name = str(message.get("name") or "tool").strip()
        content = str(message.get("content") or "").strip()
        if not content:
            continue
        if len(content) > 12000:
            content = content[:12000] + "\n... truncated ..."
        tool_blocks.append(f"### {name}\n{content}")
    tool_text = "\n\n".join(tool_blocks)
    if not tool_text:
        tool_text = "无工具结果。"
    return (
        "用户问题和回答要求：\n"
        + original_user[:12000]
        + ("\n\n附加上下文：\n" + additional_user_context[:12000] if additional_user_context else "")
        + "\n\n已执行的 Ariadne 工具结果：\n"
        + tool_text[:24000]
        + "\n\n请直接给出最终中文回答。不要再调用工具；不要输出 JSON、XML 或 tool_call。"
    )


def _preload_readonly_todo_tool(cli_command: str, user_prompt: str, requirement: dict[str, Any]) -> str:
    if str(requirement.get("required_tool") or "").strip() != "list_flow_todos":
        return ""
    if bool(requirement.get("mutating")):
        return ""
    return _run_compatible_tool(
        cli_command,
        user_prompt,
        "list_flow_todos",
        {
            "status": "open",
            "limit": 20,
        },
    )


async def _answer_from_cli_retrieval_fallback(
    *,
    client: Any,
    model_name: str,
    user_prompt: str,
    cli_command: str,
    reason: str,
) -> dict:
    question = _extract_flow_user_question(user_prompt)
    if not question:
        question = "用户的问题"
    outputs: list[tuple[str, str]] = []
    queries = [question]
    compact = re.sub(r"\s+", "", question)
    if _todo_tool_requirement(user_prompt).get("required_tool") == "list_flow_todos":
        outputs.append(("todos", _run_workmemory_cli(cli_command, "todos", ["--status", "open", "--limit", "20"])))
    if any(term in compact for term in ("谁找", "找过我", "跟谁聊", "联系人", "消息")):
        queries.append("微信 钉钉 找我 联系人 消息")
    seen_queries = set()
    for query in queries:
        query = query.strip()
        if not query or query in seen_queries:
            continue
        seen_queries.add(query)
        outputs.append(
            (
                "search",
                _run_workmemory_cli(cli_command, "search", ["--query", query, "--limit", "8", "--since-hours", "24"]),
            )
        )
    outputs.append(("recent", _run_workmemory_cli(cli_command, "recent", ["--limit", "8", "--since-hours", "24"])))
    memory_ids = _memory_ids_from_tool_outputs([content for _, content in outputs])[:5]
    for memory_id in memory_ids:
        outputs.append(("get", _run_workmemory_cli(cli_command, "get", ["--id", memory_id])))

    try:
        response = await _create_chat_completion_with_retries(
            client,
            model=model_name,
            messages=[
                {
                    "role": "system",
                    "content": "你是 Ariadne 心流 Agent。只根据 Ariadne 本地检索结果回答用户问题；不要编造证据。",
                },
                {
                    "role": "user",
                    "content": _retrieval_fallback_prompt(question, outputs, reason),
                },
            ],
            temperature=0.2,
            max_tokens=1800,
        )
        answer = _chat_completion_answer(response)
        if answer and not _looks_like_unexecuted_tool_call(answer):
            return {
                "ok": True,
                "answer": answer,
                "mode": "agent:openai-compatible-chat-tools",
                "message": "OpenAI-compatible Chat Completions 已基于 Ariadne 本地检索结果生成回答。",
            }
    except Exception:
        pass

    answer = _local_retrieval_fallback_answer(question, outputs)
    if answer:
        return {
            "ok": True,
            "answer": answer,
            "mode": "agent:openai-compatible-chat-tools",
            "message": "Ariadne 已基于本地检索结果生成回答。",
        }
    return {"ok": False, "error": "本地检索兜底未获得可用结果"}


def _retrieval_fallback_prompt(question: str, outputs: list[tuple[str, str]], reason: str) -> str:
    blocks = []
    for label, content in outputs:
        content = str(content or "").strip()
        if not content:
            continue
        if len(content) > 10000:
            content = content[:10000] + "\n... truncated ..."
        blocks.append(f"### {label}\n{content}")
    return (
        "用户问题：\n"
        + question
        + "\n\nAriadne 工具调用模式暂时不可用：\n"
        + reason[:500]
        + "\n\n本地检索结果：\n"
        + ("\n\n".join(blocks) or "无本地检索结果。")
        + "\n\n请根据本地检索结果直接回答。联系人/聊天问题要区分聊天正文、左侧列表和背景窗口；结尾列出最多 6 个 memory id。"
    )


def _memory_ids_from_tool_outputs(outputs: list[str]) -> list[str]:
    ids: list[str] = []
    seen = set()

    def visit(value: Any) -> None:
        if isinstance(value, dict):
            item_id = str(value.get("id") or "").strip()
            if item_id.startswith("memory-") and item_id not in seen:
                seen.add(item_id)
                ids.append(item_id)
            for child in value.values():
                visit(child)
        elif isinstance(value, list):
            for child in value:
                visit(child)

    for output in outputs:
        try:
            parsed = json.loads(str(output or "{}"))
        except Exception:
            continue
        visit(parsed)
    return ids


def _local_retrieval_fallback_answer(question: str, outputs: list[tuple[str, str]]) -> str:
    todo_answer = _local_todo_fallback_answer(question, outputs)
    if todo_answer:
        return todo_answer
    rows = []
    seen = set()
    for _, content in outputs:
        try:
            parsed = json.loads(str(content or "{}"))
        except Exception:
            continue
        candidates = []
        if isinstance(parsed, dict):
            for key in ("results", "entries", "memories"):
                value = parsed.get(key)
                if isinstance(value, list):
                    candidates.extend(value)
            if parsed.get("id"):
                candidates.append(parsed)
        for item in candidates:
            if not isinstance(item, dict):
                continue
            memory_id = str(item.get("id") or "").strip()
            if not memory_id or memory_id in seen:
                continue
            seen.add(memory_id)
            title = str(item.get("title") or item.get("windowTitle") or "本地记录").strip()
            summary = str(item.get("summary") or item.get("preview") or item.get("text") or "").strip()
            app_name = str(item.get("appName") or "").strip()
            if len(summary) > 180:
                summary = summary[:180] + "..."
            rows.append((memory_id, title, app_name, summary))
    if not rows:
        return ""
    lines = [
        f"我查到了和“{question}”相关的本地记录，但模型工具调用暂时不稳定。先给你列出可核对的证据摘要：",
        "",
    ]
    for memory_id, title, app_name, summary in rows[:8]:
        app_part = f"（{app_name}）" if app_name else ""
        lines.append(f"- **{title}**{app_part}：{summary}  \n  依据：{memory_id}")
    lines.append("")
    lines.append("依据：" + ", ".join(memory_id for memory_id, _, _, _ in rows[:6]))
    return "\n".join(lines)


def _local_todo_fallback_answer(question: str, outputs: list[tuple[str, str]]) -> str:
    for label, content in outputs:
        if label != "todos":
            continue
        try:
            parsed = json.loads(str(content or "{}"))
        except Exception:
            continue
        if not isinstance(parsed, dict) or not parsed.get("ok"):
            continue
        items = parsed.get("items") or parsed.get("todos") or parsed.get("results") or []
        message = str(parsed.get("message") or "").strip()
        if isinstance(items, list) and not items:
            return "当前没有未完成的 Ariadne 待办。\n\n依据：本地待办列表"
        if not isinstance(items, list):
            items = []
        lines = [f"我查了本地待办，和“{question}”相关的未完成事项如下：", ""]
        for item in items[:10]:
            if not isinstance(item, dict):
                continue
            title = str(item.get("title") or item.get("text") or item.get("id") or "未命名待办").strip()
            status = str(item.get("status") or "").strip()
            priority = str(item.get("priority") or "").strip()
            meta = " · ".join(part for part in (status, priority) if part)
            lines.append(f"- **{title}**" + (f"（{meta}）" if meta else ""))
        if len(lines) == 2 and message:
            lines.append(message)
        lines.append("")
        lines.append("依据：本地待办列表")
        return "\n".join(lines)
    return ""


def _chat_tool_choice(name: str) -> dict[str, Any]:
    return {"type": "function", "function": {"name": name}}


def _todo_tool_requirement(user_prompt: str) -> dict[str, Any]:
    question = _extract_flow_user_question(user_prompt)
    compact = re.sub(r"\s+", "", question.lower())
    full_compact = re.sub(r"\s+", "", str(user_prompt or "").lower())
    if not compact:
        return {}
    add_verbs = ("保存", "添加", "新增", "加入", "记录", "记下", "记一下", "创建", "收进", "提醒我")
    update_verbs = ("完成", "取消", "关闭", "改成", "改为", "标记", "更新")
    todo_terms = ("待办", "todo", "事项", "跟进", "提醒", "没办", "未完成", "待处理")
    has_todo_term = any(term in compact for term in todo_terms)
    retry_add_reference = any(term in compact for term in ("再加一次", "重新加", "加上", "没加成功", "没有加成功", "没保存成功", "没有保存成功"))
    context_mentions_todo = any(term in full_compact for term in ("待办", "todo", "保存待办", "记为待办", "保存为待办"))
    if retry_add_reference and context_mentions_todo:
        return {"required_tool": "add_flow_todo", "mutating": True}
    if has_todo_term and any(verb in compact for verb in add_verbs):
        return {"required_tool": "add_flow_todo", "mutating": True}
    if "保存待办" in compact or "记为待办" in compact or "保存为待办" in compact or "保存到待办" in compact:
        return {"required_tool": "add_flow_todo", "mutating": True}
    if has_todo_term and any(verb in compact for verb in update_verbs):
        return {"required_tool": "list_flow_todos", "mutating": False}
    if has_todo_term:
        return {"required_tool": "list_flow_todos", "mutating": False}
    return {}


def _extract_flow_user_question(user_prompt: str) -> str:
    text = str(user_prompt or "")
    match = re.search(r"(?m)^用户问题[:：]\s*(.+)$", text)
    if match:
        return match.group(1).strip()
    return text.strip().splitlines()[0].strip() if text.strip() else ""


def _missing_required_todo_tool(requirement: dict[str, Any], used_tool_names: list[str], todo_mutation_succeeded: bool) -> str:
    required_tool = str(requirement.get("required_tool") or "").strip()
    if not required_tool:
        return ""
    if required_tool not in used_tool_names:
        return f"模型未调用待办工具 {required_tool}，未保存或读取待办。"
    if bool(requirement.get("mutating")) and not todo_mutation_succeeded:
        return "待办工具未成功执行，未保存待办。"
    return ""


def _missing_required_todo_shell_action(requirement: dict[str, Any], executed_actions: list[str], todo_mutation_succeeded: bool) -> str:
    required_tool = str(requirement.get("required_tool") or "").strip()
    if not required_tool:
        return ""
    required_actions = {
        "add_flow_todo": {"todo-add"},
        "update_flow_todo": {"todo-update"},
        "list_flow_todos": {"todos", "todo-list"},
    }.get(required_tool, set())
    if required_actions and not any(action in required_actions for action in executed_actions):
        return f"模型未调用待办工具 {required_tool}，未保存或读取待办。"
    if bool(requirement.get("mutating")) and not todo_mutation_succeeded:
        return "待办工具未成功执行，未保存待办。"
    return ""


def _tool_output_ok(output: str) -> bool:
    try:
        parsed = json.loads(str(output or "{}"))
    except Exception:
        return False
    return bool(isinstance(parsed, dict) and parsed.get("ok"))


def _compatible_chat_tool_schemas() -> list[dict[str, Any]]:
    return [
        {
            "type": "function",
            "function": {
                "name": "search_flow_memory",
                "description": "Search Ariadne local flow memory with semantic/keyword fallback and return JSON evidence.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "query": {"type": "string"},
                        "limit": {"type": "integer"},
                        "since_hours": {"type": "integer"},
                        "source": {"type": "string"},
                        "app": {"type": "string"},
                    },
                    "required": ["query"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "recent_flow_memory",
                "description": "Return recent non-sensitive Ariadne flow memories as JSON.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "limit": {"type": "integer"},
                        "since_hours": {"type": "integer"},
                        "source": {"type": "string"},
                        "app": {"type": "string"},
                    },
                    "required": [],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "get_flow_memory_entry",
                "description": "Load one Ariadne flow memory entry by id, including text/OCR/frame metadata.",
                "parameters": {
                    "type": "object",
                    "properties": {"entry_id": {"type": "string"}},
                    "required": ["entry_id"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "list_flow_todos",
                "description": "List Ariadne todos for unfinished work, follow-ups, reminders, and pending actions.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "status": {"type": "string"},
                        "query": {"type": "string"},
                        "scope": {"type": "string"},
                        "include_done": {"type": "boolean"},
                        "limit": {"type": "integer"},
                    },
                    "required": [],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "add_flow_todo",
                "description": "Add one Ariadne todo when the task and owner are clear.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "title": {"type": "string"},
                        "note": {"type": "string"},
                        "priority": {"type": "string"},
                        "status": {"type": "string"},
                        "scope": {"type": "string"},
                        "evidence": {"type": "string"},
                    },
                    "required": ["title"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "update_flow_todo",
                "description": "Update one Ariadne todo status or metadata by id.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "todo_id": {"type": "string"},
                        "status": {"type": "string"},
                        "title": {"type": "string"},
                        "note": {"type": "string"},
                        "priority": {"type": "string"},
                        "scope": {"type": "string"},
                    },
                    "required": ["todo_id"],
                    "additionalProperties": False,
                },
            },
        },
    ]


def _assistant_message_from_tool_calls(message: Any, tool_calls: list[Any]) -> dict[str, Any]:
    payload = {"role": "assistant", "content": str(getattr(message, "content", "") or "")}
    serialized_calls = []
    for tool_call in tool_calls:
        function = getattr(tool_call, "function", None)
        tool_name = str(getattr(function, "name", "") or "").strip()
        tool_args = _tool_arguments_dict(getattr(function, "arguments", None))
        serialized_calls.append(
            {
                "id": str(getattr(tool_call, "id", "") or ""),
                "type": "function",
                "function": {
                    "name": tool_name,
                    "arguments": json.dumps(tool_args, ensure_ascii=False, separators=(",", ":")),
                },
            }
        )
    payload["tool_calls"] = serialized_calls
    return payload


def _tool_arguments_dict(raw: Any) -> dict[str, Any]:
    normalized = _normalize_tool_arguments(raw)
    text = normalized if normalized is not None else str(raw or "{}").strip()
    if not text:
        return {}
    try:
        parsed = json.loads(text)
    except Exception:
        return {}
    if isinstance(parsed, dict):
        return parsed
    return {}


def _run_compatible_tool(cli_command: str, user_prompt: str, tool_name: str, args: dict[str, Any]) -> str:
    if tool_name == "search_flow_memory":
        query = str(args.get("query") or user_prompt).strip()
        limit = _bounded_int(args.get("limit", 8), 1, 20)
        since_hours = _bounded_int(args.get("since_hours", 24), 1, 24 * 30)
        cli_args = ["--query", query, "--limit", str(limit), "--since-hours", str(since_hours)]
        if args.get("source"):
            cli_args += ["--source", str(args.get("source"))]
        if args.get("app"):
            cli_args += ["--app", str(args.get("app"))]
        return _run_workmemory_cli(cli_command, "search", cli_args)
    if tool_name == "recent_flow_memory":
        limit = _bounded_int(args.get("limit", 8), 1, 20)
        since_hours = _bounded_int(args.get("since_hours", 24), 1, 24 * 30)
        cli_args = ["--limit", str(limit), "--since-hours", str(since_hours)]
        if args.get("source"):
            cli_args += ["--source", str(args.get("source"))]
        if args.get("app"):
            cli_args += ["--app", str(args.get("app"))]
        return _run_workmemory_cli(cli_command, "recent", cli_args)
    if tool_name == "get_flow_memory_entry":
        entry_id = str(args.get("entry_id") or args.get("id") or "").strip()
        if not entry_id:
            return json.dumps({"ok": False, "message": "get_flow_memory_entry 缺少 entry_id"}, ensure_ascii=False)
        return _run_workmemory_cli(cli_command, "get", ["--id", entry_id])
    if tool_name == "list_flow_todos":
        limit = _bounded_int(args.get("limit", 20), 1, 50)
        cli_args = ["--limit", str(limit)]
        if args.get("status"):
            cli_args += ["--status", str(args.get("status"))]
        if args.get("query"):
            cli_args += ["--query", str(args.get("query"))]
        if args.get("scope"):
            cli_args += ["--scope", str(args.get("scope"))]
        if bool(args.get("include_done")):
            cli_args += ["--include-done"]
        return _run_workmemory_cli(cli_command, "todos", cli_args)
    if tool_name == "add_flow_todo":
        title = str(args.get("title") or "").strip()
        if not title:
            return json.dumps({"ok": False, "message": "add_flow_todo 缺少 title"}, ensure_ascii=False)
        cli_args = ["--title", title]
        for key, flag in (("note", "--text"), ("priority", "--priority"), ("status", "--status"), ("scope", "--scope"), ("evidence", "--evidence")):
            value = str(args.get(key) or "").strip()
            if value:
                cli_args += [flag, value]
        return _run_workmemory_cli(cli_command, "todo-add", cli_args)
    if tool_name == "update_flow_todo":
        todo_id = str(args.get("todo_id") or args.get("id") or "").strip()
        if not todo_id:
            return json.dumps({"ok": False, "message": "update_flow_todo 缺少 todo_id"}, ensure_ascii=False)
        cli_args = ["--id", todo_id]
        for key, flag in (("status", "--status"), ("title", "--title"), ("note", "--text"), ("priority", "--priority"), ("scope", "--scope")):
            value = str(args.get(key) or "").strip()
            if value:
                cli_args += [flag, value]
        return _run_workmemory_cli(cli_command, "todo-update", cli_args)
    return json.dumps({"ok": False, "message": f"未知工具: {tool_name}"}, ensure_ascii=False)


def _install_chat_tool_argument_normalizer(client: Any) -> None:
    completions = getattr(getattr(client, "chat", None), "completions", None)
    create = getattr(completions, "create", None)
    if completions is None or create is None or getattr(completions, "_ariadne_glm_tool_normalizer", False):
        return

    async def create_with_normalized_tool_arguments(*args: Any, **kwargs: Any) -> Any:
        response = await create(*args, **kwargs)
        return _normalize_chat_completion_tool_arguments(response)

    setattr(completions, "create", create_with_normalized_tool_arguments)
    setattr(completions, "_ariadne_glm_tool_normalizer", True)


def _normalize_chat_completion_tool_arguments(response: Any) -> Any:
    for choice in getattr(response, "choices", None) or []:
        message = getattr(choice, "message", None)
        for tool_call in getattr(message, "tool_calls", None) or []:
            function = getattr(tool_call, "function", None)
            if function is None:
                continue
            normalized = _normalize_tool_arguments(getattr(function, "arguments", None))
            if normalized is not None:
                try:
                    function.arguments = normalized
                except Exception:
                    pass
    return response


def _normalize_tool_arguments(raw: Any) -> str | None:
    if not isinstance(raw, str):
        return None
    text = raw.strip()
    if not _looks_like_xml_tool_arguments(text):
        return None
    args: dict[str, Any] = {}
    for key, value in re.findall(r"<arg_key>(.*?)</arg_key>\s*<arg_value>(.*?)</arg_value>", text, re.S | re.I):
        key = html.unescape(key).strip()
        if not key:
            continue
        args[key] = _coerce_tool_argument_value(html.unescape(value).strip())
    return json.dumps(args, ensure_ascii=False, separators=(",", ":"))


def _looks_like_xml_tool_arguments(text: str) -> bool:
    lowered = text.lower()
    if "<tool_call" in lowered or "</tool_call>" in lowered:
        return True
    return "<arg_key>" in lowered and "<arg_value>" in lowered


def _coerce_tool_argument_value(value: str) -> Any:
    text = value.strip()
    if text == "":
        return ""
    lowered = text.lower()
    if lowered == "true":
        return True
    if lowered == "false":
        return False
    if lowered == "null":
        return None
    if re.fullmatch(r"-?\d+", text):
        try:
            return int(text)
        except Exception:
            return text
    if re.fullmatch(r"-?(?:\d+\.\d*|\d*\.\d+)(?:[eE][+-]?\d+)?", text) or re.fullmatch(r"-?\d+[eE][+-]?\d+", text):
        try:
            return float(text)
        except Exception:
            return text
    if text[:1] in "{[":
        try:
            return json.loads(text)
        except Exception:
            return text
    return text


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
    todo_requirement = _todo_tool_requirement(user_prompt)
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
                    "description": "Query Ariadne local flow memory, todo list, timeline, OCR, clipboard, window context, and evidence details.",
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
    missing_required_tool = _missing_required_todo_shell_action(
        todo_requirement,
        shell.executed_actions,
        shell.todo_mutation_succeeded,
    )
    if missing_required_tool:
        return {"ok": False, "error": missing_required_tool}
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
        self.executed_actions: list[str] = []
        self.todo_mutation_succeeded = False

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
        self.executed_actions.append(action)
        if action in {"todo-add", "todo-update"} and _tool_output_ok(output):
            self.todo_mutation_succeeded = True
        return {"stdout": output, "stderr": "", "exit_code": 0, "timed_out": False}


def _write_skill_directory(skill: str) -> Path:
    root = Path(tempfile.gettempdir()) / "ariadne-agent-skills" / "ariadne-flow-memory"
    root.mkdir(parents=True, exist_ok=True)
    (root / "SKILL.md").write_text(skill, encoding="utf-8")
    return root


def _should_try_native_shell_skill(provider: str, base_url: str, payload: dict) -> bool:
    if _truthy(os.environ.get("ARIADNE_FLOW_AGENT_FORCE_FUNCTION_TOOLS")):
        return False

    if _truthy(os.environ.get("ARIADNE_FLOW_AGENT_NATIVE_SKILLS_STRICT")):
        return True

    requested_native = _truthy(payload.get("nativeSkills")) or _truthy(os.environ.get("ARIADNE_FLOW_AGENT_NATIVE_SKILLS"))
    if provider != "openai":
        return requested_native and _truthy(os.environ.get("ARIADNE_FLOW_AGENT_ALLOW_COMPAT_NATIVE_SKILLS"))
    if requested_native:
        return True

    parsed = urlparse(base_url)
    host = (parsed.netloc or parsed.path).lower()
    return "api.openai.com" in host or "api.openai.azure.com" in host


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
    if action not in {"status", "refresh", "search", "recent", "timeline", "get", "add-note", "todos", "todo-list", "todo-add", "todo-update", "todo-delete"}:
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
