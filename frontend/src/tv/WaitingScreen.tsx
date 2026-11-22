import { useRoom } from '../lib/room'
import { IconPhone } from '../components/Icon'

// Экран ожидания ведущего: крупный код комнаты и инструкция.
export default function WaitingScreen() {
  const { snapshot } = useRoom()
  const code = snapshot?.room.code ?? ''

  return (
    <div className="tv-waiting">
      <div className="tv-waiting__title">Своя Игра</div>
      <div className="tv-waiting__code">{code}</div>
      <div className="tv-waiting__hint" style={{ display: 'flex', alignItems: 'center', gap: '1.2vh' }}>
        <IconPhone width={40} height={40} />
        Введите код на телефоне, чтобы стать ведущим
      </div>
      <div className="tv-waiting__hint" style={{ fontSize: '3vh' }}>
        {location.host}
      </div>
    </div>
  )
}
