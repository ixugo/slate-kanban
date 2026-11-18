import * as React from "react"
import { cn } from "cn"
import { Switch as SwitchPrimitive } from "radix-ui"

function Switch({
  className,
  size = "default",
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root> & {
  size?: "sm" | "default"
}) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        "peer group/switch relative inline-flex shrink-0 items-center rounded-full p-[2px] transition-colors duration-300 ease-spring outline-none after:absolute after:-inset-x-2 after:-inset-y-2 focus-visible:ring-[3px] focus-visible:ring-ring/40 aria-invalid:ring-[3px] aria-invalid:ring-destructive/30 data-[size=default]:h-[1.625rem] data-[size=default]:w-[2.75rem] data-[size=sm]:h-5 data-[size=sm]:w-[2rem] data-checked:bg-success data-unchecked:bg-foreground/15 dark:data-unchecked:bg-white/22 data-disabled:cursor-not-allowed data-disabled:opacity-50",
        className
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="pointer-events-none block rounded-full bg-white shadow-[0_2px_4px_oklch(0_0_0/0.25),0_0_0_0.5px_oklch(0_0_0/0.06)] ring-0 transition-[transform,width] duration-300 ease-spring-bounce group-data-[size=default]/switch:h-[1.375rem] group-data-[size=default]/switch:w-[1.375rem] group-data-[size=default]/switch:group-active/switch:w-[1.75rem] group-data-[size=sm]/switch:size-4 group-data-[size=sm]/switch:group-active/switch:w-[1.25rem] group-data-[size=default]/switch:data-checked:translate-x-[1.125rem] group-data-[size=default]/switch:group-active/switch:data-checked:translate-x-[0.75rem] group-data-[size=sm]/switch:data-checked:translate-x-3 group-data-[size=sm]/switch:group-active/switch:data-checked:translate-x-[0.5rem] data-unchecked:translate-x-0"
      />
    </SwitchPrimitive.Root>
  )
}

export { Switch }
