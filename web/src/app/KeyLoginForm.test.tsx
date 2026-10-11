import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { KeyLoginForm } from "./KeyLoginForm";

afterEach(cleanup);

it("keeps secret entry masked and delegates changes without storing it", () => {
  const onChange = vi.fn();
  const onSubmit = vi.fn();
  render(<KeyLoginForm value="fixture-key" busy={false} secondary={false} onChange={onChange} onSubmit={onSubmit} />);
  const input = screen.getByLabelText("使用密钥登录") as HTMLInputElement;
  expect(input.type).toBe("password");
  expect(input.autocomplete).toBe("off");
  fireEvent.change(input, { target: { value: "new-fixture" } });
  expect(onChange).toHaveBeenCalledWith("new-fixture");
  fireEvent.submit(input.closest("form")!);
  expect(onSubmit).toHaveBeenCalledTimes(1);
});

it.each([{ value: "   ", busy: false }, { value: "fixture-key", busy: true }])("rejects empty or busy submit even through form events", ({ value, busy }) => {
  const onSubmit = vi.fn();
  render(<KeyLoginForm value={value} busy={busy} secondary onChange={vi.fn()} onSubmit={onSubmit} />);
  expect((screen.getByRole("button") as HTMLButtonElement).disabled).toBe(true);
  fireEvent.submit(screen.getByLabelText("使用密钥登录").closest("form")!);
  expect(onSubmit).not.toHaveBeenCalled();
});
