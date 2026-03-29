"""Helpers for importing traces into the Synthetic Worlds server."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any

import httpx


def import_traces(
    traces: list[dict],
    *,
    url: str | None = None,
    api_key: str | None = None,
) -> dict:
    """Import traces in native format.

    Args:
        traces: List of trace dicts, each with ``name`` and ``spans`` fields.
            Each span needs ``name``, ``input``, and ``output``.
        url: Server URL. Defaults to SYNTHETIC_WORLDS_URL or http://localhost:7878.
        api_key: API key. Defaults to SYNTHETIC_WORLDS_API_KEY.

    Returns:
        Import result with ``traces_imported`` and ``spans_imported`` counts.

    Example::

        import_traces([
            {
                "name": "customer-support-flow",
                "spans": [
                    {"name": "get_user", "input": {"id": "u1"}, "output": {"name": "Alice"}},
                    {"name": "list_tickets", "input": {"user_id": "u1"}, "output": [{"id": "t1"}]},
                ]
            }
        ])
    """
    base_url = url or os.environ.get("SYNTHETIC_WORLDS_URL", "http://localhost:7878")
    key = api_key or os.environ.get("SYNTHETIC_WORLDS_API_KEY", "")

    with httpx.Client(
        base_url=base_url,
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        timeout=60.0,
    ) as client:
        resp = client.post("/v1/traces/import", json={"traces": traces})
        resp.raise_for_status()
        return resp.json()


def import_from_file(
    path: str | Path,
    *,
    url: str | None = None,
    api_key: str | None = None,
) -> dict:
    """Import traces from a JSON or JSONL file.

    Supported formats:
    - JSON: ``{"traces": [...]}`` or ``[...]`` (list of traces)
    - JSONL: one trace per line

    Args:
        path: Path to the file.
        url: Server URL.
        api_key: API key.

    Returns:
        Import result dict.
    """
    path = Path(path)
    content = path.read_text()

    if path.suffix == ".jsonl":
        traces = [json.loads(line) for line in content.strip().splitlines() if line.strip()]
    else:
        data = json.loads(content)
        if isinstance(data, list):
            traces = data
        elif "traces" in data:
            traces = data["traces"]
        else:
            raise ValueError("Expected a list of traces or {\"traces\": [...]}")

    return import_traces(traces, url=url, api_key=api_key)


def import_langfuse(
    data: list[dict] | str | Path,
    *,
    url: str | None = None,
    api_key: str | None = None,
) -> dict:
    """Import traces from Langfuse export format.

    Args:
        data: Langfuse export data (list of dicts, JSON string, or file path).
        url: Server URL.
        api_key: API key.

    Returns:
        Import result dict.
    """
    base_url = url or os.environ.get("SYNTHETIC_WORLDS_URL", "http://localhost:7878")
    key = api_key or os.environ.get("SYNTHETIC_WORLDS_API_KEY", "")

    if isinstance(data, (str, Path)):
        raw = Path(data).read_bytes() if Path(data).exists() else data.encode() if isinstance(data, str) else data
    else:
        raw = json.dumps(data).encode()

    with httpx.Client(
        base_url=base_url,
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        timeout=60.0,
    ) as client:
        resp = client.post("/v1/traces/import/langfuse", content=raw)
        resp.raise_for_status()
        return resp.json()


def import_langsmith(
    data: list[dict] | str | Path,
    *,
    url: str | None = None,
    api_key: str | None = None,
) -> dict:
    """Import traces from LangSmith export format.

    Args:
        data: LangSmith export data (list of dicts, JSON string, or file path).
        url: Server URL.
        api_key: API key.

    Returns:
        Import result dict.
    """
    base_url = url or os.environ.get("SYNTHETIC_WORLDS_URL", "http://localhost:7878")
    key = api_key or os.environ.get("SYNTHETIC_WORLDS_API_KEY", "")

    if isinstance(data, (str, Path)):
        raw = Path(data).read_bytes() if Path(data).exists() else data.encode() if isinstance(data, str) else data
    else:
        raw = json.dumps(data).encode()

    with httpx.Client(
        base_url=base_url,
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        timeout=60.0,
    ) as client:
        resp = client.post("/v1/traces/import/langsmith", content=raw)
        resp.raise_for_status()
        return resp.json()
