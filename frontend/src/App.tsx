import { Route, Routes } from 'react-router-dom'

import HomePage from './pages/HomePage'
import NewRoomPage from './pages/NewRoomPage'
import DisplayPage from './pages/DisplayPage'
import HostPage from './pages/HostPage'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/room/new" element={<NewRoomPage />} />
      <Route path="/room/:roomId/display" element={<DisplayPage />} />
      <Route path="/room/:roomId/host" element={<HostPage />} />
      {/* /room/:id/watch — режим зрителя, зарезервирован под будущие этапы */}
      <Route path="*" element={<HomePage />} />
    </Routes>
  )
}
