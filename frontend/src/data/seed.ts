import type { SearchResult } from '../types/ariadne'

export const seedResults: SearchResult[] = [
  {
    id: 'file-readme',
    type: 'file',
    title: 'README.md',
    subtitle: 'P:\\workspace\\glwlg\\app\\x-tools',
    detail: '项目说明、插件列表、截图贴图和跨平台演进策略。',
    icon: 'file',
    tags: ['Markdown', '文件'],
    preview: {
      kind: 'text',
      title: 'README.md',
      subtitle: '本地文件',
      text: 'x-tools 是一款专为 Windows 打造的本地搜索与效率工具。Ariadne 将继承搜索、截图、插件和桌面工具。',
      meta: [
        { label: '路径', value: 'P:\\workspace\\glwlg\\app\\x-tools\\README.md' },
        { label: '默认操作', value: '打开文件 / 复制路径' },
      ],
    },
    actions: [
      { id: 'open', label: '打开文件', icon: 'open', kind: 'open', shortcut: 'Enter' },
      { id: 'open_parent', label: '打开所在文件夹', icon: 'folder', kind: 'open_parent' },
      {
        id: 'copy_path',
        label: '复制路径',
        icon: 'copy',
        kind: 'copy',
        payload: { text: 'P:\\workspace\\glwlg\\app\\x-tools\\README.md' },
        feedback: { successLabel: '已复制' },
      },
    ],
  },
  {
    id: 'plugin-uuid',
    type: 'plugin_trigger',
    title: 'UUID 生成器',
    subtitle: '插件 · uuid / guid',
    detail: '生成随机 UUID，结果只提供复制动作，不显示文件动作。',
    icon: 'plugin',
    tags: ['插件', '复制结果'],
    payload: {
      pluginId: 'uuid',
      keyword: 'uuid',
      commandSchema: {
        usage: 'uuid [count]',
        examples: ['uuid', 'uuid 10', 'guid 3'],
        params: [{ name: 'count', label: '数量', placeholder: '默认 5，最大 50', required: false }],
      },
    },
    preview: {
      kind: 'text',
      title: 'UUID 生成器',
      subtitle: 'uuid [count]',
      text: '输入 uuid 或 guid 后生成 UUID，结果可直接复制。',
      meta: [
        { label: '示例', value: 'uuid 5' },
        { label: '结果动作', value: '生成后复制' },
      ],
    },
    actions: [
      { id: 'prepare_command', label: '补全命令', icon: 'run', kind: 'run', shortcut: 'Enter', payload: { command: 'uuid' } },
      {
        id: 'copy_command',
        label: '复制用法',
        icon: 'copy',
        kind: 'copy',
        payload: { text: 'uuid [count]' },
        feedback: { successLabel: '已复制' },
      },
    ],
  },
]
