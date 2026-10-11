import { expect, test } from '@playwright/test';
import path from 'node:path';

const destinations = [
  { label: '我的工作台', route: 'home', region: '个人工作台摘要', mobile: '首页' },
  { label: '任务中心', route: 'tasks', region: '持久操作任务', mobile: '任务' },
  { label: 'Agent 管理', route: 'agents', region: '新协议 Agent 授权', mobile: 'Agent' },
  { label: '存储管理', route: 'storage', region: '', mobile: '' },
];

for (const theme of ['light', 'dark'] as const) test(`canonical navigation and shell geometry in six widths with ${theme} theme`, async ({ page }) => {
  test.setTimeout(120_000);
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS;
  expect(artifacts, 'Use the owner evidence runner').toBeTruthy();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
  await page.goto('/');
  await page.locator('#login-key').fill('dbu_' + '2'.repeat(96));
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await expect(page.locator('.app')).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('data-theme', theme);

  for (const width of [320, 390, 768, 1280, 1440, 1920]) {
    await page.setViewportSize({ width, height: width < 768 ? 844 : 1000 });
    for (const destination of destinations) {
      if (width < 768) {
        const trigger = page.getByRole('button', { name: '打开全部导航', exact: true });
        await trigger.click();
        const drawer = page.getByRole('dialog', { name: '全部导航', exact: true });
        const control = drawer.getByRole('button', { name: destination.label, exact: true });
        await expect(control).toBeVisible();
        expect((await control.boundingBox())!.height).toBeGreaterThanOrEqual(44);
        await control.click();
        await expect(drawer).toHaveCount(0);
        await expect(trigger).toBeFocused();
        if (destination.mobile) {
          const navigation = page.getByRole('navigation', { name: '手机主导航' });
          await expect(navigation.locator('[aria-current="page"]')).toHaveCount(1);
          await expect(navigation.getByRole('button', { name: destination.mobile, exact: true })).toHaveAttribute('aria-current', 'page');
        }
      } else {
        const navigation = page.getByRole('navigation', { name: '工作区导航', exact: true });
        const control = navigation.getByRole('button', { name: destination.label, exact: true });
        await control.focus();
        // A dialog close must not schedule a second focus restoration that steals
        // a subsequent keyboard navigation action.
        await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
        await expect(control).toBeFocused();
        await page.keyboard.press('Enter');
        await expect(control).toHaveAttribute('aria-current', 'page');
        await expect(navigation.locator('[aria-current="page"]')).toHaveCount(1);
        expect(Math.round((await page.locator('.sidebar').boundingBox())!.width)).toBe(width < 1280 ? 56 : 184);
      }
      await expect(page).toHaveURL(new RegExp('#/' + destination.route + '$'));
      await expect(page.locator('.breadcrumb')).toContainText(destination.label);
      if (destination.region) await expect(page.getByRole('region', { name: destination.region, exact: true })).toBeVisible();
      else await expect(page.getByRole('button', { name: '连接存储', exact: true })).toBeVisible();
      await page.waitForLoadState('networkidle');
      expect(Math.round((await page.locator('.topbar').boundingBox())!.height)).toBe(48);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${destination.route} ${width}`).toBe(true);
      const controls = await page.locator('.topbar button').evaluateAll(elements => elements.filter(element => {
        const rect = element.getBoundingClientRect();
        return rect.width && rect.height && getComputedStyle(element).visibility !== 'hidden';
      }).map(element => {
        const rect = element.getBoundingClientRect();
        return { label: element.getAttribute('aria-label') || element.textContent, left: rect.left, right: rect.right, width: rect.width, height: rect.height };
      }));
      for (const control of controls) {
        expect(control.left, control.label || '').toBeGreaterThanOrEqual(0);
        expect(control.right, control.label || '').toBeLessThanOrEqual(width);
        if (width < 768) {
          expect(control.height, control.label || '').toBeGreaterThanOrEqual(44);
          expect(control.width, control.label || '').toBeGreaterThanOrEqual(44);
        }
      }
      await page.screenshot({ path: path.join(artifacts!, `shell-${theme}-${width}-${destination.route}.png`), animations: 'disabled' });
      const currentTitle = page.locator('.breadcrumb > span');
      const titleSize = await currentTitle.evaluate(element => ({ scroll: element.scrollWidth, client: element.clientWidth, rect: element.getBoundingClientRect().width }));
      expect(titleSize.scroll <= titleSize.client, `Visible title: ${destination.label} ${width} ${JSON.stringify(titleSize)}`).toBe(true);
    }
    if (width < 768) {
      const navigation = page.getByRole('button', { name: '打开全部导航', exact: true });
      await navigation.click();
      const drawer = page.getByRole('dialog', { name: '全部导航', exact: true });
      const closeBox = (await drawer.getByRole('button', { name: '关闭', exact: true }).boundingBox())!;
      expect(closeBox.width).toBeGreaterThanOrEqual(44);
      expect(closeBox.height).toBeGreaterThanOrEqual(44);
      const toggle = drawer.getByRole('button', { name: theme === 'dark' ? '切换浅色主题' : '切换深色主题', exact: true });
      await toggle.click();
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme === 'dark' ? 'light' : 'dark');
      await drawer.getByRole('button', { name: theme === 'dark' ? '切换深色主题' : '切换浅色主题', exact: true }).click();
      await page.keyboard.press('Escape');
      await expect(navigation).toBeFocused();
    }
    const search = page.getByRole('button', { name: '搜索工作区', exact: true });
    await search.click();
    const searchDialog = page.getByRole('dialog', { name: '搜索工作区', exact: true });
    await expect(searchDialog).toBeVisible();
    const dialogBox = (await searchDialog.boundingBox())!;
    expect(dialogBox.x).toBeGreaterThanOrEqual(0);
    expect(dialogBox.x + dialogBox.width).toBeLessThanOrEqual(width);
    expect(await searchDialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
    const hintContrast = await searchDialog.getByText('查询整个文件路径的字面子串，不查询正文或标签。保存条件只记录查询，不授予文件权限。', { exact: true }).evaluate(element => {
      const luminance = (value: string) => {
        const rgb = value.match(/[\d.]+/g)!.slice(0, 3).map(Number).map(channel => { const s = channel / 255; return s <= .04045 ? s / 12.92 : ((s + .055) / 1.055) ** 2.4; });
        return .2126 * rgb[0] + .7152 * rgb[1] + .0722 * rgb[2];
      };
      const foreground = luminance(getComputedStyle(element).color);
      const background = luminance(getComputedStyle(element.closest('[data-region=query]')!).backgroundColor);
      return (Math.max(foreground, background) + .05) / (Math.min(foreground, background) + .05);
    });
    expect(hintContrast).toBeGreaterThanOrEqual(4.5);
    if (width < 768) {
      const closeBox = (await searchDialog.getByRole('button', { name: '关闭', exact: true }).boundingBox())!;
      expect(closeBox.width).toBeGreaterThanOrEqual(44);
      expect(closeBox.height).toBeGreaterThanOrEqual(44);
    }
    await page.screenshot({ path: path.join(artifacts!, `search-dialog-${theme}-${width}.png`), animations: 'disabled' });
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog', { name: '搜索工作区', exact: true })).toHaveCount(0);
    await expect(search).toBeFocused();
  }
  expect(errors).toEqual([]);
});
