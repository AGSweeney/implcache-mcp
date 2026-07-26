#!/usr/bin/env python3
"""LibraryDocs knowledge extraction validator.

Usage (from the target repository root):

    python path/to/validate_librarydocs.py --repo-root . --strict

Arguments:
  --repo-root   Repository root containing LibraryDocs/ (default: current directory)
  --strict      Treat standards-related warnings as failures (required for completion)

The script location does not imply the repository root. Pass --repo-root explicitly
when invoking from a global skill install under %USERPROFILE%\\.cursor\\skills\\.
"""
from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


# ---------------------------------------------------------------------------
# Paths — repository root is always supplied by the caller (--repo-root)
# ---------------------------------------------------------------------------

def libdocs_root(root: Path) -> Path:
    return root / "LibraryDocs"


# ---------------------------------------------------------------------------
# Reporting
# ---------------------------------------------------------------------------

@dataclass
class Report:
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)

    def error(self, msg: str) -> None:
        self.errors.append(msg)

    def warn(self, msg: str) -> None:
        self.warnings.append(msg)


# ---------------------------------------------------------------------------
# Markdown / YAML helpers
# ---------------------------------------------------------------------------

def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8", errors="replace")


def split_frontmatter(text: str) -> tuple[dict[str, Any], str, bool]:
    """Parse YAML-like frontmatter without external dependencies."""
    if not text.startswith("---"):
        return {}, text, False
    end = text.find("\n---", 3)
    if end == -1:
        return {}, text, False
    block = text[3:end].strip()
    body = text[end + 4 :].lstrip("\n")
    meta: dict[str, Any] = {}
    current_key: str | None = None
    current_list: list[str] | None = None
    nested_key: str | None = None
    nested: dict[str, Any] = {}

    def flush_list() -> None:
        nonlocal current_key, current_list
        if current_key and current_list is not None:
            meta[current_key] = current_list
        current_key = None
        current_list = None

    def flush_nested() -> None:
        nonlocal nested_key, nested
        if nested_key and nested:
            meta[nested_key] = nested
        nested_key = None
        nested = {}

    for raw_line in block.splitlines():
        line = raw_line.rstrip()
        if not line.strip() or line.strip().startswith("#"):
            continue
        if line.startswith("  ") and nested_key:
            m = re.match(r"\s+(\w+):\s*(.*)$", line)
            if m:
                val = m.group(2).strip().strip('"').strip("'")
                nested[m.group(1)] = val
            elif line.startswith("  - "):
                # retrieval questions list
                if "questions" not in nested:
                    nested["questions"] = []
                if isinstance(nested["questions"], list):
                    nested["questions"].append(line.strip()[2:].strip().strip('"').strip("'"))
            continue
        if line.startswith("  - ") and current_key:
            if current_list is None:
                current_list = []
            current_list.append(line.strip()[2:].strip().strip('"').strip("'"))
            continue
        if nested_key and line.startswith("  ") and re.match(r"\s+(\w+):\s*$", line):
            # nested empty key under retrieval: — ignore, sub-lines follow
            continue
        flush_list()
        flush_nested()
        if re.match(r"^retrieval:\s*$", line):
            nested_key = "retrieval"
            nested = {}
            continue
        m = re.match(r"([\w.-]+):\s*(.*)$", line)
        if not m:
            continue
        key, val = m.group(1), m.group(2).strip()
        if val == "" or val == "|":
            current_key = key
            current_list = []
        elif val.startswith("[") and val.endswith("]"):
            inner = val[1:-1].strip()
            meta[key] = [x.strip().strip('"').strip("'") for x in inner.split(",") if x.strip()]
            current_key = None
        else:
            meta[key] = val.strip('"').strip("'")
            current_key = None
    flush_list()
    flush_nested()
    return meta, body, True


