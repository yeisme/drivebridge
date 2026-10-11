import { InputGroup, InputGroupAddon, InputGroupInput } from "./components/ui/input-group";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import React, { useEffect, useMemo, useRef, useState } from "react";
import { Search, LocateFixed } from "lucide-react";
import { bytes, type DriveFile } from "./api";
import { MediaThumbnail, mediaKind } from "./Media";

export function PreviewFiles({ files, selected, onSelect }: {
  files: DriveFile[]; selected: DriveFile; onSelect: (file: DriveFile) => void;
}) {
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState("all");
  const [page, setPage] = useState(0);
  const list = useRef<HTMLElement>(null);
  const filtered = useMemo(() => files.filter(file =>
    (!query || file.path.toLocaleLowerCase().includes(query.toLocaleLowerCase())) &&
    (kind === "all" || mediaKind(file) === kind)), [files, query, kind]);
  const pageCount = Math.max(1, Math.ceil(filtered.length / 40));
  const shownPage = Math.min(page, pageCount - 1);
  useEffect(() => {
    const index = filtered.findIndex(file => file.id === selected.id);
    if (index >= 0) setPage(Math.floor(index / 40));
  }, [selected.id, filtered]);
  useEffect(() => {
    list.current?.querySelector('[aria-current="true"]')?.scrollIntoView({ block: "nearest" });
  }, [selected.id, shownPage, query, kind]);
  return <section className="preview-file-picker flex flex-col gap-3" aria-label="预览文件导航">
    <header><strong>当前文件 <small>{files.length}</small></strong><Button variant="ghost" size="icon" type="submit" className="icon" aria-label="定位当前文件" title="定位当前文件" onClick={() => { setQuery(""); setKind("all"); setPage(Math.max(0, Math.floor(files.findIndex(f => f.id === selected.id) / 40))); list.current?.querySelector('[aria-current="true"]')?.scrollIntoView({ block: "nearest" }); }}><LocateFixed size={18} /></Button></header>
    {files.length > 1 && <><InputGroup className="h-8"><InputGroupAddon><Search size={16}/></InputGroupAddon><InputGroupInput aria-label="搜索预览文件" placeholder="搜索名称或路径" value={query} onChange={e => { setQuery(e.target.value); setPage(0); }}/></InputGroup>
    <SelectAdapter aria-label="预览文件类型" value={kind} onChange={e => { setKind(e.target.value); setPage(0); }}>
      <option value="all">全部类型</option><option value="image">图片</option><option value="video">视频</option><option value="html">网页</option><option value="pdf">PDF</option><option value="text">文本</option><option value="audio">音频</option>
    </SelectAdapter></>}
    <nav ref={list} className="quicklook-files" aria-label="当前文件列表">
      {filtered.slice(shownPage * 40, (shownPage + 1) * 40).map(file => <Button layout="surface" variant="ghost" type="submit" key={file.id} aria-current={file.id === selected.id ? "true" : undefined} title={file.path} onClick={() => onSelect(file)}><MediaThumbnail file={file}/><span>{file.path.split("/").pop()}<small>{bytes(file.size_bytes)}</small></span></Button>)}
    </nav>
    {!filtered.length && <p role="status">没有匹配的文件</p>}
    {pageCount > 1 && <div className="preview-file-pages"><Button variant="outline" type="submit"  disabled={!shownPage} onClick={() => setPage(shownPage - 1)}>上一组</Button><span>{shownPage + 1} / {pageCount}</span><Button variant="outline" type="submit"  disabled={shownPage + 1 >= pageCount} onClick={() => setPage(shownPage + 1)}>下一组</Button></div>}
    {files.length > 1 && <small className="preview-scope">仅浏览本次打开的 {files.length} 个文件</small>}
  </section>;
}
