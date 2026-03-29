"""Synthetic Worlds SDK.

Create stateful synthetic worlds for fast local iteration with realistic tool
responses. Every world maintains a knowledge graph so multi-step interactions
are coherent — a user created in step 1 is remembered in step 3.

Example:
    from synthetic_worlds import create_synthetic_world

    with create_synthetic_world(project_id="my-project") as world:
        @world.tool
        def get_user(user_id: str) -> dict:
            \"\"\"Fetch a user by ID.\"\"\"
            ...

        result = get_user("u1")
        print(result)  # {"id": "u1", "name": "Alice", ...}
"""

from synthetic_worlds.client import (
    SyntheticWorld,
    ToolSpec,
    create_synthetic_world,
)

__all__ = [
    "SyntheticWorld",
    "ToolSpec",
    "create_synthetic_world",
]