def parse_md_tables(text: str) -> list[list[dict[str, str]]]:
    """Return list of tables; each table is list of row dicts."""
    tables: list[list[dict[str, str]]] = []
    lines = text.splitlines()
    i = 0
    while i < len(lines):
        line = lines[i]
        if "|" not in line:
            i += 1
            continue
        if i + 1 >= len(lines) or not re.match(r"^\|[\s\-:|]+\|?\s*$", lines[i + 1]):
            i += 1
            continue
        header = [c.strip() for c in line.strip().strip("|").split("|")]
        i += 2
        rows: list[dict[str, str]] = []
        while i < len(lines) and "|" in lines[i]:
            cells = [c.strip() for c in lines[i].strip().strip("|").split("|")]
            if len(cells) == len(header):
                rows.append(dict(zip(header, cells)))
            i += 1
        if rows:
            tables.append(rows)
    return tables


def table_after_heading(text: str, heading: str) -> list[dict[str, str]] | None:
    idx = text.find(heading)
    if idx == -1:
        return None
    sub = text[idx + len(heading) :]
    tables = parse_md_tables(sub)
    return tables[0] if tables else None


def parse_artifact_ids(cell: str) -> list[str]:
    if not cell or cell in {"—", "-", "n/a", "N/A", ""}:
        return []
    return [x.strip() for x in re.split(r",\s*", cell) if x.strip() and x.strip() not in {"—", "-"}]


def normalize_md_link_target(raw: str) -> str:
    t = raw.strip().split("#")[0].strip()
    return t


# ---------------------------------------------------------------------------
# Inventory model
# ---------------------------------------------------------------------------

@dataclass
class InventoryRow:
    id: str
    name: str
    level: str
    folder: str
    source_paths: str
    artifact_ids: list[str]
    doc_status: str
    evidence: str


def load_inventory(path: Path, report: Report) -> tuple[dict[str, InventoryRow], dict[str, str], str]:
    """Return rows by ID, artifact_id->file path map, repo_root."""
    text = read_text(path)
    meta, _, _ = split_frontmatter(text)
    repo = str(meta.get("repo_root", "")).strip()

    rows_table = table_after_heading(text, "## Inventory table")
    if not rows_table:
        report.error("COMPONENT_INVENTORY.md: missing or empty ## Inventory table")
        return {}, {}, repo

    rows: dict[str, InventoryRow] = {}
    for r in rows_table:
        cid = r.get("ID", "").strip()
        if not cid or cid == "ID":
            continue
        row = InventoryRow(
            id=cid,
            name=r.get("Name", ""),
            level=r.get("Level", "").strip().lower(),
            folder=r.get("Folder", "").strip().rstrip("/"),
            source_paths=r.get("Source paths", r.get("Source paths", "")),
            artifact_ids=parse_artifact_ids(r.get("Artifact IDs", "")),
            doc_status=r.get("Doc status", "").strip().lower(),
            evidence=r.get("Evidence", "").strip(),
        )
        if cid in rows:
            report.error(f"COMPONENT_INVENTORY.md: duplicate ID {cid}")
        rows[cid] = row

    artifact_map: dict[str, str] = {}
    map_table = table_after_heading(text, "### Artifact ID map")
    if map_table:
        for r in map_table:
            aid = r.get("ID", "").strip()
            fpath = r.get("File", "").strip()
            if aid and fpath:
                artifact_map[aid] = fpath

    # Also collect IDs from inventory rows
    for row in rows.values():
        for aid in row.artifact_ids:
            if aid not in artifact_map:
                artifact_map.setdefault(aid, "")

    return rows, artifact_map, repo


def load_artifact_registry(artifacts_readme: Path, report: Report) -> dict[str, str]:
    """Parse artifacts/README.md ID registry table if present."""
    if not artifacts_readme.is_file():
        report.error("Missing artifacts/README.md")
        return {}
    text = read_text(artifacts_readme)
    registry: dict[str, str] = {}
    for table in parse_md_tables(text):
        if "ID" in (table[0] if table else {}):
            for r in table:
                aid = r.get("ID", "").strip()
                fpath = r.get("File", r.get("Artifact", "")).strip()
                if aid and aid != "ID":
                    # strip markdown link to path
                    m = re.search(r"\]\(([^)]+)\)", fpath)
                    if m:
                        fpath = m.group(1)
                    registry[aid] = fpath
    return registry


