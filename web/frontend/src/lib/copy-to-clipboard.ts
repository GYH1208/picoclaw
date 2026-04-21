/**
 * Copy text to the clipboard. Uses the Clipboard API when available; falls back to
 * execCommand for contexts where clipboard API fails (e.g. non-secure HTTP origins).
 */
export async function copyTextToClipboard(text: string): Promise<boolean> {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Fall through — common on http:// LAN URLs or locked-down embeds.
    }
  }

  try {
    const ta = document.createElement("textarea")
    ta.value = text
    ta.setAttribute("readonly", "")
    ta.style.position = "fixed"
    ta.style.left = "-9999px"
    ta.style.top = "0"
    document.body.appendChild(ta)
    ta.select()
    ta.setSelectionRange(0, text.length)
    const ok = document.execCommand("copy")
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}
