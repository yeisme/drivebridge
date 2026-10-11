import { X } from "lucide-react";
import { ComboboxField, type ComboboxOption } from "./ComboboxField";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
export function MultiSelect({
  values,
  onValueChange,
  options,
  label,
  disabled,
  onSearchChange,
  loading,
  error,
}: {
  values: string[];
  onValueChange: (values: string[]) => void;
  options: ComboboxOption[];
  label: string;
  disabled?: boolean;
  onSearchChange?: (query: string) => void;
  loading?: boolean;
  error?: string;
}) {
  return (
    <div className="grid gap-2">
      <ComboboxField
        aria-label={label}
        value=""
        onSearchChange={onSearchChange}
        loading={loading}
        error={error}
        onValueChange={(value) => {
          if (value && !values.includes(value))
            onValueChange([...values, value]);
        }}
        options={options.filter((o) => !values.includes(o.value))}
        disabled={disabled}
        placeholder={label}
      />
      <div className="flex flex-wrap gap-1">
        {values.map((value) => (
          <Badge key={value} variant="secondary">
            {options.find((o) => o.value === value)?.label || value}
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              disabled={disabled}
              aria-label={`移除 ${options.find((o) => o.value === value)?.label || value}`}
              onClick={() => onValueChange(values.filter((v) => v !== value))}
            >
              <X />
            </Button>
          </Badge>
        ))}
      </div>
    </div>
  );
}
