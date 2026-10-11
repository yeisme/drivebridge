import { Fragment, type ReactNode } from "react";
import { MoreHorizontal } from "lucide-react";
import { Button } from "./ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu";

export type ActionMenuItem = {
  label: string;
  onSelect: () => void;
  destructive?: boolean;
  disabled?: boolean;
};

export function ActionMenu({
  items,
  label = "更多操作",
}: {
  items: ActionMenuItem[];
  label?: string;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="ghost" size="icon" aria-label={label}>
            <MoreHorizontal />
          </Button>
        }
      />
      <DropdownMenuContent align="end">
        {items.map((item, index) => (
          <Fragment key={item.label}>
            {index > 0 && item.destructive && <DropdownMenuSeparator />}
            <DropdownMenuItem
              variant={item.destructive ? "destructive" : "default"}
              disabled={item.disabled}
              onClick={item.onSelect}
            >
              {item.label}
            </DropdownMenuItem>
          </Fragment>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
