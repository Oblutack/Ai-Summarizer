#!/usr/bin/env python3
"""Writes CHANGELOG.md from the git history.

Every commit follows the Conventional Commits style ("feat(go-api): add API keys"), so the history already says what
changed. This turns it into a list a person can read: what was added, changed and fixed, newest month first. Tests,
docs, CI and dependency commits are left out, as they change nothing for the people using Inkling.

    python scripts/changelog.py                 print it
    python scripts/changelog.py -o CHANGELOG.md write it to a file
    python scripts/changelog.py --since v1.0.0  only what came after a tag or commit
"""

import argparse
import re
import subprocess
import sys
from collections import defaultdict

# What each commit type is called in the changelog, in the order the sections appear.
SECTIONS = [("feat", "Added"), ("refactor", "Changed"), ("perf", "Changed"), ("fix", "Fixed")]
HEADINGS = ["Added", "Changed", "Fixed"]
PATTERN = re.compile(r"^(?P<type>[a-z]+)(?:\((?P<scope>[^)]*)\))?(?P<breaking>!)?: (?P<text>.+)$")

PREAMBLE = """# Changelog

What changed in Inkling, newest first. This file is generated from the commit history by `scripts/changelog.py`;
run `python scripts/changelog.py -o CHANGELOG.md` to bring it up to date. Only changes you would notice are listed:
new features, changes and fixes. Tests, documentation, CI and dependency updates are in the history but not here.
"""


def commits(since):
    command = ["git", "log", "--no-merges", "--reverse", "--format=%ad%x1f%s", "--date=format:%Y-%m"]
    if since:
        command.append(f"{since}..HEAD")
    out = subprocess.run(command, capture_output=True, text=True, encoding="utf-8", check=True).stdout
    for line in out.splitlines():
        month, _, subject = line.partition("\x1f")
        match = PATTERN.match(subject.strip())
        if match:
            yield month, match


def sentence(text):
    text = text.strip()
    return text[:1].upper() + text[1:]


def build(since=None):
    by_month = defaultdict(lambda: defaultdict(list))
    types = dict(SECTIONS)
    for month, match in commits(since):
        heading = types.get(match["type"])
        if heading is None:
            continue
        scope = f"**{match['scope']}:** " if match["scope"] else ""
        breaking = " (breaking)" if match["breaking"] else ""
        by_month[month][heading].append(f"- {scope}{sentence(match['text'])}{breaking}")

    lines = [PREAMBLE]
    for month in sorted(by_month, reverse=True):
        lines.append(f"\n## {month}\n")
        for heading in HEADINGS:
            entries = by_month[month].get(heading)
            if entries:
                lines.append(f"### {heading}\n")
                # newest first within a section, as the months are
                lines.extend(reversed(entries))
                lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def main():
    parser = argparse.ArgumentParser(description="Write the changelog from the git history.")
    parser.add_argument("-o", "--output", help="write to this file instead of printing")
    parser.add_argument("--since", help="only the commits after this tag or commit")
    args = parser.parse_args()
    text = build(args.since)
    if args.output:
        with open(args.output, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(text)
    else:
        sys.stdout.buffer.write(text.encode("utf-8"))


if __name__ == "__main__":
    main()
