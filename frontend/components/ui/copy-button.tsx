"use client"

import * as React from "react"
import { Check, Copy } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

export interface CopyButtonProps extends Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "onClick"> {
  value: string
  variant?: "default" | "destructive" | "outline" | "secondary" | "ghost" | "link" | "none"
  size?: "default" | "sm" | "lg" | "icon" | "icon-sm" | "icon-lg" | "none"
  iconClassName?: string
  successDuration?: number
  showToast?: boolean
  toastMessage?: string
  tooltip?: string
}

export const CopyButton = React.forwardRef<HTMLButtonElement, CopyButtonProps>(
  (
    {
      value,
      variant = "outline",
      size = "icon",
      className,
      iconClassName,
      successDuration = 1500,
      showToast = false,
      toastMessage = "Copied to clipboard",
      tooltip,
      type = "button",
      ...props
    },
    ref
  ) => {
    const [copied, setCopied] = React.useState(false)

    const handleCopy = React.useCallback(
      async (e: React.MouseEvent<HTMLButtonElement>) => {
        e.stopPropagation()
        e.preventDefault()

        try {
          await navigator.clipboard.writeText(value)
          setCopied(true)

          // Haptic feedback
          if (typeof navigator !== "undefined" && typeof navigator.vibrate === "function") {
            try {
              navigator.vibrate(15) // short clean haptic pulse
            } catch (vibrateErr) {
              // Ignore vibration failure
            }
          }

          if (showToast) {
            toast.success(toastMessage)
          }
        } catch (err) {
          console.error("Failed to copy text: ", err)
        }
      },
      [value, showToast, toastMessage]
    )

    React.useEffect(() => {
      if (!copied) return
      const timer = setTimeout(() => setCopied(false), successDuration)
      return () => clearTimeout(timer)
    }, [copied, successDuration])

    const Icon = copied ? Check : Copy
    const computedIconClassName = cn(
      "transition-transform duration-200 shrink-0",
      copied ? "text-emerald-500 scale-110" : "",
      iconClassName
    )

    const buttonElement =
      variant === "none" ? (
        <button
          ref={ref}
          type={type}
          onClick={handleCopy}
          className={cn("active:scale-95 transition-transform duration-100 outline-none select-none", className)}
          {...props}
        >
          <Icon className={computedIconClassName} />
        </button>
      ) : (
        <Button
          ref={ref}
          type={type}
          variant={variant}
          size={size === "none" ? undefined : size}
          onClick={handleCopy}
          className={cn("active:scale-95 transition-transform duration-100 select-none", className)}
          {...props}
        >
          <Icon className={computedIconClassName} />
        </Button>
      )

    if (tooltip) {
      return (
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>{buttonElement}</TooltipTrigger>
            <TooltipContent>
              <p>{tooltip}</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      )
    }

    return buttonElement
  }
)

CopyButton.displayName = "CopyButton"
