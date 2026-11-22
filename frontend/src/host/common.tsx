// Общие элементы пульта: тосты и подтверждение опасных действий.

import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'

interface ToastCtx {
  showToast: (message: string, isError?: boolean) => void
}

const ToastContext = createContext<ToastCtx>({ showToast: () => {} })

export function useToast() {
  return useContext(ToastContext)
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState<{ message: string; isError: boolean; id: number } | null>(null)

  const showToast = useCallback((message: string, isError = false) => {
    const id = Date.now()
    setToast({ message, isError, id })
    window.setTimeout(() => setToast((t) => (t?.id === id ? null : t)), 3200)
  }, [])

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      {toast && (
        <div className={'toast' + (toast.isError ? ' toast--error' : '')}>{toast.message}</div>
      )}
    </ToastContext.Provider>
  )
}

// Модальное подтверждение опасного действия.
export function Confirm(props: {
  title: string
  confirmLabel?: string
  onConfirm: () => void
  onCancel: () => void
}) {
  return (
    <div className="confirm-overlay" onClick={props.onCancel}>
      <div className="confirm-box" onClick={(e) => e.stopPropagation()}>
        <div className="confirm-box__title">{props.title}</div>
        <div className="confirm-box__actions">
          <button className="btn btn--ghost" onClick={props.onCancel}>
            Отмена
          </button>
          <button className="btn btn--danger" onClick={props.onConfirm}>
            {props.confirmLabel ?? 'Подтвердить'}
          </button>
        </div>
      </div>
    </div>
  )
}
