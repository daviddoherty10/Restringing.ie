import { useEffect, useRef, useState } from 'react'
import SessionMenu from '../../features/auth/SessionMenu'
import { BrandLogo } from '../ui/BrandLogo'
import './SiteHeader.css'
export default function SiteHeader({ variant = 'home' }: { variant?: 'home' | 'marketplace' }) {
  const [open, setOpen] = useState(false)
  const header = useRef<HTMLElement>(null), toggle = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const media = window.matchMedia('(max-width: 1050px)')
    const resize = () => setOpen(false)
    media.addEventListener('change', resize)
    return () => media.removeEventListener('change', resize)
  }, [])
  useEffect(() => {
    if (!open) return
    function escape(e: KeyboardEvent) { if (e.key === 'Escape') { setOpen(false); toggle.current?.focus() } }
    function outside(e: PointerEvent) { if (!header.current?.contains(e.target as Node)) setOpen(false) }
    window.addEventListener('keydown', escape); window.addEventListener('pointerdown', outside)
    return () => { window.removeEventListener('keydown', escape); window.removeEventListener('pointerdown', outside) }
  }, [open])
  return <header ref={header} onBlur={e => { if (e.relatedTarget && !e.currentTarget.contains(e.relatedTarget as Node)) setOpen(false) }} className="site-header shared-header"><div className="shared-header-inner"><BrandLogo variant={variant} /><button ref={toggle} type="button" className="shared-menu-toggle" aria-label={open ? 'Close menu' : 'Open menu'} aria-expanded={open} aria-controls="site-navigation" onClick={() => setOpen(v => !v)}><span aria-hidden="true">{open ? '✕' : '☰'}</span><span>Menu</span></button><nav id="site-navigation" className={`shared-navigation${open ? ' shared-navigation-open' : ''}`} aria-label="Main navigation" onClick={e => { if ((e.target as HTMLElement).closest('a')) setOpen(false) }}><div className="shared-primary-links"><a href="/#how-it-works">How it works</a><a href="/search">Find a stringer</a><a href="/#why-restring">Why Restringing.ie</a><a href="/#for-stringers">For stringers</a></div><SessionMenu /></nav></div></header>
}
