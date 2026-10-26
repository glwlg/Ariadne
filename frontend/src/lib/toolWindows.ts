import definitions from '../../../internal/toolregistry/views.json'

export const toolViews = definitions
export type AppToolView = keyof typeof toolViews

export function isToolView(view: string): view is AppToolView {
  return Object.hasOwn(toolViews, view)
}

export function toolViewForCommand(command: string): AppToolView | undefined {
  return (Object.keys(toolViews) as AppToolView[]).find((view) => toolViews[view].command === command)
}
