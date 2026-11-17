#!/usr/bin/env python3
"""Render both deployment paths and check shared safety/configuration contracts.

Uses example configuration only. Does not start, build, pull or deploy services.
"""

import json
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[1]
FLAGS = ("BOOKING_LEGAL_ENFORCEMENT", "TRADER_VERIFICATION_ENFORCEMENT")


def render(filename, flag_values, image_tag=""):
    # Do not read real deployment credentials or let ambient provider settings
    # change this configuration-only check.
    env = {"PATH": os.environ["PATH"], "IMAGE_TAG": image_tag,
           **dict(zip(FLAGS, flag_values))}
    result = subprocess.run(
        ["docker", "compose", "--project-directory", str(ROOT),
         "--env-file", str(ROOT / "deploy/.env.example"),
         "-f", str(ROOT / "deploy" / filename), "config", "--format", "json"],
        cwd=ROOT, env=env, check=True, capture_output=True, text=True,
    )
    return json.loads(result.stdout)


def require(condition, message):
    if not condition:
        raise SystemExit(message)


def main():
    for values in [("", ""), ("false", "false"), ("true", "false"),
                   ("false", "true"), ("true", "true")]:
        source = render("compose.yml", values)
        prebuilt = render("compose.prebuilt.yml", values)
        for config in (source, prebuilt):
            services = config["services"]
            api = services["api"]
            require(api["logging"] == {"driver": "json-file", "options": {"max-size": "10m", "max-file": "3"}},
                    "Bound API request log disk usage in both deployments")
            for flag, value in zip(FLAGS, values):
                require(api["environment"][flag] == (value or "false"),
                        f"API must forward {flag} with a disabled default")
            require(not services["db"].get("ports") and not api.get("ports"),
                    "Database and API must remain private")
            require(api["depends_on"]["migrate"]["condition"] == "service_completed_successfully",
                    "API must wait for migrations")
            require(services["db"]["volumes"][0]["source"] == "postgres_data",
                    "Preserve the existing database volume")
        require(source["name"] == prebuilt["name"] == "restringing",
                "Preserve the deployment project name")
        require(source["volumes"] == prebuilt["volumes"], "Database volumes must match")
        require(source["services"]["api"]["environment"] == prebuilt["services"]["api"]["environment"],
                "Build and prebuilt API configuration must match")
        for service, context in {"api": "backend", "migrate": "backend",
                                 "web": "frontend", "landing": "deploy/landing"}.items():
            require(source["services"][service]["build"]["context"] == str(ROOT / context),
                    f"{service} build context must resolve from repository root")
            require("build" not in prebuilt["services"][service],
                    "Prebuilt deployment must not build on the VPS")
            require(prebuilt["services"][service]["image"] ==
                    f"ghcr.io/daviddoherty10/restringing-ie-{service}:main-amd64",
                    f"{service} must default to its AMD64 image")
    for tag in ("main-arm64", "a926457d5b205320a94691ac4c04458e8bc1ae1b-amd64"):
        config = render("compose.prebuilt.yml", ("", ""), tag)
        for service in ("api", "migrate", "web", "landing"):
            require(config["services"][service]["image"] ==
                    f"ghcr.io/daviddoherty10/restringing-ie-{service}:{tag}",
                    f"{service} must respect the selected architecture/release tag")
    print("Both deployment configurations pass: contexts, flags, images, migrations and volumes")


if __name__ == "__main__":
    main()
