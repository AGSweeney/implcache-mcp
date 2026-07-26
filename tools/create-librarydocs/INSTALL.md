# Install create-librarydocs (global skill)

**Source in this repository:** [`tools/create-librarydocs/`](.)  
**Zip bundle:** [`tools/create-librarydocs.zip`](../create-librarydocs.zip) (same contents; use either)

This tree is git-tracked. Cursor’s project skill path (`.cursor/skills/`) is gitignored in ImplCache, so install the skill **globally** (or into a consumer repo’s `.cursor/skills/`) rather than relying on a project copy here.

**Do not copy the full skill into every consumer repository.** Generated repos normally contain only `LibraryDocs/`. Install once globally:

```powershell
$Src = "D:\path\to\implcache-mcp\tools\create-librarydocs"   # or expand create-librarydocs.zip
$SkillRoot = "$env:USERPROFILE\.cursor\skills\create-librarydocs"
New-Item -ItemType Directory -Force "$SkillRoot\reference", "$SkillRoot\scripts" | Out-Null
Copy-Item -Recurse -Force "$Src\*" $SkillRoot
```

From the zip:

```powershell
Expand-Archive tools\create-librarydocs.zip -DestinationPath $env:TEMP\create-librarydocs-skill -Force
$Src = "$env:TEMP\create-librarydocs-skill\create-librarydocs"
$SkillRoot = "$env:USERPROFILE\.cursor\skills\create-librarydocs"
New-Item -ItemType Directory -Force "$SkillRoot\reference", "$SkillRoot\scripts" | Out-Null
Copy-Item -Recurse -Force "$Src\*" $SkillRoot
```

Confirm under **Cursor Settings → Rules, Skills, Subagents → Skills**.

## CI / pinned validation

Copy only the validator into a consumer repo if needed:

```powershell
Copy-Item tools\create-librarydocs\scripts\validate_librarydocs.py tools\
```

Run from that repository root:

```powershell
python tools\validate_librarydocs.py --repo-root . --strict
```

## Version

Skill version: **2.1.0** (see `SKILL.md` frontmatter)

Operator overview for ImplCache: [docs/CREATE_LIBRARYDOCS.md](../../docs/CREATE_LIBRARYDOCS.md)
