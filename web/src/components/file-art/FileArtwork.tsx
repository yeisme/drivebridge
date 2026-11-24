import type { FileVisualTone } from "../../lib/file-display";
import "./file-art.css";

function renderSheetInterior(tone: FileVisualTone) {
  switch (tone) {
    case "sheet":
      return (
        <g>
          <rect x="12" y="22" width="32" height="5.5" rx="1.5" fill="currentColor" fillOpacity="0.2" />
          <rect x="12" y="30.5" width="14.5" height="5" rx="1.2" fill="currentColor" fillOpacity="0.13" />
          <rect x="29.5" y="30.5" width="14.5" height="5" rx="1.2" fill="currentColor" fillOpacity="0.13" />
          <rect x="12" y="38.5" width="14.5" height="5" rx="1.2" fill="currentColor" fillOpacity="0.13" />
          <rect x="29.5" y="38.5" width="14.5" height="5" rx="1.2" fill="currentColor" fillOpacity="0.22" />
          <rect x="12" y="46.5" width="32" height="4" rx="1.2" fill="currentColor" fillOpacity="0.1" />
        </g>
      );
    case "pdf":
      return (
        <g>
          <rect x="12" y="21" width="18" height="4.5" rx="1.5" fill="currentColor" fillOpacity="0.78" />
          <rect x="12" y="30" width="32" height="3" rx="1.5" fill="currentColor" fillOpacity="0.18" />
          <rect x="12" y="36.5" width="26" height="3" rx="1.5" fill="currentColor" fillOpacity="0.18" />
          <rect x="12" y="43" width="20" height="3" rx="1.5" fill="currentColor" fillOpacity="0.14" />
        </g>
      );
    case "image":
      return (
        <g>
          <rect x="11" y="21" width="34" height="30" rx="3.5" fill="currentColor" fillOpacity="0.11" />
          <circle cx="20" cy="29.5" r="3" fill="currentColor" fillOpacity="0.65" />
          <path
            d="M13 48l8.5-9.5 6.5 6.5 5-4.5L43 48H13z"
            fill="currentColor"
            fillOpacity="0.48"
          />
        </g>
      );
    case "video":
      return (
        <g>
          <rect x="11" y="21" width="34" height="30" rx="4" fill="currentColor" fillOpacity="0.13" />
          <circle cx="28" cy="36" r="8.5" fill="currentColor" fillOpacity="0.78" />
          <path d="M26 32.2l6.2 3.8-6.2 3.8v-7.6z" fill="#ffffff" />
        </g>
      );
    case "audio":
      return (
        <g stroke="currentColor" strokeWidth="2.6" strokeLinecap="round">
          <line x1="16" y1="39" x2="16" y2="33" strokeOpacity="0.4" />
          <line x1="22" y1="44" x2="22" y2="28" strokeOpacity="0.7" />
          <line x1="28" y1="47" x2="28" y2="24" />
          <line x1="34" y1="42" x2="34" y2="30" strokeOpacity="0.7" />
          <line x1="40" y1="39" x2="40" y2="33" strokeOpacity="0.4" />
        </g>
      );
    case "archive":
      return (
        <g>
          <rect x="24.5" y="16" width="7" height="34" rx="1.8" fill="currentColor" fillOpacity="0.13" />
          <rect x="26" y="19" width="4" height="2.6" rx="0.8" fill="currentColor" fillOpacity="0.6" />
          <rect x="26" y="24.5" width="4" height="2.6" rx="0.8" fill="currentColor" fillOpacity="0.6" />
          <rect x="26" y="30" width="4" height="2.6" rx="0.8" fill="currentColor" fillOpacity="0.6" />
          <rect x="24.5" y="36.5" width="7" height="8" rx="2" fill="currentColor" fillOpacity="0.78" />
        </g>
      );
    case "code":
      return (
        <g>
          <path
            d="M21.5 31.5L16 36.5l5.5 5M34.5 31.5l5.5 5-5.5 5M30 29.5l-4 14"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </g>
      );
    default:
      return (
        <g>
          <rect x="12" y="22" width="18" height="4" rx="2" fill="currentColor" fillOpacity="0.68" />
          <rect x="12" y="30" width="32" height="3" rx="1.5" fill="currentColor" fillOpacity="0.2" />
          <rect x="12" y="36.5" width="32" height="3" rx="1.5" fill="currentColor" fillOpacity="0.2" />
          <rect x="12" y="43" width="22" height="3" rx="1.5" fill="currentColor" fillOpacity="0.14" />
        </g>
      );
  }
}

export function FileSheetSvg({
  tone,
  className = "file-art-svg",
}: {
  tone: FileVisualTone;
  className?: string;
}) {
  return (
    <svg viewBox="0 0 56 68" className={className} aria-hidden="true" focusable="false">
      <path
        className="file-art-paper"
        d="M9 3h26l13 13v44a5 5 0 0 1-5 5H9a5 5 0 0 1-5-5V8a5 5 0 0 1 5-5z"
      />
      <path
        className="file-art-flap"
        d="M35 3l13 13h-8.5A4.5 4.5 0 0 1 35 11.5V3z"
      />
      <path
        d="M35 3v8.5a4.5 4.5 0 0 0 4.5 4.5H48"
        fill="none"
        stroke="currentColor"
        strokeOpacity="0.16"
        strokeWidth="1"
      />
      {renderSheetInterior(tone)}
    </svg>
  );
}

export function FolderGlyph({
  tone = 1,
  size = 20,
  className = "",
}: {
  tone?: number;
  size?: number;
  className?: string;
}) {
  const height = Math.round(size * 0.84);
  return (
    <svg
      width={size}
      height={height}
      viewBox="0 0 32 27"
      className={`folder-dim-glyph folder-glyph ${className}`.trim()}
      data-tone={tone}
      aria-hidden="true"
      focusable="false"
    >
      <path
        className="folder-dim-back"
        d="M2.5 5A2.5 2.5 0 0 1 5 2.5h6.8c1 0 1.7.45 2.25 1.3l1.05 1.7H27A2.5 2.5 0 0 1 29.5 8v14A2.5 2.5 0 0 1 27 24.5H5A2.5 2.5 0 0 1 2.5 22V5z"
      />
      <rect className="folder-dim-front" x="2.5" y="8.5" width="27" height="16" rx="2.5" />
    </svg>
  );
}

export function EmptyDropIllustration({
  tone = 1,
  open = false,
}: {
  tone?: number;
  open?: boolean;
}) {
  return (
    <span className="empty-drop-scene" data-open={open ? "true" : undefined} aria-hidden="true">
      <svg viewBox="0 0 64 52" className="empty-drop-glyph" data-tone={tone} focusable="false">
        <rect
          x="6"
          y="10"
          width="52"
          height="36"
          rx="8"
          fill="var(--ui-muted)"
          stroke="var(--ui-border)"
          strokeWidth="1.25"
        />
        <path
          d="M22 28h20M32 19v18"
          stroke="var(--ui-muted-foreground)"
          strokeWidth="1.75"
          strokeLinecap="round"
        />
      </svg>
    </span>
  );
}
