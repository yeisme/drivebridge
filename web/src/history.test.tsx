import React from "react";
import {beforeEach,afterEach,expect,it,vi} from "vitest";
import {render,screen,fireEvent,cleanup} from "@testing-library/react";
import {History,filterHistory,isFinished,historyStatusKey,readHistoryStatus} from "./History";
import type {Operation} from "./api";
import { historyScope } from "./app/history-scope";
const ops=[{id:"done",path:"image.png",caller:"agent",status:"completed"},{id:"active",path:"video.mp4",status:"transferring"},{id:"failed",path:"failed.txt",status:"failed"}] as Operation[];
beforeEach(()=>{const values=new Map<string,string>();Object.defineProperty(window,"localStorage",{configurable:true,value:{getItem:(key:string)=>values.get(key)||null,setItem:(key:string,value:string)=>values.set(key,value),clear:()=>values.clear()}});});
afterEach(()=>{cleanup();vi.restoreAllMocks();window.localStorage.clear();});
const rows=(items:Operation[]) => <div>{items.map(o=><span key={o.id}>{o.path}</span>)}</div>;
async function selectStatus(value:string){
 fireEvent.click(screen.getAllByRole('combobox')[0]);
 const option=await screen.findByRole('option',{name:value});fireEvent.pointerDown(option,{pointerType:'mouse'});fireEvent.click(option);
}
it('persists only state enum and reloads it while search and source remain session-only',async()=>{
 const scope=historyScope('instance','alice','tenant','space','transfers');
 const view=render(<History ops={ops} scope={scope}>{rows}</History>);
 await selectStatus('已完成');expect(localStorage.getItem(historyStatusKey(scope))).toBe('completed');expect(screen.queryByText('video.mp4')).toBeNull();
 fireEvent.change(screen.getByRole('searchbox'),{target:{value:'private search'}});view.unmount();
 render(<History ops={ops} scope={scope}>{rows}</History>);expect(screen.getByText('image.png')).toBeTruthy();expect(screen.queryByText('failed.txt')).toBeNull();expect((screen.getByRole('searchbox') as HTMLInputElement).value).toBe('');
});
it('does not carry a saved status into any different identity dimension or page',()=>{
 const identity=['instance','alice','tenant','space'] as const;const scope=historyScope(...identity,'transfers');localStorage.setItem(historyStatusKey(scope),'failed');
 const view=render(<History ops={ops} scope={scope}>{rows}</History>);expect(screen.queryByText('image.png')).toBeNull();expect(screen.getByText('failed.txt')).toBeTruthy();
 for(let index=0;index<4;index++){const changed:[string,string,string,string]=[...identity];changed[index]='other';view.rerender(<History ops={ops} scope={historyScope(...changed,'transfers')}>{rows}</History>);expect(screen.getByText('image.png')).toBeTruthy();}
 view.rerender(<History ops={ops} scope={historyScope(...identity,'inbox')}>{rows}</History>);expect(screen.getByText('image.png')).toBeTruthy();
 view.rerender(<History ops={ops} scope={scope}>{rows}</History>);expect(screen.queryByText('image.png')).toBeNull();
});
it('ignores malformed saved states and never accesses a shared empty-scope key',async()=>{
 localStorage.setItem(historyStatusKey('alice'),'{"token":"bad"}');expect(readHistoryStatus('alice')).toBe('all');
 const get=vi.spyOn(localStorage,'getItem'),set=vi.spyOn(localStorage,'setItem');expect(readHistoryStatus('')).toBe('all');expect(get).not.toHaveBeenCalled();
 render(<History ops={ops} scope="">{rows}</History>);await selectStatus('已完成');expect(screen.queryByText('failed.txt')).toBeNull();expect(set).not.toHaveBeenCalled();
});
it('storage failure retains a session filter without changing operation facts',async()=>{
 vi.spyOn(localStorage,'getItem').mockImplementation(()=>{throw new Error('denied');});vi.spyOn(localStorage,'setItem').mockImplementation(()=>{throw new Error('denied');});
 render(<History ops={ops} scope="alice">{rows}</History>);await selectStatus('已完成');expect(screen.getByText('image.png')).toBeTruthy();expect(screen.queryByText('failed.txt')).toBeNull();expect(ops[2].status).toBe('failed');
});
it("retains active and recoverable work while clearing filtered history",()=>{
 expect(isFinished(ops[2])).toBe(false);
 expect(filterHistory(ops,"image","all","agent",[]).map(o=>o.id)).toEqual(["done"]);
 render(<History ops={ops} scope="user:tenant:transfers">{rows=><div>{rows.map(o=><span key={o.id}>{o.path}</span>)}</div>}</History>);
 fireEvent.click(screen.getByRole("button",{name:"清理历史"}));
 fireEvent.click(screen.getByRole("button",{name:"确认清理"}));
 expect(screen.queryByText("image.png")).toBeNull();
 expect(screen.getByText("video.mp4")).toBeTruthy();expect(screen.getByText("failed.txt")).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"撤销"}));expect(screen.getByText("image.png")).toBeTruthy();
});
it("resets hidden and undo state when the mounted scope changes",()=>{
 const scopeA=historyScope("instance-a","user","tenant","space","inbox");
 const scopeB=historyScope("instance-b","user","tenant","space","inbox");
 const rows=(items:Operation[]) => <div>{items.map(o=><span key={o.id}>{o.path}</span>)}</div>;
 const view=render(<History ops={ops} scope={scopeA}>{rows}</History>);
 fireEvent.click(screen.getByRole("button",{name:"清理历史"}));
 fireEvent.click(screen.getByRole("button",{name:"确认清理"}));
 expect(screen.queryByText("image.png")).toBeNull();
 view.rerender(<History ops={ops} scope={scopeB}>{rows}</History>);
 expect(screen.getByText("image.png")).toBeTruthy();
 expect(screen.queryByRole("button",{name:"撤销"})).toBeNull();
 view.rerender(<History ops={ops} scope={scopeA}>{rows}</History>);
 expect(screen.queryByText("image.png")).toBeNull();
 expect(historyScope("a:b","c","","","inbox")).not.toBe(historyScope("a","b:c","","","inbox"));
 expect(scopeA).not.toBe(historyScope("instance-a","user","tenant","other-space","inbox"));
});

