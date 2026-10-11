#!/usr/bin/env python3
"""Create an online SQLite checkpoint; never modify or stop the source demo."""
import argparse, os, pathlib, shutil, sqlite3
p=argparse.ArgumentParser()
p.add_argument('--source',required=True);p.add_argument('--output',required=True)
a=p.parse_args();source=pathlib.Path(a.source).resolve();dest=pathlib.Path(a.output).resolve()
os.umask(0o077)
if dest.exists(): raise SystemExit('Output already exists; choose a new checkpoint directory')
with sqlite3.connect(f'file:{source / "metadata.sqlite"}?mode=ro',uri=True) as db:
    pending=db.execute("SELECT COUNT(*) FROM operations WHERE status NOT IN ('completed','cancelled','failed')").fetchone()[0]
    if pending: raise SystemExit('Finish or cancel pending operations before checkpointing')
    dest.mkdir(mode=0o700,parents=True)
    for entry in source.iterdir():
        if entry.name in {'metadata.sqlite','metadata.sqlite-wal','metadata.sqlite-shm','process.lock'}: continue
        if entry.is_symlink(): raise SystemExit('Symlinked state is not supported')
        if entry.is_dir(): shutil.copytree(entry,dest/entry.name)
        else: shutil.copy2(entry,dest/entry.name)
    with sqlite3.connect(dest/'metadata.sqlite') as snapshot:
        db.backup(snapshot)
        if snapshot.execute('PRAGMA integrity_check').fetchone()[0]!='ok': raise SystemExit('Checkpoint integrity failure')
        count=snapshot.execute('SELECT COUNT(*) FROM versions').fetchone()[0]
print(f'Metadata checkpoint complete: {count} versions. Object verification required before cutover.')
