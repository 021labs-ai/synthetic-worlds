"""Synthetic Worlds client — standalone SDK without RDK dependencies."""

from __future__ import annotations

import functools
import inspect
import json
import logging
import os
from typing import Any, Callable, get_type_hints

import httpx

logger = logging.getLogger(__name__)

DEFAULT_URL = "http://localhost:7878"


# ── ToolSpec ────────────────────────────────────────────


_TYPE_MAP: dict[type, str] = {
    str: "string",
    int: "integer",
    float: "number",
    bool: "boolean",
    list: "array",
    dict: "object",
}


class ToolSpec:
    """JSON Schema descriptor for a tool function.

    Use ``ToolSpec.from_function(fn)`` to auto-extract the schema from
    type hints and docstring.
    """

    __slots__ = ("name", "description", "parameters", "returns", "_param_names")

    def __init__(
        self,
        name: str,
        description: str,
        parameters: dict,
        returns: dict,
        param_names: list[str] | None = None,
    ) -> None:
        self.name = name
        self.description = description
        self.parameters = parameters
        self.returns = returns
        self._param_names = param_names or []

    @classmethod
    def from_function(cls, fn: Callable) -> ToolSpec:
        """Build a ToolSpec from a function's type hints and docstring."""
        hints = get_type_hints(fn)
        sig = inspect.signature(fn)
        return_hint = hints.pop("return", dict)

        properties: dict[str, dict] = {}
        required: list[str] = []
        param_names: list[str] = []

        for pname, param in sig.parameters.items():
            hint = hints.get(pname, str)
            properties[pname] = {"type": _TYPE_MAP.get(hint, "string")}
            if param.default is inspect.Parameter.empty:
                required.append(pname)
            param_names.append(pname)

        return cls(
            name=fn.__name__,
            description=inspect.getdoc(fn) or "",
            parameters={
                "type": "object",
                "properties": properties,
                **({"required": required} if required else {}),
            },
            returns={"type": _TYPE_MAP.get(return_hint, "object")},
            param_names=param_names,
        )

    @classmethod
    def from_openai(cls, tool: dict) -> ToolSpec:
        """Build a ToolSpec from an OpenAI function-calling tool definition."""
        func = tool.get("function", tool)
        name = func["name"]
        params = func.get("parameters", {"type": "object", "properties": {}})
        return cls(
            name=name,
            description=func.get("description", ""),
            parameters=params,
            returns={"type": "object"},
            param_names=list(params.get("properties", {}).keys()),
        )

    @classmethod
    def from_anthropic(cls, tool: dict) -> ToolSpec:
        """Build a ToolSpec from an Anthropic tool definition."""
        name = tool["name"]
        params = tool.get("input_schema", {"type": "object", "properties": {}})
        return cls(
            name=name,
            description=tool.get("description", ""),
            parameters=params,
            returns={"type": "object"},
            param_names=list(params.get("properties", {}).keys()),
        )

    def to_schema(self) -> dict:
        return {
            "name": self.name,
            "description": self.description,
            "parameters": self.parameters,
            "returns": self.returns,
        }

    def build_input(self, *args: Any, **kwargs: Any) -> dict:
        """Map positional + keyword args to a dict using the parameter names."""
        result = {}
        for i, val in enumerate(args):
            if i < len(self._param_names):
                result[self._param_names[i]] = val
        result.update(kwargs)
        return result


# ── SyntheticWorld ──────────────────────────────────────


