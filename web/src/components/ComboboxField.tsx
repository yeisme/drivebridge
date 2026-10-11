import {
  Combobox,
  ComboboxInput,
  ComboboxContent,
  ComboboxList,
  ComboboxItem,
  ComboboxEmpty,
} from "./ui/combobox";
import { Button } from "./ui/button";
export type ComboboxOption = {
  value: string;
  label: string;
  disabled?: boolean;
  reason?: string;
};
export function ComboboxField({
  value,
  onValueChange,
  options,
  placeholder = "请选择",
  disabled,
  name,
  required,
  id,
  className,
  onSearchChange,
  loading,
  error,
  onRetry,
  clearable = true,
  "aria-label": label,
}: {
  value: string;
  onValueChange: (value: string) => void;
  options: ComboboxOption[];
  placeholder?: string;
  disabled?: boolean;
  name?: string;
  required?: boolean;
  id?: string;
  className?: string;
  onSearchChange?: (query: string) => void;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
  clearable?: boolean;
  "aria-label"?: string;
}) {
  const selected = options.find((o) => o.value === value) ?? null;
  return (
    <Combobox
      items={options}
      value={selected}
      name={name}
      required={required}
      disabled={disabled}
      isItemEqualToValue={(a, b) => a.value === b.value}
      onValueChange={(next) => onValueChange(next?.value ?? "")}
      filter={onSearchChange ? null : undefined}
      onInputValueChange={(query, details) => {
        if (details.reason === "input-change") onSearchChange?.(query);
      }}
    >
      <ComboboxInput
        id={id}
        className={className ?? "w-full"}
        aria-label={label}
        data-field-name={name}
        data-value={value}
        placeholder={placeholder}
        showClear={clearable && Boolean(value)}
        showTrigger
        disabled={disabled}
      />
      <ComboboxContent>
        {loading && (
          <p role="status" className="p-2 text-sm text-muted-foreground">
            正在搜索…
          </p>
        )}
        {error && (
          <div role="alert" className="p-2 text-sm text-destructive">
            {error}
            {onRetry && (
              <Button type="button" variant="outline" onClick={onRetry}>
                重试
              </Button>
            )}
          </div>
        )}
        <ComboboxEmpty>没有匹配结果</ComboboxEmpty>
        <ComboboxList>
          {(option: ComboboxOption) => (
            <ComboboxItem
              key={option.value}
              value={option}
              disabled={option.disabled}
              data-value={option.value}
            >
              <span>
                {option.label}
                {option.reason && (
                  <small className="block text-muted-foreground">
                    {option.reason}
                  </small>
                )}
              </span>
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}