def load_index_ids(index_path: Path) -> set[str]:
    if not index_path.is_file():
        return set()
    text = read_text(index_path)
    return set(re.findall(r"\b([LP]{1,2}\d{2})\b", text))


# ---------------------------------------------------------------------------
# Exemptions
# ---------------------------------------------------------------------------

META_EXEMPT_FRONTMATTER = {
    "INDEX.md",
    "CREATE_LIBRARYDOCS.md",
    "VALIDATION.md",
    "README.md",  # root only
    "COMPONENT_INVENTORY.md",
    "COMPONENT_DEPENDENCIES.md",
    "ANALYSIS_FINDINGS.md",
    "OPEN_QUESTIONS.md",
}

ARTIFACT_CODE_SUFFIXES = {".hpp", ".h", ".cpp", ".c", ".py", ".mk", ".ts", ".js"}
ARTIFACT_DATA_SUFFIXES = {".txt", ".http", ".json", ".yaml", ".yml", ".md"}
MAX_ARTIFACT_LINES = 120


def is_api_reference(rel: Path) -> bool:
    return rel.name == "api-reference.md"


def is_recipe(rel: Path) -> bool:
    return "recipes" in rel.parts


def is_meta_exempt(rel: Path) -> bool:
    name = rel.name
    if name in META_EXEMPT_FRONTMATTER:
        return True
    if rel.parts == ("libraries", "README.md") or rel.parts == ("project", "README.md"):
        return True
    if rel.parts == ("platform", "README.md"):
        return True
    if rel.parts[0] == "artifacts" and name == "README.md":
        return True
    if "architecture" in rel.parts and name == "README.md":
        return True
    if "recipes" in rel.parts and name == "README.md":
        return True
    if "build" in rel.parts and name == "README.md":
        return True
    if "subsystems" in rel.parts and name == "README.md" and rel.parent.name == "subsystems":
        return True
    if "web-configuration" in rel.parts and name == "README.md" and rel.parent.name == "web-configuration":
        return True
    return False


# ---------------------------------------------------------------------------
# Checks
# ---------------------------------------------------------------------------

def check_structure(libdocs: Path, report: Report) -> None:
    for name in ("README.md", "INDEX.md", "VALIDATION.md"):
        if not (libdocs / name).is_file():
            report.error(f"Missing LibraryDocs/{name}")
    for name in ("libraries", "project", "platform", "artifacts"):
        if not (libdocs / name).is_dir():
            report.error(f"Missing LibraryDocs/{name}/")
    if not (libdocs / "project" / "COMPONENT_INVENTORY.md").is_file():
        report.error("Missing LibraryDocs/project/COMPONENT_INVENTORY.md")


