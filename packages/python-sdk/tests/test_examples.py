"""Run each example's offline ``--self-test`` so the examples cannot silently rot."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

EXAMPLES = Path(__file__).resolve().parent.parent / "examples"


def _load(name: str):
    spec = importlib.util.spec_from_file_location(f"example_{name}", EXAMPLES / f"{name}.py")
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@pytest.mark.parametrize("name", ["pagination", "errors", "sse", "webhooks"])
def test_example_self_test_passes(name: str) -> None:
    assert _load(name).main(["--self-test"]) == 0
