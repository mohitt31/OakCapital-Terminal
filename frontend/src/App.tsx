import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import TerminalLayout from './components/TerminalLayout'
import DesktopPage from './components/DesktopPage'
import LoginPage from './components/LoginPage'
import SignInPage from './components/SignInPage'
import { MarketsPage } from './components/markets/MarketsPage'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<DesktopPage />} />
        <Route path="/dashboard" element={<DesktopPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signin" element={<SignInPage />} />
        <Route path="/markets" element={<MarketsPage />} />
        <Route path="/terminal" element={<TerminalLayout />} />
        <Route path="/trade" element={<TerminalLayout />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
