import { useRoom } from '../lib/room'
import { IconWifiOff, IconPause } from '../components/Icon'

// Оверлеи состояния соединения и паузы:
// - потеря связи ТВ с сервером;
// - ведущий отключился → «Игра на паузе»;
// - ручная пауза.
export default function ConnectionOverlay() {
  const { snapshot, status } = useRoom()
  const room = snapshot?.room

  if (status === 'offline') {
    return (
      <div className="tv-overlay">
        <div className="tv-overlay__box">
          <IconWifiOff width={56} height={56} />
          Потеряно соединение с сервером
          <div className="tv-overlay__sub">Переподключение…</div>
        </div>
      </div>
    )
  }

  if (room?.status === 'paused') {
    const hostGone = !room.hostConnected
    return (
      <div className="tv-overlay">
        <div className="tv-overlay__box">
          <IconPause width={56} height={56} />
          {hostGone ? 'Ведущий отключился. Игра на паузе' : 'Пауза'}
          <div className="tv-overlay__sub">
            {hostGone ? 'Ожидание переподключения ведущего…' : 'Ведущий поставил игру на паузу'}
          </div>
        </div>
      </div>
    )
  }

  return null
}