def check_inventory_docs(
    libdocs: Path,
    inventory: dict[str, InventoryRow],
    report: Report,
) -> dict[str, Path]:
    """Ensure bidirectional mapping inventory ID <-> doc folder."""
    doc_folders: dict[str, Path] = {}

    def doc_path_for_folder(folder: str) -> Path | None:
        # folder is relative to LibraryDocs e.g. libraries/example-client
        candidate = libdocs / folder / "README.md"
        if candidate.is_file():
            return candidate
        # platform/build is a directory of docs without single README requirement
        if (libdocs / folder).is_dir() and folder.startswith("platform/"):
            readme = libdocs / folder / "README.md"
            if readme.is_file():
                return readme
            # accept any .md in folder for PL platform rows
            mds = list((libdocs / folder).glob("*.md"))
            return mds[0] if mds else None
        if folder == "project/architecture":
            return libdocs / "project" / "architecture" / "system-overview.md"
        return None

    for cid, row in inventory.items():
        if not row.folder:
            report.error(f"Inventory {cid}: missing Folder")
            continue
        dp = doc_path_for_folder(row.folder)
        if dp is None or not dp.is_file():
            report.error(f"Inventory {cid}: no documentation at LibraryDocs/{row.folder}/")
        else:
            doc_folders[cid] = dp

    # Orphan doc folders (libraries and subsystems)
    inv_folder_set = {row.folder for row in inventory.values()}
    for readme in libdocs.rglob("README.md"):
        rel = readme.relative_to(libdocs)
        if is_meta_exempt(rel):
            continue
        parts = rel.parts
        folder_key: str | None = None
        if parts[0] == "libraries" and len(parts) >= 2:
            folder_key = f"libraries/{parts[1]}"
        elif parts[0] == "project" and len(parts) >= 3 and parts[1] == "subsystems":
            if parts[2] == "web-configuration" and len(parts) >= 4:
                folder_key = f"project/subsystems/web-configuration/{parts[3]}"
            elif parts[2] != "README.md":
                folder_key = f"project/subsystems/{parts[2]}"
        if folder_key and folder_key not in inv_folder_set:
            report.warn(f"Orphan documentation folder not in inventory: {folder_key}")

    return doc_folders


def check_source_paths(root: Path, inventory: dict[str, InventoryRow], repo_sub: str, report: Report) -> None:
    for cid, row in inventory.items():
        paths_raw = row.source_paths
        if not paths_raw or paths_raw.lower() in {"n/a", "—", "-"}:
            continue
        base = root / repo_sub if repo_sub else root
        for part in re.split(r",\s*", paths_raw):
            part = part.strip()
            if not part:
                continue
            if "*" in part:
                matches = list(base.glob(part))
                if not matches:
                    report.error(f"Inventory {cid}: source_paths glob matches nothing: {part}")
                continue
            full = base / part
            if not full.is_file():
                # try under root directly
                if not (root / part).is_file():
                    report.error(f"Inventory {cid}: source_paths missing on disk: {part}")


