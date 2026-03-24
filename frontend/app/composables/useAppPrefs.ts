export type AppStyle = 'classic' | 'bento'

export function useAppPrefs() {
  const isDark    = useState('tsm:dark',  () => false)
  const appStyle  = useState<AppStyle>('tsm:style', () => 'bento')

  const isBento   = computed(() => appStyle.value === 'bento')
  const isClassic = computed(() => appStyle.value === 'classic')

  onMounted(() => {
    isDark.value = localStorage.getItem('tsm-theme') === 'dark'
    const s = localStorage.getItem('tsm-style')
    if (s === 'classic' || s === 'bento') appStyle.value = s
  })

  function setTheme(dark: boolean) {
    isDark.value = dark
    localStorage.setItem('tsm-theme', dark ? 'dark' : 'light')
  }

  function toggleTheme() { setTheme(!isDark.value) }

  function setStyle(style: AppStyle) {
    appStyle.value = style
    localStorage.setItem('tsm-style', style)
  }

  return { isDark, appStyle, isBento, isClassic, setTheme, toggleTheme, setStyle }
}
