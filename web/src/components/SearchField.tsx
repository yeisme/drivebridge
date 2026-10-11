import * as React from "react";
import { Search } from "lucide-react";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "./ui/input-group";

export function SearchField({
  hint,
  ...props
}: Omit<React.ComponentProps<"input">, "type"> & { hint?: string }) {
  return (
    <InputGroup>
      <InputGroupAddon>
        <Search />
      </InputGroupAddon>
      <InputGroupInput {...props} type="search" />
      {hint ? (
        <InputGroupAddon align="inline-end">
          <kbd className="pointer-events-none select-none text-muted-foreground">
            {hint}
          </kbd>
        </InputGroupAddon>
      ) : null}
    </InputGroup>
  );
}
