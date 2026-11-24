export type TargetImageFormat = "png" | "jpeg" | "webp";
export type ImageScalePreset = "100" | "75" | "50" | "max1920" | "max1280";

export interface ImageConvertOptions {
  format: TargetImageFormat;
  quality?: number;
  scale?: ImageScalePreset;
}

export const IMAGE_FORMAT_META: Record<
  TargetImageFormat,
  { ext: string; mime: string; label: string; description: string }
> = {
  png: {
    ext: "png",
    mime: "image/png",
    label: "PNG（无损高清）",
    description: "保留完整细节与透明通道，适合截图与设计素材",
  },
  jpeg: {
    ext: "jpg",
    mime: "image/jpeg",
    label: "JPG（通用兼容）",
    description: "体积适中、全平台直接打开，透明底自动填白",
  },
  webp: {
    ext: "webp",
    mime: "image/webp",
    label: "WebP（高压缩比）",
    description: "更小体积同时保持高画质与透明通道",
  },
};

export const IMAGE_SCALE_OPTIONS: { value: ImageScalePreset; label: string }[] = [
  { value: "100", label: "原尺寸 (100%)" },
  { value: "75", label: "缩放至 75%" },
  { value: "50", label: "缩放至 50%" },
  { value: "max1920", label: "长边限制 1920 px" },
  { value: "max1280", label: "长边限制 1280 px" },
];

export function computeTargetDimensions(
  width: number,
  height: number,
  scale: ImageScalePreset = "100",
): { width: number; height: number } {
  const w = Math.max(1, Math.round(width));
  const h = Math.max(1, Math.round(height));
  if (scale === "75") {
    return { width: Math.max(1, Math.round(w * 0.75)), height: Math.max(1, Math.round(h * 0.75)) };
  }
  if (scale === "50") {
    return { width: Math.max(1, Math.round(w * 0.5)), height: Math.max(1, Math.round(h * 0.5)) };
  }
  if (scale === "max1920" || scale === "max1280") {
    const limit = scale === "max1920" ? 1920 : 1280;
    const longest = Math.max(w, h);
    if (longest <= limit) return { width: w, height: h };
    const ratio = limit / longest;
    return {
      width: Math.max(1, Math.round(w * ratio)),
      height: Math.max(1, Math.round(h * ratio)),
    };
  }
  return { width: w, height: h };
}

export function buildConvertedFileName(
  originalPath: string,
  format: TargetImageFormat,
  existingPaths: readonly string[] = [],
): string {
  const base = originalPath.split("/").pop() || "image";
  const dir = originalPath.includes("/") ? originalPath.slice(0, originalPath.lastIndexOf("/")) : "";
  const dot = base.lastIndexOf(".");
  const stem = dot > 0 ? base.slice(0, dot) : base;
  const oldExt = dot > 0 ? base.slice(dot + 1).toLowerCase() : "";
  const targetExt = IMAGE_FORMAT_META[format].ext;

  let candidateBase =
    oldExt === targetExt || (oldExt === "jpeg" && targetExt === "jpg")
      ? `${stem}-converted.${targetExt}`
      : `${stem}.${targetExt}`;

  const fullPath = (name: string) => (dir ? `${dir}/${name}` : name);
  const existingSet = new Set(existingPaths);
  if (!existingSet.has(fullPath(candidateBase)) && fullPath(candidateBase) !== originalPath) {
    return candidateBase;
  }

  let index = 2;
  const stemForConflict =
    oldExt === targetExt || (oldExt === "jpeg" && targetExt === "jpg")
      ? `${stem}-converted`
      : stem;
  while (existingSet.has(fullPath(`${stemForConflict}-${index}.${targetExt}`))) {
    index++;
  }
  candidateBase = `${stemForConflict}-${index}.${targetExt}`;
  return candidateBase;
}

export async function convertImageElementToFile(
  img: HTMLImageElement,
  originalPath: string,
  options: ImageConvertOptions,
  existingPaths: readonly string[] = [],
): Promise<File> {
  const naturalW = img.naturalWidth || img.width || 800;
  const naturalH = img.naturalHeight || img.height || 600;
  const { width, height } = computeTargetDimensions(naturalW, naturalH, options.scale ?? "100");
  const meta = IMAGE_FORMAT_META[options.format];
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("浏览器当前不支持画布图像编码。");

  if (options.format === "jpeg") {
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, width, height);
  }
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(img, 0, 0, width, height);

  const quality = options.quality ?? (options.format === "jpeg" ? 0.92 : options.format === "webp" ? 0.9 : undefined);
  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(
      (out) => {
        if (out) resolve(out);
        else reject(new Error("图片格式转换失败。"));
      },
      meta.mime,
      quality,
    );
  });

  const fileName = buildConvertedFileName(originalPath, options.format, existingPaths);
  return new File([blob], fileName, { type: meta.mime });
}

export async function convertImageUrlToFile(
  src: string,
  originalPath: string,
  options: ImageConvertOptions,
  existingPaths: readonly string[] = [],
): Promise<File> {
  const img = new Image();
  img.crossOrigin = "anonymous";
  await new Promise<void>((resolve, reject) => {
    img.onload = () => resolve();
    img.onerror = () => reject(new Error("无法加载待转换图片内容。"));
    img.src = src;
  });
  return convertImageElementToFile(img, originalPath, options, existingPaths);
}

export function triggerFileDownload(file: File): void {
  const url = URL.createObjectURL(file);
  const link = document.createElement("a");
  link.href = url;
  link.download = file.name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 4000);
}
