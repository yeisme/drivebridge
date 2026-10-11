"""Import the official Base Nova registry through the host's HTTP transport."""
import json
import pathlib
import re
import subprocess

root = pathlib.Path(__file__).resolve().parent.parent
names = 'button input textarea badge select dialog alert-dialog dropdown-menu popover tabs tooltip card table pagination skeleton progress sheet checkbox radio-group switch combobox command separator input-group'.split()
for name in names:
    raw = subprocess.check_output(['curl', '-fLsS', '--retry', '2', '--max-time', '30', f'https://ui.shadcn.com/r/styles/base-nova/{name}.json'])
    item = json.loads(raw)
    for entry in item['files']:
        if entry.get('type') != 'registry:ui':
            continue
        content = entry['content'].replace('from "cn"', 'from "@/lib/utils"')
        content = content.replace('@/registry/base-nova/ui/', '@/components/ui/')
        content = re.sub(r'import \{ IconPlaceholder \} from [^\n]+\n', '', content)
        icons = set()
        def icon(match):
            body = match.group(1)
            symbol = re.search(r'lucide="([^"]+)"', body).group(1)
            icons.add(symbol)
            body = re.sub(r'\s*(lucide|tabler|hugeicons|phosphor|remixicon)="[^"]*"', '', body)
            return '<' + symbol + body + '/>'
        content = re.sub(r'<IconPlaceholder([\s\S]*?)/>', icon, content)
        if icons:
            content = 'import { ' + ', '.join(sorted(icons)) + ' } from "lucide-react"\n' + content
        target = root / 'src/components/ui' / pathlib.Path(entry['path']).name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)
    print('Imported', name)
