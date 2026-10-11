import { expect, type Locator } from "@playwright/test";
export async function selectChoice(
  trigger: Locator,
  choice: string | { label?: string; labelContains?: string },
) {
  await trigger.click();
  await expect(trigger).toHaveAttribute("aria-expanded", "true");
  const page = trigger.page();
  const visible = page.locator("[role=option]:visible");
  const option =
    typeof choice === "string"
      ? page.locator(
          `[role=option][data-value=${JSON.stringify(choice)}]:visible`,
        )
      : choice.labelContains
        ? visible.filter({ hasText: choice.labelContains })
        : page.getByRole("option", { name: choice.label, exact: true });
  await option.first().click();
  await expect
    .poll(
      async () =>
        (await trigger.count()) === 0 ||
        (await trigger.getAttribute("aria-expanded")) === "false",
    )
    .toBe(true);
}
