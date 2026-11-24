export function isComposingKeyboardEvent(event: Pick<KeyboardEvent, 'isComposing' | 'keyCode'>): boolean {
  return Boolean(event.isComposing) || event.keyCode === 229
}

export function isEditableElement(target: EventTarget | null): boolean {
  if (!target || typeof target !== 'object') {
    return false
  }
  const el = target as { tagName?: string; isContentEditable?: boolean }
  const tag = typeof el.tagName === 'string' ? el.tagName.toLowerCase() : ''
  return tag === 'input' || tag === 'textarea' || Boolean(el.isContentEditable)
}

export function isNativeInteractiveTarget(target: EventTarget | null): boolean {
  if (!target || typeof target !== 'object') {
    return false
  }
  const el = target as { closest?: (selector: string) => unknown }
  if (typeof el.closest !== 'function') {
    return false
  }
  return Boolean(el.closest('button, a, [role="menuitem"]'))
}

export function isOverlayMenuOrDialogOpen(
  target: EventTarget | null,
  querySelector?: (selector: string) => unknown,
): boolean {
  if (target && typeof target === 'object') {
    const el = target as { closest?: (selector: string) => unknown }
    if (typeof el.closest === 'function' && el.closest('[role="menu"], [role="dialog"]')) {
      return true
    }
  }
  if (typeof querySelector === 'function') {
    return Boolean(
      querySelector('[role="menu"][data-state="open"], .action-menu[data-state="open"], [role="dialog"][data-state="open"]'),
    )
  }
  return false
}

export type LauncherKeyAction = 'ignore' | 'escape' | 'down' | 'up' | 'enter'

export function resolveLauncherKeyAction(
  event: Pick<KeyboardEvent, 'key' | 'isComposing' | 'keyCode' | 'target'>,
  searchInputEl: EventTarget | null,
  querySelector?: (selector: string) => unknown,
): LauncherKeyAction {
  if (isComposingKeyboardEvent(event)) {
    return 'ignore'
  }
  const key = event.key
  if (key !== 'Escape' && key !== 'ArrowDown' && key !== 'ArrowUp' && key !== 'Enter') {
    return 'ignore'
  }
  if (isOverlayMenuOrDialogOpen(event.target, querySelector)) {
    return 'ignore'
  }
  if (key === 'Escape') {
    return 'escape'
  }
  if (event.target !== searchInputEl && isEditableElement(event.target)) {
    return 'ignore'
  }
  if (key === 'Enter' && event.target !== searchInputEl && isNativeInteractiveTarget(event.target)) {
    return 'ignore'
  }
  if (key === 'ArrowDown') return 'down'
  if (key === 'ArrowUp') return 'up'
  return 'enter'
}
