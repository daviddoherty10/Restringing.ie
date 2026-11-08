import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './features/compliance/Compliance.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <a className="skip-link" href="#main-content">Skip to main content</a>
    <App />
  </StrictMode>,
)