def check_artifacts(
    libdocs: Path,
    inventory: dict[str, InventoryRow],
    artifact_map: dict[str, str],
    registry: dict[str, str],
    doc_folders: dict[str, Path],
    report: Report,
) -> dict[str, str]:
    """Returns artifact_id -> relative file path under artifacts/."""
    merged: dict[str, str] = {}
    merged.update({k: v for k, v in artifact_map.items() if v})
    merged.update(registry)

    all_inv_artifact_ids: set[str] = set()
    for row in inventory.values():
        all_inv_artifact_ids.update(row.artifact_ids)

    # Inventory artifact IDs must be registered with a file path
    for aid in sorted(all_inv_artifact_ids):
        if aid not in merged or not merged[aid]:
            report.error(f"Inventory artifact ID {aid} not registered in Artifact ID map or artifacts/README.md")
        else:
            rel = merged[aid].replace("\\", "/")
            if rel.startswith("artifacts/"):
                rel = rel[len("artifacts/") :]
            full = libdocs / "artifacts" / rel
            if not full.is_file():
                report.error(f"Artifact {aid} registered as {rel} but file missing on disk")

    # On-disk code/data artifacts (exclude bench README index files from registration requirement)
    disk_files: set[str] = set()
    for p in (libdocs / "artifacts").rglob("*"):
        if not p.is_file():
            continue
        rel = p.relative_to(libdocs / "artifacts").as_posix()
        if rel == "README.md" or rel.startswith("bench/"):
            continue
        disk_files.add(rel)

    registered_paths = {v.replace("\\", "/").removeprefix("artifacts/") for v in merged.values() if v}

    for aid, fpath in merged.items():
        fp = fpath.replace("\\", "/").removeprefix("artifacts/")
        if fp and fp not in disk_files:
            report.error(f"Registry artifact {aid} points to missing file artifacts/{fp}")

    for rel in sorted(disk_files):
        if rel.endswith(".md") and rel != "README.md":
            continue
        if rel not in registered_paths:
            report.warn(f"On-disk artifact not registered: artifacts/{rel}")

    # EXCERPT, EVIDENCE, line limit for code-like artifacts
    for rel in disk_files:
        if Path(rel).suffix.lower() not in ARTIFACT_CODE_SUFFIXES:
            continue
        full = libdocs / "artifacts" / rel
        text = read_text(full)
        lines = text.splitlines()
        if len(lines) > MAX_ARTIFACT_LINES:
            report.error(f"Artifact exceeds {MAX_ARTIFACT_LINES} lines ({len(lines)}): artifacts/{rel}")
        head = "\n".join(lines[:5])
        if "EXCERPT" not in head and "EXCERPT" not in text[:300]:
            report.error(f"Artifact missing EXCERPT header: artifacts/{rel}")
        if "EVIDENCE:" not in text[:400]:
            report.error(f"Artifact missing EVIDENCE: line: artifacts/{rel}")

    # Linkage: each registered artifact linked from some component doc
    libdocs_text_cache: dict[Path, str] = {}

    def doc_text(p: Path) -> str:
        if p not in libdocs_text_cache:
            libdocs_text_cache[p] = read_text(p)
        return libdocs_text_cache[p]

    # Gather all markdown under component folders
    search_docs: list[Path] = list(doc_folders.values())
    for cid, row in inventory.items():
        folder = libdocs / row.folder
        if folder.is_dir():
            search_docs.extend(folder.rglob("*.md"))

    # Inventory-required artifact IDs
    required_artifact_ids: set[str] = set(all_inv_artifact_ids)

    for aid, fpath in merged.items():
        fp = fpath.replace("\\", "/").removeprefix("artifacts/")
        if not fp or fp not in disk_files:
            continue
        if aid not in required_artifact_ids:
            continue  # supplementary registry entries optional for linkage
        needle_paths = {fp, f"artifacts/{fp}", aid}
        linked = False
        for doc in set(search_docs):
            if not doc.is_file():
                continue
            body = doc_text(doc)
            if any(n in body for n in needle_paths):
                linked = True
                break
        if not linked:
            report.error(f"Artifact {aid} (artifacts/{fp}) not linked from any component document")

    return merged


def check_frontmatter_all(libdocs: Path, inventory: dict[str, InventoryRow], report: Report) -> None:
    for md in sorted(libdocs.rglob("*.md")):
        rel = md.relative_to(libdocs)
        if is_meta_exempt(rel):
            continue
        text = read_text(md)
        meta, body, has_fm = split_frontmatter(text)
        if not has_fm:
            report.error(f"Missing frontmatter: LibraryDocs/{rel.as_posix()}")
            continue

        recipe = is_recipe(rel)
        api_ref = is_api_reference(rel)

        for key in ("title", "component", "level", "status"):
            if key not in meta:
                report.error(f"LibraryDocs/{rel.as_posix()}: frontmatter missing '{key}'")

        if not api_ref:
            if "topics" not in meta:
                report.error(f"LibraryDocs/{rel.as_posix()}: frontmatter missing 'topics'")
            topics = meta.get("topics", [])
            if isinstance(topics, str):
                topics = [topics]
            if len(topics) < 3:
                report.error(f"LibraryDocs/{rel.as_posix()}: topics must have >=3 entries (has {len(topics)})")

        level = str(meta.get("level", "")).lower()

        if level == "library" and not api_ref:
            if "reuse" not in meta and str(meta.get("artifacts_required", "true")).lower() != "false":
                report.warn(f"LibraryDocs/{rel.as_posix()}: library doc missing 'reuse' in frontmatter")
            if rel.name == "README.md" and str(rel).startswith("libraries/"):
                retrieval = meta.get("retrieval", {})
                questions: list[str] = []
                if isinstance(retrieval, dict):
                    q = retrieval.get("questions", [])
                    if isinstance(q, list):
                        questions = q
                if len(questions) < 3:
                    body_q = re.findall(r"^\s*-\s+.+\?\s*$", body, re.MULTILINE)
                    questions = list(questions) + body_q
                if len(questions) < 3:
                    report.error(
                        f"LibraryDocs/{rel.as_posix()}: library requires >=3 retrieval.questions (has {len(questions)})"
                    )

        status = str(meta.get("status", "")).lower()
        if status == "verified" and not recipe and not api_ref:
            if "## Source evidence" not in body and "Source evidence" not in text:
                report.error(f"LibraryDocs/{rel.as_posix()}: status verified but no Source evidence section")
            else:
                ev_section = body[body.find("Source evidence") :] if "Source evidence" in body else ""
                if not re.search(r"\bE[12]\b", ev_section):
                    report.error(
                        f"LibraryDocs/{rel.as_posix()}: status verified but Source evidence lacks E1 or E2 row"
                    )