class SyntheticWorld:
    """A synthetic world for simulating tool calls via LLM-generated responses.

    Use as a context manager or call `close()` when done::

        with create_synthetic_world(project_id="proj") as world:
            @world.tool
            def get_user(user_id: str) -> dict: ...

            result = get_user("u1")
    """

    def __init__(
        self,
        world_id: str,
        project_id: str,
        mode: str,
        seed: int | None,
        expires_at: str,
        client: httpx.Client,
    ) -> None:
        self.world_id = world_id
        self.project_id = project_id
        self.mode = mode
        self.seed = seed
        self.expires_at = expires_at
        self._client = client
        self._tools: dict[str, ToolSpec] = {}
        self._closed = False

    # ── Tool registration ───────────────────────────────

    def tool(self, fn: Callable) -> Callable:
        """Decorator to register a function as a synthetic tool."""
        spec = ToolSpec.from_function(fn)
        self._tools[spec.name] = spec

        @functools.wraps(fn)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            input_data = spec.build_input(*args, **kwargs)
            return self.call(spec.name, input_data)

        return wrapper

    def register_tool(self, spec: ToolSpec) -> None:
        """Register a pre-built ToolSpec without decorating a function."""
        self._tools[spec.name] = spec

    def register_openai_tools(self, tools: list[dict]) -> None:
        """Register tools from OpenAI function-calling format."""
        for tool_def in tools:
            spec = ToolSpec.from_openai(tool_def)
            self._tools[spec.name] = spec

    def register_anthropic_tools(self, tools: list[dict]) -> None:
        """Register tools from Anthropic format."""
        for tool_def in tools:
            spec = ToolSpec.from_anthropic(tool_def)
            self._tools[spec.name] = spec

    # ── Schema export ────────────────────────────────────

    def as_openai_tools(self) -> list[dict]:
        """Export registered tools in OpenAI function-calling format."""
        return [
            {
                "type": "function",
                "function": {
                    "name": spec.name,
                    "description": spec.description,
                    "parameters": spec.parameters,
                },
            }
            for spec in self._tools.values()
        ]

    def as_anthropic_tools(self) -> list[dict]:
        """Export registered tools in Anthropic format."""
        return [
            {
                "name": spec.name,
                "description": spec.description,
                "input_schema": spec.parameters,
            }
            for spec in self._tools.values()
        ]

    # ── Operations ──────────────────────────────────────

    def call(
        self,
        tool_name: str,
        input_data: dict,
        *,
        idempotency_key: str | None = None,
    ) -> dict:
        """Execute a synthetic tool call and return the generated output."""
        self._ensure_open()
        spec = self._tools.get(tool_name)

        resp = self._post(
            "/v1/synthetic/call",
            {
                "world_id": self.world_id,
                "tool_name": tool_name,
                "tool_schema": spec.to_schema() if spec else {},
                "input": input_data,
                "idempotency_key": idempotency_key,
            },
        )
        return resp["output"]

    def reset(self, *, hard: bool = False) -> None:
        """Reset the world's step counter and optionally clear all state."""
        self._ensure_open()
        self._post(f"/v1/synthetic/worlds/{self.world_id}/reset", {"hard": hard})

    def close(self) -> None:
        """Destroy the world and free its resources."""
        if self._closed:
            return
        try:
            self._client.delete(f"/v1/synthetic/worlds/{self.world_id}")
        except Exception as exc:
            logger.warning(f"Failed to close world {self.world_id}: {exc}")
        finally:
            self._client.close()
            self._closed = True

    # ── Context manager ─────────────────────────────────

    def __enter__(self) -> SyntheticWorld:
        return self

    def __exit__(self, *exc: Any) -> None:
        self.close()

    # ── Internals ───────────────────────────────────────

    def _ensure_open(self) -> None:
        if self._closed:
            raise RuntimeError(f"World {self.world_id} is closed")

    def _post(self, path: str, body: dict) -> dict:
        resp = self._client.post(path, json=body)
        resp.raise_for_status()
        return resp.json()


# ── Factory ─────────────────────────────────────────────


def create_synthetic_world(
    project_id: str = "default",
    *,
    mode: str = "stateful",
    seed: int | None = None,
    model: str | None = None,
    failure_profile: dict | None = None,
    url: str | None = None,
    api_key: str | None = None,
    timeout: float = 30.0,
) -> SyntheticWorld:
    """Create a new synthetic world.

    Args:
        project_id: The project to scope the world to.
        mode: Context richness for LLM generation:
            - ``"schema_only"`` -- schema + input only
            - ``"examples"`` -- adds real tool spans from imported traces
            - ``"stateful"`` -- examples + accumulated state (default)
        seed: Optional seed for deterministic output.
        model: LLM model to use for generation.
        failure_profile: Optional error simulation config, e.g.
            ``{"rate": 0.3, "codes": ["timeout"]}``.
        url: Synthetic Worlds server URL. Defaults to
            ``SYNTHETIC_WORLDS_URL`` env var or ``http://localhost:7878``.
        api_key: Bearer token for authentication. Defaults to
            ``SYNTHETIC_WORLDS_API_KEY`` env var.
        timeout: HTTP timeout in seconds.

    Returns:
        A :class:`SyntheticWorld` instance (usable as a context manager).
    """
    base_url = url or os.environ.get("SYNTHETIC_WORLDS_URL", DEFAULT_URL)
    key = api_key or os.environ.get("SYNTHETIC_WORLDS_API_KEY", "")
    if not key:
        raise ValueError(
            "api_key is required. Pass it directly or set SYNTHETIC_WORLDS_API_KEY."
        )

    client = httpx.Client(
        base_url=base_url,
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        timeout=timeout,
    )

    body: dict[str, Any] = {
        "project_id": project_id,
        "mode": mode,
    }
    if seed is not None:
        body["seed"] = seed
    if model is not None:
        body["model"] = model
    if failure_profile is not None:
        body["failure_profile"] = failure_profile

    try:
        resp = client.post("/v1/synthetic/worlds", json=body)
        resp.raise_for_status()
    except Exception:
        client.close()
        raise
    data = resp.json()

    return SyntheticWorld(
        world_id=data["world_id"],
        project_id=data.get("project_id", project_id),
        mode=data.get("mode", mode),
        seed=data.get("seed"),
        expires_at=data.get("expires_at", ""),
        client=client,
    )
