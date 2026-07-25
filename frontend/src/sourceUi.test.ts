import { describe, expect, it } from "vitest";
import {
  formatLastIndexed,
  mapSourceStatus,
  sourceDisplayName,
  sourceSecondaryLine,
  sourceVersion,
  typeLabel,
} from "./sourceUi";
import type { SourceSummary } from "./api";

function src(partial: Partial<SourceSummary> & Pick<SourceSummary, "kind" | "id">): SourceSummary {
  return {
    rootName: "Root",
    documentCount: 0,
    chunkCount: 0,
    ...partial,
  };
}

describe("sourceDisplayName", () => {
  it("prefers friendly repo titles over remote URLs", () => {
    expect(
      sourceDisplayName(
        src({
          kind: "repo",
          id: "my-repo",
          title: "https://github.com/org/my-repo.git",
          detail: { remoteUrl: "https://github.com/org/my-repo.git" },
        }),
      ),
    ).toBe("my-repo");
  });

  it("uses web id when distinct from title", () => {
    expect(
      sourceDisplayName(
        src({ kind: "web", id: "docs-site", title: "https://example.com/docs" }),
      ),
    ).toBe("docs-site");
  });
});

describe("mapSourceStatus", () => {
  it("maps ok to Ready", () => {
    expect(mapSourceStatus(src({ kind: "local", id: "a", lastStatus: "ok" })).label).toBe("Ready");
  });

  it("maps idle with no docs to Never indexed", () => {
    expect(mapSourceStatus(src({ kind: "local", id: "a", lastStatus: "idle" })).ui).toBe("never");
  });

  it("maps empty status with documents to Ready", () => {
    expect(
      mapSourceStatus(src({ kind: "local", id: "a", documentCount: 99 })).label,
    ).toBe("Ready");
  });

  it("maps failed* to Failed", () => {
    expect(mapSourceStatus(src({ kind: "web", id: "a", lastStatus: "failed:timeout" })).ui).toBe("failed");
  });
});

describe("sourceVersion / secondary", () => {
  it("shortens repo commit", () => {
    expect(
      sourceVersion(
        src({
          kind: "repo",
          id: "r",
          detail: { resolvedCommit: "abcdef0123456789" },
        }),
      ),
    ).toBe("abcdef01");
  });

  it("returns web start URL as secondary", () => {
    expect(
      sourceSecondaryLine(
        src({
          kind: "web",
          id: "w",
          detail: { startUrl: "https://example.com/path" },
        }),
      ),
    ).toBe("https://example.com/path");
  });
});

describe("typeLabel", () => {
  it("uppercases known kinds", () => {
    expect(typeLabel("pdf")).toBe("PDF");
    expect(typeLabel("local")).toBe("LOCAL");
  });
});

describe("local placeholders", () => {
  it("labels local version and last-indexed instead of dashes", () => {
    const local = src({ kind: "local", id: "Docs", documentCount: 10 });
    expect(sourceVersion(local)).toBe("Local root");
    expect(formatLastIndexed(local).text).toBe("On ingest");
    expect(formatLastIndexed(local).title).toMatch(/do not track refresh/i);
  });
});
