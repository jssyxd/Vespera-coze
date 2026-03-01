import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import ScanManagement from './pages/ScanManagement'
import AiValidation from './pages/AiValidation'
import Etherscan from './pages/Etherscan'
import Github from './pages/Github'

function App() {
  return (
    <Routes>
      <Route path="/panel" element={<Layout />}>
        <Route index element={<Dashboard />} />
        <Route path="scans" element={<ScanManagement />} />
        <Route path="ai-validate" element={<AiValidation />} />
        <Route path="etherscan" element={<Etherscan />} />
        <Route path="github" element={<Github />} />
      </Route>
      <Route path="*" element={<Navigate to="/panel" replace />} />
    </Routes>
  )
}

export default App
