"use client"

import { IconHistory, IconPencil, IconTrash } from "@tabler/icons-react"
import dayjs from "dayjs"
import type { RefObject } from "react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import type { SessionSummary } from "@/api/sessions"
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"

interface SessionHistoryMenuProps {
  sessions: SessionSummary[]
  activeSessionId: string
  hasMore: boolean
  loadError: boolean
  loadErrorMessage: string
  observerRef: RefObject<HTMLDivElement | null>
  onOpenChange: (open: boolean) => void
  onSwitchSession: (sessionId: string) => void
  onDeleteSession: (sessionId: string) => void
  onRenameSession: (sessionId: string, title: string) => void | Promise<void>
}

export function SessionHistoryMenu({
  sessions,
  activeSessionId,
  hasMore,
  loadError,
  loadErrorMessage,
  observerRef,
  onOpenChange,
  onSwitchSession,
  onDeleteSession,
  onRenameSession,
}: SessionHistoryMenuProps) {
  const { t } = useTranslation()
  const [menuOpen, setMenuOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<SessionSummary | null>(null)
  const [renameDraft, setRenameDraft] = useState("")
  const [renameSaving, setRenameSaving] = useState(false)

  const handleMenuOpenChange = (open: boolean) => {
    setMenuOpen(open)
    onOpenChange(open)
  }

  const openRename = (session: SessionSummary) => {
    setRenameTarget(session)
    setRenameDraft(session.title || session.preview)
    setMenuOpen(false)
  }

  const handleRenameDialogOpenChange = (open: boolean) => {
    if (!open && !renameSaving) {
      setRenameTarget(null)
    }
  }

  const submitRename = async () => {
    if (!renameTarget) return
    setRenameSaving(true)
    try {
      await onRenameSession(renameTarget.id, renameDraft.trim())
      setRenameTarget(null)
    } finally {
      setRenameSaving(false)
    }
  }

  return (
    <>
      <DropdownMenu open={menuOpen} onOpenChange={handleMenuOpenChange}>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary" size="sm" className="h-9 gap-2">
            <IconHistory className="size-4" />
            <span className="hidden sm:inline">{t("chat.history")}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-72">
          <ScrollArea className="max-h-[300px]">
            {loadError && (
              <DropdownMenuItem disabled>
                <span className="text-destructive text-xs">
                  {loadErrorMessage}
                </span>
              </DropdownMenuItem>
            )}
            {sessions.length === 0 && !loadError ? (
              <DropdownMenuItem disabled>
                <span className="text-muted-foreground text-xs">
                  {t("chat.noHistory")}
                </span>
              </DropdownMenuItem>
            ) : (
              sessions.map((session) => (
                <DropdownMenuItem
                  key={session.id}
                  className={`group relative my-0.5 flex flex-col items-start gap-0.5 pr-16 ${
                    session.id === activeSessionId ? "bg-accent" : ""
                  }`}
                  onClick={() => onSwitchSession(session.id)}
                >
                  <span className="line-clamp-1 text-sm font-medium">
                    {session.title || session.preview}
                  </span>
                  <span className="text-muted-foreground text-xs">
                    {t("chat.messagesCount", {
                      count: session.message_count,
                    })}{" "}
                    · {dayjs(session.updated).fromNow()}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={t("chat.renameSession")}
                    className="text-muted-foreground hover:bg-accent absolute top-1/2 right-9 h-6 w-6 -translate-y-1/2 opacity-0 transition-opacity group-hover:opacity-100"
                    onClick={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      openRename(session)
                    }}
                  >
                    <IconPencil className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={t("chat.deleteSession")}
                    className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive absolute top-1/2 right-2 h-6 w-6 -translate-y-1/2 opacity-0 transition-opacity group-hover:opacity-100"
                    onClick={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      onDeleteSession(session.id)
                    }}
                  >
                    <IconTrash className="h-4 w-4" />
                  </Button>
                </DropdownMenuItem>
              ))
            )}
            {hasMore && sessions.length > 0 && (
              <div ref={observerRef} className="py-2 text-center">
                <span className="text-muted-foreground animate-pulse text-xs">
                  {t("chat.loadingMore")}
                </span>
              </div>
            )}
          </ScrollArea>
        </DropdownMenuContent>
      </DropdownMenu>

      <AlertDialog
        open={renameTarget !== null}
        onOpenChange={handleRenameDialogOpenChange}
      >
        <AlertDialogContent className="sm:max-w-md" size="default">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("chat.renameSessionTitle")}</AlertDialogTitle>
          </AlertDialogHeader>
          <div className="grid gap-2 py-1">
            <Label htmlFor="session-rename-input">
              {t("chat.renameSessionLabel")}
            </Label>
            <Input
              id="session-rename-input"
              value={renameDraft}
              onChange={(e) => setRenameDraft(e.target.value)}
              placeholder={t("chat.renameSessionPlaceholder")}
              disabled={renameSaving}
              maxLength={60}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !renameSaving) {
                  e.preventDefault()
                  void submitRename()
                }
              }}
            />
            <p className="text-muted-foreground text-xs">
              {t("chat.renameSessionHint")}
            </p>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={renameSaving}>
              {t("chat.renameSessionCancel")}
            </AlertDialogCancel>
            <Button
              type="button"
              disabled={renameSaving}
              onClick={() => void submitRename()}
            >
              {renameSaving
                ? t("chat.renameSessionSaving")
                : t("chat.renameSessionSave")}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