def check_open_questions(libdocs: Path, inventory: dict[str, InventoryRow], report: Report) -> None:
    oq = libdocs / "project" / "OPEN_QUESTIONS.md"
    oq_text = read_text(oq) if oq.is_file() else ""
    for cid, row in inventory.items():
        if "E4" in row.evidence.upper().replace(" ", ""):
            if cid not in oq_text and row.name not in oq_text:
                report.error(f"Inventory {cid} has E4 evidence but no entry in OPEN_QUESTIONS.md")


def check_coupling_register(libdocs: Path, inventory: dict[str, InventoryRow], report: Report) -> None:
    inv_path = libdocs / "project" / "COMPONENT_INVENTORY.md"
    text = read_text(inv_path)
    table = table_after_heading(text, "## Coupling register")
    if not table:
        report.error("COMPONENT_INVENTORY.md: missing ## Coupling register")
        return
    mentioned: set[str] = set()
    for r in table:
        mentioned.add(r.get("From ID", "").strip())
        mentioned.add(r.get("To ID", "").strip())
    project_ids = {cid for cid, row in inventory.items() if re.match(r"^P\d+$", cid)}
    for pid in sorted(project_ids):
        if pid not in mentioned:
            report.error(f"Coupling register missing project subsystem {pid}")


def check_index_ids(inventory: dict[str, InventoryRow], index_ids: set[str], report: Report) -> None:
    inv_ids = set(inventory.keys())
    missing_in_index = inv_ids - index_ids
    extra_in_index = index_ids - inv_ids
    if missing_in_index:
        report.error(f"INDEX.md missing inventory IDs: {', '.join(sorted(missing_in_index))}")
    if extra_in_index:
        report.warn(f"INDEX.md IDs not in inventory: {', '.join(sorted(extra_in_index))}")


def check_all_markdown_links(libdocs: Path, report: Report) -> None:
    link_re = re.compile(r"\]\(([^)]+)\)")
    repo_root_path = libdocs.parent.resolve()
    for md in libdocs.rglob("*.md"):
        text = read_text(md)
        rel_md = md.relative_to(libdocs)
        for m in link_re.finditer(text):
            target = normalize_md_link_target(m.group(1))
            if not target or target.startswith(("http://", "https://", "mailto:")):
                continue
            # Links to repo-root paths (.cursor, ReadMe.md)
            if target.startswith("../") and not target.startswith("../project") and not target.startswith("../platform"):
                if ".cursor" in target or target.count("/") <= 2:
                    candidate = (repo_root_path / target.replace("../", "", 1)).resolve()
                    if candidate.exists():
                        continue
            resolved = (md.parent / target).resolve()
            try:
                resolved.relative_to(libdocs.resolve())
            except ValueError:
                candidate = (repo_root_path / target.lstrip("./")).resolve()
                if candidate.exists():
                    continue
                report.error(f"Broken link in {rel_md.as_posix()}: {target}")
                continue
            if not resolved.exists():
                report.error(f"Broken link in {rel_md.as_posix()}: {target}")


