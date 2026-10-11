import { expect, test, type Page } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';

const routes = ['overview', 'files', 'knowledge', 'resources', 'collections', 'properties', 'search', 'agents', 'transfers', 'inbox', 'connections', 'devices', 'automation', 'capabilities', 'trash', 'settings', 'shares'];

// These fixture states are intentional and page-specific. A mounted container,
// a quiet network or absence of overflow does not prove its data has loaded.
const stableText: Record<string, string> = {
  overview: '当前观察没有可展示的最近文件。',
  knowledge: '当前来源没有可阅读的 Markdown 文件。',
  resources: '当前范围与筛选下没有可见资源描述；原文件和集合不会自动变成已登记资源。',
  collections: '当前授权空间没有集合。',
  properties: '当前空间尚无属性定义。',
  search: '输入检索条件后开始查询，尚未读取搜索结果。',
  agents: '当前视图暂无 Agent',
  transfers: '当前授权范围及筛选下没有操作任务。',
  inbox: '当前范围没有匹配的提交收件。',
  connections: '管理员配置的平台存储（S3）',
  devices: '浏览器报告网络可用',
  automation: '当前空间没有已保存的整理规则。',
  trash: '回收站是空的',
  shares: '还没有分享',
};
const pendingText = /(?:正在(?:读取|加载|检索)|读取[^\n。]*…|加载中|读取中)/;
async function ready(page: Page, route: string) {
  if (stableText[route]) await expect(page.locator('main').getByText(stableText[route], { exact: true })).toBeVisible();
  else if (route === 'files') await expect(page.getByRole('button', { name: /^打开网盘 / }).first()).toBeEnabled();
  else if (route === 'capabilities') await expect(page.getByRole('region', { name: '服务限额', exact: true })).toBeVisible();
  else if (route === 'settings') await expect(page.getByRole('radiogroup', { name: '主题模式', exact: true })).toBeVisible();
  else throw new Error('missing_page_readiness_contract');
  await expect(page.locator('main')).not.toContainText(pendingText);
  await expect(page.locator('main [role="alert"]:visible')).toHaveCount(0);
  return stableText[route] || { files: '我的网盘', capabilities: '当前限额', settings: '浅色和深色共用 DriveBridge 绿色品牌' }[route]!;
}

test('capture the actual personal workspace route coverage without treating fallback as delivery', async ({ page }) => {
  const full = process.env.DRIVEBRIDGE_TEST_VISUAL_FULL === '1';
  test.setTimeout(full ? 240_000 : 120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS;
  expect(artifacts, 'Use the owner evidence runner').toBeTruthy();
  const observations: unknown[] = [];
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await page.getByRole('link', { name: '使用账号登录' }).click();
  await page.getByRole('link', { name: '以 Alice 登录' }).click();
  await expect(page.getByRole('heading', { name: '我的网盘', exact: true })).toBeVisible();
  for (const theme of (full ? ['light', 'dark'] : ['light']) as ('light' | 'dark')[]) {
  await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
  for (const width of full ? [320, 390, 768, 1280, 1440, 1920] : [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
    for (const route of routes) {
      await page.goto('/#/' + (route === 'overview' ? 'home' : route === 'transfers' ? 'tasks' : route));
      await expect(page.locator('main')).toBeVisible();
      // Per-page terminal state is required even when networkidle fires before
      // React's subsequent effect starts its read.
      const expectedState = await ready(page, route);
      const region = { overview: '个人工作台摘要', knowledge: '知识库', resources: '资源库', collections: '项目集合', properties: '标签与属性', agents: '新协议 Agent 授权', inbox: 'Agent 提交收件', transfers: '持久操作任务', devices: '设备与离线', automation: '备份与整理', capabilities: '能力与连接说明' }[route];
      if (region) await expect(page.getByRole('region', { name: region, exact: true })).toBeVisible();
      const state = await page.locator('main').evaluate(main => ({
        headings: Array.from(main.querySelectorAll('h1,h2,h3')).map(element => element.textContent),
        text: (main as HTMLElement).innerText,
        overflow: document.documentElement.scrollWidth > innerWidth + 1,
        controls: Array.from(main.querySelectorAll('button,input,select'))
          .filter(element => element.getBoundingClientRect().width && getComputedStyle(element).visibility !== 'hidden')
          .map(element => ({ role: element.getAttribute('role'), label: element.getAttribute('aria-label'), text: element.textContent })),
      }));
      expect(state.text).not.toMatch(pendingText);
      observations.push({ route, width, theme, ...state, stable: true, expectedState, errors: [...errors] });
      await page.screenshot({ path: path.join(artifacts!, full ? `product-${theme}-${width}-${route}.png` : `product-${width}-${route}.png`), animations: 'disabled' });
      expect(state.overflow, `${route}/${theme}/${width}`).toBe(false);
    }
  }
  }
  await fs.writeFile(path.join(artifacts!, 'workspace-observations.json'), JSON.stringify({ scope: 'local-fixture-route-inspection', businessVerified: false, observations }, null, 2));
  expect(errors).toEqual([]);
});
