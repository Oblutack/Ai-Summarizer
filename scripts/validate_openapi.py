#!/usr/bin/env python3
"""Checks that docs/openapi.yaml is a valid OpenAPI document, and that it describes the routes the API really has.

    pip install openapi-spec-validator pyyaml
    python scripts/validate_openapi.py

The second check reads go-api/server/server.go: every /v1 route registered there (and the API key routes) must be in
the document, and the document must not describe routes that are gone, so the two cannot drift apart unnoticed.
"""

import re
import sys
from pathlib import Path

import yaml
from openapi_spec_validator import validate

ROOT = Path(__file__).resolve().parent.parent
SPEC = ROOT / "docs" / "openapi.yaml"
ROUTES = ROOT / "go-api" / "server" / "server.go"

# The routes that make up the API for programs, as they appear in server.go.
DOCUMENTED = re.compile(r'\b(?:v1|authorized)\.(GET|POST|DELETE|PUT)\("(/v1/[^"]*|/account/api-keys[^"]*)"')


def main() -> int:
    spec = yaml.safe_load(SPEC.read_text(encoding="utf-8"))
    validate(spec)

    # server.go registers the /v1 routes on a group (so they read "/summarize"): add the prefix back.
    source = ROUTES.read_text(encoding="utf-8")
    in_v1 = re.search(r'v1 := r\.Group\("/v1".*?\n\t\}', source, re.S)
    routes = set()
    if in_v1:
        for method, path in re.findall(r'v1\.(GET|POST)\("(/[^"]*)"', in_v1.group(0)):
            routes.add((method.lower(), "/v1" + path))
    for method, path in DOCUMENTED.findall(source):
        if path.startswith("/account/api-keys"):
            routes.add((method.lower(), re.sub(r":(\w+)", r"{\1}", path)))

    described = {(method, path) for path, item in spec["paths"].items() for method in item if method in {"get", "post", "put", "delete"}}
    missing = sorted(routes - described)
    stale = sorted(described - routes)
    if not routes:
        print("could not find the API routes in go-api/server/server.go: has it been reorganized?")
        return 1
    for method, path in missing:
        print(f"in server.go but not in openapi.yaml: {method.upper()} {path}")
    for method, path in stale:
        print(f"in openapi.yaml but not in server.go: {method.upper()} {path}")
    if missing or stale:
        return 1
    print(f"openapi.yaml is valid ({spec['openapi']}) and describes all {len(routes)} routes of the API")
    return 0


if __name__ == "__main__":
    sys.exit(main())
