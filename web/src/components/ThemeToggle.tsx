import { useState, useEffect } from 'react';

function getInitialTheme(): 'dark' | 'light' {
  const stored = localStorage.getItem('fleet-theme');
  if (stored === 'light' || stored === 'dark') return stored;
  return 'dark';
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<'dark' | 'light'>(getInitialTheme);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('fleet-theme', theme);
  }, [theme]);

  // Apply theme on mount (before first paint would be ideal but this is close enough)
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', getInitialTheme());
  }, []);

  const toggle = () => setTheme(t => t === 'dark' ? 'light' : 'dark');

  return (
    <button
      onClick={toggle}
      aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
      style={{
        position: 'fixed',
        top: '1rem',
        right: '1rem',
        zIndex: 9999,
        background: 'var(--bg-tertiary)',
        border: '1px solid var(--border)',
        borderRadius: '8px',
        padding: '0.4rem 0.5rem',
        cursor: 'pointer',
        fontSize: '1.1rem',
        lineHeight: 1,
        color: 'var(--text-primary)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {theme === 'dark' ? '☀️' : '🌙'}
    </button>
  );
}
