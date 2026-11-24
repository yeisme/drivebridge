import { expect, type Locator } from "@playwright/test";
export async function selectChoice(
  trigger: Locator,
  choice: string | { label?: string; labelContains?: string },
) {
  if (typeof choice === 'string' && await trigger.getAttribute('data-value') === choice) return;
  await trigger.click();
  await expect(trigger).toHaveAttribute("aria-expanded", "true");
  const page = trigger.page();
  await expect(trigger).toHaveAttribute("aria-controls", /.+/);
  // Closing portals can briefly remain visible. Only select in this control's list.
  const popup = page.locator(`[id=${JSON.stringify(await trigger.getAttribute("aria-controls"))}]`);
  const visible = popup.locator("[role=option]:visible");
  const option =
    typeof choice === "string"
      ? popup.locator(
          `[role=option][data-value=${JSON.stringify(choice)}]:visible`,
        )
      : choice.labelContains
        ? visible.filter({ hasText: choice.labelContains })
        : popup.getByRole("option", { name: choice.label, exact: true });
  await option.first().click();
  await expect
    .poll(
      async () =>
        (await trigger.count()) === 0 ||
        (await trigger.getAttribute("aria-expanded")) === "false",
    )
    .toBe(true);
}
