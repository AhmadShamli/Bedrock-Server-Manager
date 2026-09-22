import React, { useEffect, useState } from 'react';
import { Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom';
import { api } from './api/client';
import { User } from './types';
import { Navbar } from './components/Navbar';
import { Dashboard } from './pages/Dashboard';
import { ServerHub } from './pages/ServerHub';
import { Login } from './pages/Login';
import { Setup } from './pages/Setup';
import { KnockPortal } from './pages/KnockPortal';
import { Tasks } from './pages/Tasks';
import { AuditLogs } from './pages/AuditLogs';
import { GlobalPlayers } from './pages/GlobalPlayers';
import { Loader2 } from 'lucide-react';

export const App: React.FC = () => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [needsSetup, setNeedsSetup] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    // If accessing knock portal, skip setup/auth checks
    if (location.pathname.startsWith('/knock/')) {
      setLoading(false);
      return;
    }

    const checkState = async () => {
      try {
        const setupRes = await api.getSetupStatus();
        if (setupRes.needs_setup) {
          setNeedsSetup(true);
          navigate('/setup');
          return;
        }

        const meRes = await api.getMe();
        setUser(meRes.user);
      } catch {
        setUser(null);
      } finally {
        setLoading(false);
      }
    };

    checkState();
  }, [location.pathname]);

  if (loading) {
    return (
      <div className="min-h-screen bg-obsidian-950 flex items-center justify-center text-slate-400">
        <Loader2 className="w-8 h-8 animate-spin text-emerald-400" />
      </div>
    );
  }

  const isKnock = location.pathname.startsWith('/knock/');

  return (
    <div className="min-h-screen bg-obsidian-950 flex flex-col font-sans">
      {!isKnock && <Navbar user={user} onLogout={() => setUser(null)} />}

      <main className="flex-1">
        <Routes>
          {/* Public Knock Portal */}
          <Route path="/knock/:id" element={<KnockPortal />} />

          {/* Setup Wizard */}
          <Route
            path="/setup"
            element={
              needsSetup ? (
                <Setup
                  onSetupSuccess={(u) => {
                    setUser(u);
                    setNeedsSetup(false);
                  }}
                />
              ) : (
                <Navigate to="/" replace />
              )
            }
          />

          {/* Login */}
          <Route
            path="/login"
            element={
              user ? <Navigate to="/" replace /> : <Login onLoginSuccess={(u) => setUser(u)} />
            }
          />

          {/* Protected Routes */}
          <Route
            path="/"
            element={user ? <Dashboard user={user} /> : <Navigate to="/login" replace />}
          />
          <Route
            path="/servers/:id"
            element={user ? <ServerHub user={user} /> : <Navigate to="/login" replace />}
          />
          <Route
            path="/tasks"
            element={user && user.role === 'admin' ? <Tasks /> : <Navigate to="/" replace />}
          />
          <Route
            path="/audit"
            element={user && user.role === 'admin' ? <AuditLogs /> : <Navigate to="/" replace />}
          />
          <Route
            path="/global-players"
            element={user && user.role === 'admin' ? <GlobalPlayers /> : <Navigate to="/" replace />}
          />

          {/* Fallback */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
};
