import { useRoom } from '../lib/room'

// Подготовка игры: название пака, логотип, список игроков.
export default function LobbyScreen() {
  const { snapshot } = useRoom()
  const pack = snapshot?.pack
  const players = snapshot?.players ?? []

  return (
    <div className="tv-lobby">
      <div className="tv-waiting__title" style={{ fontSize: '7vh' }}>Своя Игра</div>
      {pack ? (
        <>
          {pack.logoUrl && <img className="tv-lobby__logo" src={pack.logoUrl} alt={pack.name} />}
          <div className="tv-lobby__pack">{pack.name}</div>
          {pack.author && <div className="tv-lobby__meta">Автор пакета: {pack.author}</div>}
          <div className="tv-lobby__meta">
            Раундов: {pack.rounds.length}
          </div>
        </>
      ) : (
        <div className="tv-lobby__meta">Ведущий подключён. Ожидание загрузки пакета вопросов…</div>
      )}
      {players.length > 0 && (
        <div className="tv-lobby__meta">
          Игроков: {players.length} — {players.map((p) => p.name).join(', ')}
        </div>
      )}
      <div className="tv-lobby__meta">Ожидание начала игры…</div>
    </div>
  )
}
