/**
 * Collect File objects from a drag-and-drop DataTransfer, including dropped
 * folders (Chromium) via webkitGetAsEntry. Sets webkitRelativePath on each
 * file so the import API receives correct paths.
 */
export async function collectFilesFromDataTransfer(
  dt: DataTransfer,
): Promise<File[]> {
  const items = dt.items
  if (
    items &&
    items.length > 0 &&
    typeof items[0].webkitGetAsEntry === "function"
  ) {
    const out: File[] = []
    for (let i = 0; i < items.length; i++) {
      const entry = items[i].webkitGetAsEntry?.()
      if (entry) {
        await walkEntry(entry, "", out)
      }
    }
    if (out.length > 0) {
      return out
    }
  }
  return Array.from(dt.files ?? [])
}

async function walkEntry(
  entry: FileSystemEntry,
  pathPrefix: string,
  out: File[],
): Promise<void> {
  if (entry.isFile) {
    const rel = pathPrefix + entry.name
    const fe = entry as FileSystemFileEntry
    const file = await new Promise<File>((resolve, reject) => {
      fe.file(resolve, reject)
    })
    Object.defineProperty(file, "webkitRelativePath", {
      value: rel,
      configurable: true,
      enumerable: true,
    })
    out.push(file)
    return
  }
  if (!entry.isDirectory) {
    return
  }
  const dir = entry as FileSystemDirectoryEntry
  const prefix = pathPrefix + entry.name + "/"
  const children = await readAllDirectoryEntries(dir)
  for (const child of children) {
    await walkEntry(child, prefix, out)
  }
}

function readAllDirectoryEntries(
  dir: FileSystemDirectoryEntry,
): Promise<FileSystemEntry[]> {
  return new Promise((resolve, reject) => {
    const reader = dir.createReader()
    const all: FileSystemEntry[] = []
    const read = () => {
      reader.readEntries(
        (entries) => {
          if (entries.length === 0) {
            resolve(all)
            return
          }
          all.push(...entries)
          read()
        },
        (err) => reject(err),
      )
    }
    read()
  })
}