it("keeps incomplete identities in memory without reading or writing a shared preference",()=>{
 expect(historyScope("","user","tenant","space","inbox")).toBe("");
 expect(historyScope("instance","","tenant","space","inbox")).toBe("");
 window.localStorage.setItem("drivebridge:history:",JSON.stringify(["done"]));
 const rows=(items:Operation[]) => <div>{items.map(o=><span key={o.id}>{o.path}</span>)}</div>;
 const view=render(<History ops={ops} scope="">{rows}</History>);
 expect(screen.getByText("image.png")).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"清理历史"}));
 fireEvent.click(screen.getByRole("button",{name:"确认清理"}));
 expect(screen.queryByText("image.png")).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"撤销"}));
 expect(window.localStorage.getItem("drivebridge:history:")).toBe(JSON.stringify(["done"]));
 view.rerender(<History ops={ops} scope={historyScope("instance","user","tenant","space","inbox")}>{rows}</History>);
 expect(screen.getByText("image.png")).toBeTruthy();
});

it("keeps browser history preferences isolated by identity and page",()=>{
 window.localStorage.setItem("drivebridge:history:a:t:inbox",JSON.stringify(["done"]));
 const view=render(<History ops={ops} scope="a:t:inbox">{rows=><div>{rows.map(o=><span key={o.id}>{o.path}</span>)}</div>}</History>);
 expect(screen.queryByText("image.png")).toBeNull();view.unmount();
 render(<History ops={ops} scope="b:t:inbox">{rows=><div>{rows.map(o=><span key={o.id}>{o.path}</span>)}</div>}</History>);
 expect(screen.getByText("image.png")).toBeTruthy();
});
