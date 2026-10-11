#!/usr/bin/env python3
"""填表批量开通：一张 CSV 进，一批 key 出。

输入（scripts/users-template.csv 形状，UTF-8）：
    name[,days[,admin]]
每行一个用户。days 空=长期有效；admin 为 true/1/yes 时签管理员。

输出（--out，默认 keys-<时间戳>.csv，0600）：
    name,drivebridge_key,expires_at

密钥只落在输出文件与终端一次，不入库不进日志。

用法（部署 state 经 DRIVEBRIDGE_STATE 或配置文件）：
    DRIVEBRIDGE_STATE=<state目录> python3 scripts/issue_users.py users.csv -o dist.csv
"""

import csv
import datetime
import json
import os
import subprocess
import sys

TEMPLATE_NOTE = "name,days,admin  # days 空=长期；admin=true/1/yes"


def read_rows(path):
    rows = []
    with open(path, newline="", encoding="utf-8-sig") as f:
        for i, row in enumerate(csv.reader(f)):
            if not row or not "".join(row).strip():
                continue
            cells = [c.strip() for c in row]
            if i == 0 and cells[0].lower() == "name":
                continue
            name = cells[0]
            days = ""
            admin = False
            if len(cells) > 1:
                days = cells[1]
            if len(cells) > 2 and cells[2].lower() in ("true", "1", "yes"):
                admin = True
            if not name:
                sys.exit(f"bad row {i + 1}: empty name")
            if days and (not days.isdigit() or int(days) < 0):
                sys.exit(f"bad row {i + 1}: days must be a non-negative integer")
            rows.append({"name": name, "days": days, "admin": admin})
    if not rows:
        sys.exit("no user rows found")
    return rows




def main():
    if len(sys.argv) < 2:
        sys.exit(__doc__)
    src = sys.argv[1]
    out = "keys-" + datetime.datetime.now().strftime("%Y%m%d-%H%M%S") + ".csv"
    if "-o" in sys.argv:
        out = sys.argv[sys.argv.index("-o") + 1]
    rows = read_rows(src)

    entries = [
        {"name": r["name"], **({"days": int(r["days"])} if r["days"] else {}), **({"admin": True} if r["admin"] else {})}
        for r in rows
    ]
    base = ["./drivebridge-server", "--json"]
    if os.environ.get("DRIVEBRIDGE_STATE"):
        base += ["--state", os.environ["DRIVEBRIDGE_STATE"]]
    proc = subprocess.run(
        base + ["users", "import"],
        input=json.dumps(entries),
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        sys.exit("users import failed: " + (proc.stderr.strip() or proc.stdout.strip()))
    issued = json.loads(proc.stdout)["data"]["issued"]

    header = ["name", "drivebridge_key", "expires_at"]
    fd = os.open(out, os.O_CREAT | os.O_WRONLY | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(header)
        for v in issued:
            expires = v.get("expires_at", "")
            if expires.startswith("0001"):
                expires = ""
            else:
                expires = expires[:10]
            w.writerow([v["name"], v["key"], expires])
    os.chmod(out, 0o600)
    print(f"issued {len(issued)} user(s); distribution CSV: {out} (0600, contains plaintext keys)")


if __name__ == "__main__":
    main()
