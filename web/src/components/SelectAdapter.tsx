import { ComboboxField } from "./ComboboxField";
import * as React from "react";
import { SelectField } from "./SelectField";

type Change = { target: { value: string }; currentTarget: { value: string } };
type Props = Omit<React.SelectHTMLAttributes<HTMLSelectElement>, "onChange"> & {
  onChange?: (event: Change) => void;
};

// Converts existing option declarations to the single shared selection control.
export function SelectAdapter({
  children,
  value,
  defaultValue,
  onChange,
  disabled,
  required,
  name,
  className,
  id,
  "aria-label": label,
  "aria-describedby": description,
  autoFocus,
}: Props) {
  const [internal, setInternal] = React.useState(
    defaultValue == null ? undefined : String(defaultValue),
  );
  const options = React.Children.toArray(children).flatMap((child) => {
    if (!React.isValidElement(child)) return [];
    const props = child.props as {
      value?: string | number;
      children?: React.ReactNode;
      disabled?: boolean;
    };
    return [
      {
        value: String(props.value ?? ""),
        label: props.children,
        disabled: props.disabled,
      },
    ];
  });
  const searchable =
    /user|connection|space|team|target/.test(name ?? "") ||
    /存储|空间|租户|所属团队/.test(label ?? "");
  if (searchable) {
    const text = (node: React.ReactNode): string =>
      React.Children.toArray(node)
        .map((c) =>
          React.isValidElement<{ children?: React.ReactNode }>(c)
            ? text(c.props.children)
            : String(c),
        )
        .join("");
    const current =
      value == null
        ? (internal ?? options.find((o) => !o.disabled)?.value ?? "")
        : String(value);
    return (
      <ComboboxField
        value={current}
        onValueChange={(next) => {
          setInternal(next);
          onChange?.({
            target: { value: next },
            currentTarget: { value: next },
          });
        }}
        options={options.map((o) => ({ ...o, label: text(o.label) }))}
        disabled={disabled}
        required={required}
        clearable={!required && options.some(option => option.value === "")}
        name={name}
        id={id}
        className={className}
        aria-label={label}
      />
    );
  }
  return (
    <>
      <SelectField
        value={value == null ? undefined : String(value)}
        defaultValue={
          defaultValue == null
            ? options.find((option) => !option.disabled)?.value
            : String(defaultValue)
        }
        options={options}
        onValueChange={(next) =>
          onChange?.({
            target: { value: next },
            currentTarget: { value: next },
          })
        }
        disabled={disabled}
        required={required}
        name={name}
        id={id}
        className={className}
        aria-label={label}
        aria-describedby={description}
        autoFocus={autoFocus}
      />
    </>
  );
}
