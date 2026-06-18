import { inject, type InjectionKey } from 'vue'

export type WorkMemoryFlowContext = Record<string, any>

export const workMemoryFlowContextKey: InjectionKey<WorkMemoryFlowContext> = Symbol('work-memory-flow-context')

export function useWorkMemoryFlowContext() {
  const context = inject(workMemoryFlowContextKey)
  if (!context) {
    throw new Error('Work memory flow context is missing')
  }
  return context
}
