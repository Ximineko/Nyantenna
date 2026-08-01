export default defineAppConfig({
  ui: {
    // 紫罗兰做强调色：在明暗两种主题下都够醒目又不刺眼，
    // 冷灰中性色与之搭配，整体偏冷静的工具感。
    colors: {
      primary: 'violet',
      neutral: 'slate'
    },
    button: {
      defaultVariants: { size: 'md' }
    },
    card: {
      slots: {
        root: 'rounded-lg',
        header: 'px-4 py-3 sm:px-5',
        body: 'px-4 py-4 sm:px-5',
        footer: 'px-4 py-3 sm:px-5'
      }
    }
  }
})
