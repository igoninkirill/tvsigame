import { useEffect, useState } from 'react'

import { useRoom } from '../lib/room'

// Заставка раунда: как в телепередаче — темы объявляются по одной,
// затем ведущий переводит игру к табло (BEGIN_ROUND).
// Звук (музыка объявления тем) запускает SoundManager при входе в round_intro.
const REVEAL_MS = 1600

export default function RoundIntro() {
  const { snapshot } = useRoom()
  const intro = snapshot?.game?.intro
  const roundIndex = snapshot?.game?.roundIndex ?? 0
  const themes = intro?.themeNames ?? []
  const [shown, setShown] = useState(0)

  useEffect(() => {
    setShown(0)
  }, [roundIndex, themes.length])

  useEffect(() => {
    if (shown >= themes.length) return
    const t = window.setTimeout(() => {
      setShown((s) => s + 1)
    }, shown === 0 ? 700 : REVEAL_MS)
    return () => window.clearTimeout(t)
  }, [shown, themes.length])

  if (!intro) return null

  return (
    <div className="tv-intro">
      <div className="tv-intro__round">{intro.roundName}</div>
      <div className="tv-intro__themes">
        {themes.slice(0, shown).map((name, i) => (
          <div key={i} className="tv-intro__theme" style={{ animationDelay: '0ms' }}>
            {name}
          </div>
        ))}
      </div>
      {shown >= themes.length && themes.length > 0 && (
        <div className="tv-intro__hint">Ведущий начинает раунд…</div>
      )}
    </div>
  )
}
