import { useEffect, useId, useRef, type ReactNode } from 'react'

interface Props {
  title: string
  onClose: () => void
  children: ReactNode
}

/**
 * Modal built on the native <dialog>: showModal() traps focus and makes the
 * page inert; Escape fires `cancel`, which we route to onClose. Mount it only
 * while open. On unmount, focus goes back to the element that opened it.
 */
export function Modal({ title, onClose, children }: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const titleId = useId()

  useEffect(() => {
    const dialog = ref.current
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    dialog?.showModal()
    return () => {
      dialog?.close()
      opener?.focus()
    }
  }, [])

  return (
    <dialog
      ref={ref}
      className="modal"
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault()
        onClose()
      }}
    >
      <div className="modal-head">
        <h2 id={titleId}>{title}</h2>
        <button type="button" className="icon-btn" aria-label="Close" onClick={onClose}>
          ×
        </button>
      </div>
      {children}
    </dialog>
  )
}
