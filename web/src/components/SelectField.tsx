import * as React from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select";

type Choice = { value: string; label: React.ReactNode; disabled?: boolean };
export function SelectField({
  value,
  defaultValue,
  onValueChange,
  options,
  name,
  required,
  disabled,
  id,
  className,
  "aria-label": label,
  "aria-describedby": description,
  autoFocus,
}: {
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  options: Choice[];
  name?: string;
  required?: boolean;
  disabled?: boolean;
  id?: string;
  className?: string;
  "aria-label"?: string;
  "aria-describedby"?: string;
  autoFocus?: boolean;
}) {
  return (
    <Select
      value={value}
      defaultValue={defaultValue}
      onValueChange={(next) => onValueChange?.(next ?? "")}
      items={options}
      name={name}
      required={required}
      disabled={disabled}
    >
      <SelectTrigger
        data-field-name={name}
        data-value={value ?? defaultValue}
        className={className}
        id={id}
        aria-label={label}
        aria-describedby={description}
        autoFocus={autoFocus}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        {options.map((option, index) => (
          <SelectItem
            data-value={option.value}
            key={`${option.value}-${index}`}
            value={option.value}
            disabled={option.disabled}
          >
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