def check_validation_md(libdocs: Path, strict: bool, report: Report) -> None:
    path = libdocs / "VALIDATION.md"
    if not path.is_file():
        report.error("Missing LibraryDocs/VALIDATION.md")
        return
    text = read_text(path)
    meta, _, has_fm = split_frontmatter(text)
    if strict:
        if not has_fm or str(meta.get("result", "")).lower() != "pass":
            report.error("VALIDATION.md must have frontmatter result: pass (strict completion)")


def check_library_artifact_exemption(libdocs: Path, inventory: dict[str, InventoryRow], report: Report) -> None:
    for cid, row in inventory.items():
        if row.level != "library":
            continue
        if not row.artifact_ids:
            readme = libdocs / row.folder / "README.md"
            if readme.is_file():
                meta, _, has_fm = split_frontmatter(read_text(readme))
                if has_fm and str(meta.get("artifacts_required", "true")).lower() == "false":
                    if not meta.get("artifact_exemption"):
                        report.warn(f"{cid}: artifacts_required false but artifact_exemption reason missing")
                    continue
            report.error(f"Library {cid}: no artifact IDs and no artifacts_required:false exemption")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def run(root: Path, strict: bool) -> int:
    libdocs = libdocs_root(root)
    report = Report()

    if not libdocs.is_dir():
        report.error("LibraryDocs/ directory missing")
        _print_report(report, strict, root)
        return 1

    check_structure(libdocs, report)

    inv_path = libdocs / "project" / "COMPONENT_INVENTORY.md"
    inventory, artifact_map, repo_sub = load_inventory(inv_path, report) if inv_path.is_file() else ({}, {}, "")

    registry = load_artifact_registry(libdocs / "artifacts" / "README.md", report)

    doc_folders = check_inventory_docs(libdocs, inventory, report) if inventory else {}

    if inventory:
        check_source_paths(root, inventory, repo_sub, report)
        check_coupling_register(libdocs, inventory, report)
        check_open_questions(libdocs, inventory, report)
        check_library_artifact_exemption(libdocs, inventory, report)
        index_ids = load_index_ids(libdocs / "INDEX.md")
        check_index_ids(inventory, index_ids, report)

    if inventory:
        check_artifacts(libdocs, inventory, artifact_map, registry, doc_folders, report)

    check_frontmatter_all(libdocs, inventory, report)
    check_all_markdown_links(libdocs, report)
    check_validation_md(libdocs, strict, report)

    return _print_report(report, strict, root)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Validate a repository's LibraryDocs package.")
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=Path.cwd(),
        help="Repository root. Defaults to the current working directory.",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Treat standards-related warnings as failures.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = args.repo_root.resolve()
    return run(root, strict=args.strict)


def _print_report(report: Report, strict: bool, root: Path) -> int:
    print("LibraryDocs validation")
    print("=" * 40)
    print(f"Repository: {root}")
    print(f"Mode: {'strict' if strict else 'default'}")
    print()

    if report.errors:
        print(f"ERRORS ({len(report.errors)}):")
        for e in report.errors:
            print(f"  [FAIL] {e}")
        print()

    if report.warnings:
        print(f"WARNINGS ({len(report.warnings)}):")
        for w in report.warnings:
            print(f"  [WARN] {w}")
        print()

    if strict and report.warnings:
        print("Strict mode: warnings treated as failures.")
        print("Result: FAIL")
        return 1

    if report.errors:
        print("Result: FAIL")
        return 1

    if not report.errors and not report.warnings:
        print("All checks passed.")
    else:
        print("Result: PASS with warnings (use --strict for definition-of-done)")

    return 0


if __name__ == "__main__":
    sys.exit(main())
