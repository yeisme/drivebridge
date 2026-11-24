import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { EmptyDropIllustration, FileSheetSvg, FolderGlyph } from "./FileArtwork";

afterEach(cleanup);

describe("FileArtwork components", () => {
  it("renders dog-ear paper sheets across all file visual tones", () => {
    for (const tone of ["doc", "sheet", "pdf", "image", "video", "audio", "archive", "code"] as const) {
      const { container, unmount } = render(<FileSheetSvg tone={tone} />);
      expect(container.querySelector(".file-art-paper")).toBeTruthy();
      expect(container.querySelector(".file-art-flap")).toBeTruthy();
      unmount();
    }
  });

  it("renders restrained folder glyphs and minimal empty drop illustrations", () => {
    const { container } = render(
      <div>
        <FolderGlyph tone={3} size={20} />
        <EmptyDropIllustration tone={2} open />
      </div>,
    );
    expect(container.querySelector('svg.folder-dim-glyph[data-tone="3"]')).toBeTruthy();
    expect(container.querySelector('.empty-drop-scene[data-open="true"]')).toBeTruthy();
  });
});
