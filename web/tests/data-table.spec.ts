import {test,expect} from '@playwright/test';
test('directory table selection, columns, sorting and keyboard focus',async({page})=>{
 await page.goto('/');await page.locator('#login-key').fill('dbu_'+'2'.repeat(96));await page.getByRole('button',{name:'登录',exact:true}).click();
 await page.getByRole('button',{name:'访问凭据',exact:true}).click();await page.getByRole('tab',{name:'Key',exact:true}).click();
 const table=page.getByRole('region',{name:'Key 目录',exact:true});
 await table.getByRole('checkbox',{name:'选择Key 目录当前页'}).check();await expect(table.getByText(/已选 \d+ 项/)).toBeVisible();
 await table.getByRole('button',{name:'清空选择'}).click();await expect(table.getByRole('button',{name:'复制所选'})).toHaveCount(0);
 const columnTrigger=table.getByRole('button',{name:'Key 目录列设置'});await columnTrigger.focus();await page.keyboard.press('Enter');
 const column=page.getByRole('menuitemcheckbox',{name:'团队',exact:true});await expect(column).toBeChecked();await column.click();await page.keyboard.press('Escape');await expect(columnTrigger).toBeFocused();
 await expect(table.getByRole('columnheader',{name:'团队',exact:true})).toHaveCount(0);
 await columnTrigger.click();await page.getByRole('menuitemcheckbox',{name:'团队',exact:true}).click();await page.keyboard.press('Escape');
 await expect(table.getByRole('columnheader',{name:'团队',exact:true})).toBeVisible();
 await table.getByRole('button',{name:'名称 / 标识',exact:true}).click();await expect(table.getByRole('columnheader',{name:'名称 / 标识',exact:true})).toHaveAttribute('aria-sort','ascending');
 await page.getByRole('searchbox',{name:'搜索 Key',exact:true}).fill('no-matching-record');await expect(page.getByRole('heading',{name:'没有匹配的 Key',exact:true})).toBeVisible();
});
