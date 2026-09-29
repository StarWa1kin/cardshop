import type { ObjectDirective } from 'vue'

const cleanups = new WeakMap<HTMLElement, () => void>()

/** Keyboard-only presentation behavior. The component owns its existing close action. */
export const vModalFocus: ObjectDirective<HTMLElement, () => void> = {
  mounted(element, binding) {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const focusable = () => Array.from(element.querySelectorAll<HTMLElement>(
      'button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    )).filter(item => item.getClientRects().length > 0 && item.getAttribute('aria-hidden') !== 'true')
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !event.isComposing) {
        event.preventDefault()
        event.stopPropagation()
        binding.value()
      } else if (event.key === 'Tab') {
        const nodes = focusable()
        const first = nodes[0]
        const last = nodes[nodes.length - 1]
        if (!first || !last) { event.preventDefault(); element.focus(); return }
        if (event.shiftKey && (document.activeElement === first || document.activeElement === element)) {
          event.preventDefault(); last.focus()
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault(); first.focus()
        }
      }
    }
    element.addEventListener('keydown', keydown)
    // Wait for the Teleport/transition to attach, without changing component state.
    const frame = requestAnimationFrame(() => { (focusable()[0] || element).focus({ preventScroll: true }) })
    cleanups.set(element, () => {
      cancelAnimationFrame(frame)
      element.removeEventListener('keydown', keydown)
      if (previous?.isConnected) previous.focus({ preventScroll: true })
    })
  },
  beforeUnmount(element) {
    cleanups.get(element)?.()
    cleanups.delete(element)
  },
}
