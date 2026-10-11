import type {Locator} from "@playwright/test";
export async function keyAction(row: Locator, action: string) {
  await row.getByRole("button",{name:/的更多操作$/}).click();
  await row.page().getByRole("menuitem",{name:action,exact:true}).click();
}
