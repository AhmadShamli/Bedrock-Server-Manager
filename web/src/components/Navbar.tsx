import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Shield, Server as ServerIcon, LogOut, Terminal, Clock, History, Users, UserCog } from 'lucide-react';
import { api } from '../api/client';
import { User } from '../types';

interface NavbarProps {
  user: User | null;
  onLogout: () => void;
}

export const Navbar: React.FC<NavbarProps> = ({ user, onLogout }) => {
  const navigate = useNavigate();

  const handleLogout = async () => {
    await api.logout();
    onLogout();
    navigate('/login');
  };

  return (
    <header className="bg-obsidian-900 border-b border-obsidian-700/60 sticky top-0 z-40 backdrop-blur-md bg-opacity-95">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <Link to="/" className="flex items-center space-x-3 group">
          <div className="w-9 h-9 rounded-lg bg-emerald-950 border border-emerald-500/40 flex items-center justify-center text-emerald-400 group-hover:border-emerald-400 shadow-[0_0_15px_rgba(16,185,129,0.15)] transition-all">
            <Terminal className="w-5 h-5" />
          </div>
          <div>
            <div className="font-mono font-bold tracking-wider text-slate-100 flex items-center gap-2">
              <span>BEDROCK</span>
              <span className="text-xs px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">BSM</span>
            </div>
            <div className="text-[10px] tracking-widest text-slate-400 uppercase font-mono">Server Manager</div>
          </div>
        </Link>

        {user && (
          <div className="flex items-center space-x-4">
            <Link
              to="/"
              className="px-3 py-1.5 text-sm font-medium text-slate-300 hover:text-emerald-400 flex items-center space-x-1.5 transition-colors"
            >
              <ServerIcon className="w-4 h-4" />
              <span className="hidden sm:inline">Instances</span>
            </Link>

            {user.role === 'admin' && (
              <>
                <Link
                  to="/tasks"
                  className="px-3 py-1.5 text-sm font-medium text-slate-300 hover:text-emerald-400 flex items-center space-x-1.5 transition-colors"
                >
                  <Clock className="w-4 h-4" />
                  <span className="hidden sm:inline">Tasks</span>
                </Link>

                <Link
                  to="/audit"
                  className="px-3 py-1.5 text-sm font-medium text-slate-300 hover:text-emerald-400 flex items-center space-x-1.5 transition-colors"
                >
                  <History className="w-4 h-4" />
                  <span className="hidden sm:inline">Audit</span>
                </Link>

                <Link
                  to="/global-players"
                  className="px-3 py-1.5 text-sm font-medium text-slate-300 hover:text-emerald-400 flex items-center space-x-1.5 transition-colors"
                >
                  <Users className="w-4 h-4" />
                  <span className="hidden sm:inline">Global Players</span>
                </Link>

                <Link
                  to="/users"
                  className="px-3 py-1.5 text-sm font-medium text-slate-300 hover:text-emerald-400 flex items-center space-x-1.5 transition-colors"
                >
                  <UserCog className="w-4 h-4" />
                  <span className="hidden sm:inline">Users</span>
                </Link>
              </>
            )}

            <div className="h-4 w-px bg-obsidian-700"></div>

            <div className="flex items-center space-x-3">
              <div className="flex items-center space-x-2 bg-obsidian-850 border border-obsidian-700 px-3 py-1 rounded-full text-xs">
                <Shield className="w-3.5 h-3.5 text-emerald-400" />
                <span className="text-slate-300 font-mono">{user.username}</span>
                <span className="text-[10px] uppercase font-bold text-emerald-400 bg-emerald-500/10 px-1.5 rounded">
                  {user.role}
                </span>
              </div>

              <button
                onClick={handleLogout}
                title="Log Out"
                className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-obsidian-800 transition-colors"
              >
                <LogOut className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </header>
  );
};
