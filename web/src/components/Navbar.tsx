import React, { useState, useRef, useEffect } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { 
  LayoutDashboard, Server as ServerIcon, Layers, Shield, 
  ChevronDown, Users, UserCog, Clock, History, LogOut, Terminal, Menu, X, Sparkles
} from 'lucide-react';
import { api } from '../api/client';
import { User, UserPlanStatus } from '../types';

interface NavbarProps {
  user: User | null;
  onLogout: () => void;
}

export const Navbar: React.FC<NavbarProps> = ({ user, onLogout }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const [adminMenuOpen, setAdminMenuOpen] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [userPlan, setUserPlan] = useState<UserPlanStatus | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const handleLogout = async () => {
    await api.logout();
    onLogout();
    navigate('/login');
  };

  // Close dropdown on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setAdminMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Fetch plan status for normal users
  useEffect(() => {
    if (user && user.role === 'user') {
      api.getMyPlan().then(setUserPlan).catch(() => {});
    }
  }, [user]);

  // Close menus on route change
  useEffect(() => {
    setAdminMenuOpen(false);
    setMobileMenuOpen(false);
  }, [location.pathname]);

  const isAdminSectionActive = [
    '/users',
    '/global-players',
    '/portgate',
    '/tasks',
    '/audit',
  ].some((path) => location.pathname.startsWith(path));

  return (
    <header className="bg-obsidian-900 border-b border-obsidian-700/60 sticky top-0 z-40 backdrop-blur-md bg-opacity-95">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        {/* Brand */}
        <Link to="/" className="flex items-center space-x-3 group shrink-0">
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
          <>
            {/* Desktop Navigation */}
            <nav className="hidden md:flex items-center space-x-1 lg:space-x-2">
              <Link
                to="/"
                className={`px-3 py-1.5 text-sm font-medium flex items-center space-x-1.5 transition-colors rounded-lg ${
                  location.pathname === '/'
                    ? 'text-emerald-400 bg-obsidian-850 border border-obsidian-750'
                    : 'text-slate-300 hover:text-emerald-400 hover:bg-obsidian-850/50'
                }`}
              >
                <LayoutDashboard className="w-4 h-4" />
                <span>Dashboard</span>
              </Link>

              <Link
                to="/servers"
                className={`px-3 py-1.5 text-sm font-medium flex items-center space-x-1.5 transition-colors rounded-lg ${
                  location.pathname.startsWith('/servers')
                    ? 'text-emerald-400 bg-obsidian-850 border border-obsidian-750'
                    : 'text-slate-300 hover:text-emerald-400 hover:bg-obsidian-850/50'
                }`}
              >
                <ServerIcon className="w-4 h-4" />
                <span>{user.role === 'user' ? 'My Servers' : 'Servers'}</span>
              </Link>

              {user.role === 'admin' && (
                <Link
                  to="/plans"
                  className={`px-3 py-1.5 text-sm font-medium flex items-center space-x-1.5 transition-colors rounded-lg ${
                    location.pathname.startsWith('/plans')
                      ? 'text-emerald-400 bg-obsidian-850 border border-obsidian-750'
                      : 'text-slate-300 hover:text-emerald-400 hover:bg-obsidian-850/50'
                  }`}
                >
                  <Layers className="w-4 h-4" />
                  <span>Plans</span>
                </Link>
              )}

              {/* Admin Management Dropdown */}
              {user.role === 'admin' && (
                <div className="relative" ref={dropdownRef}>
                  <button
                    onClick={() => setAdminMenuOpen(!adminMenuOpen)}
                    className={`px-3 py-1.5 text-sm font-medium flex items-center space-x-1.5 transition-colors rounded-lg ${
                      isAdminSectionActive
                        ? 'text-emerald-400 bg-obsidian-850 border border-obsidian-750'
                        : 'text-slate-300 hover:text-emerald-400 hover:bg-obsidian-850/50'
                    }`}
                  >
                    <Shield className="w-4 h-4" />
                    <span>Management</span>
                    <ChevronDown className={`w-3.5 h-3.5 transition-transform ${adminMenuOpen ? 'rotate-180 text-emerald-400' : ''}`} />
                  </button>

                  {adminMenuOpen && (
                    <div className="absolute right-0 mt-2 w-56 bg-obsidian-900 border border-obsidian-700/80 rounded-xl shadow-2xl p-1.5 space-y-1 z-50 backdrop-blur-xl animate-in fade-in slide-in-from-top-2 duration-150">
                      <Link
                        to="/users"
                        className="flex items-center space-x-2.5 px-3 py-2 text-xs font-mono text-slate-300 hover:text-emerald-400 hover:bg-obsidian-800 rounded-lg transition-colors"
                      >
                        <UserCog className="w-4 h-4 text-emerald-400" />
                        <div>
                          <div className="font-bold">Users & Access</div>
                          <div className="text-[10px] text-slate-500">Accounts & permissions</div>
                        </div>
                      </Link>

                      <Link
                        to="/global-players"
                        className="flex items-center space-x-2.5 px-3 py-2 text-xs font-mono text-slate-300 hover:text-emerald-400 hover:bg-obsidian-800 rounded-lg transition-colors"
                      >
                        <Users className="w-4 h-4 text-cyan-400" />
                        <div>
                          <div className="font-bold">Global Players</div>
                          <div className="text-[10px] text-slate-500">Cross-server allowlist & bans</div>
                        </div>
                      </Link>

                      <Link
                        to="/portgate"
                        className="flex items-center space-x-2.5 px-3 py-2 text-xs font-mono text-slate-300 hover:text-emerald-400 hover:bg-obsidian-800 rounded-lg transition-colors"
                      >
                        <Shield className="w-4 h-4 text-amber-400" />
                        <div>
                          <div className="font-bold">Port Gate Firewall</div>
                          <div className="text-[10px] text-slate-500">Knock portal & dynamic leases</div>
                        </div>
                      </Link>

                      <Link
                        to="/tasks"
                        className="flex items-center space-x-2.5 px-3 py-2 text-xs font-mono text-slate-300 hover:text-emerald-400 hover:bg-obsidian-800 rounded-lg transition-colors"
                      >
                        <Clock className="w-4 h-4 text-indigo-400" />
                        <div>
                          <div className="font-bold">Automated Tasks</div>
                          <div className="text-[10px] text-slate-500">Cron backups & restarts</div>
                        </div>
                      </Link>

                      <div className="border-t border-obsidian-800 my-1"></div>

                      <Link
                        to="/audit"
                        className="flex items-center space-x-2.5 px-3 py-2 text-xs font-mono text-slate-300 hover:text-emerald-400 hover:bg-obsidian-800 rounded-lg transition-colors"
                      >
                        <History className="w-4 h-4 text-purple-400" />
                        <div>
                          <div className="font-bold">System Audit Logs</div>
                          <div className="text-[10px] text-slate-500">Security & admin activity</div>
                        </div>
                      </Link>
                    </div>
                  )}
                </div>
              )}
            </nav>

            {/* Profile & Session */}
            <div className="hidden md:flex items-center space-x-3">
              <div className="flex items-center space-x-2 bg-obsidian-850 border border-obsidian-750 px-3 py-1.5 rounded-full text-xs">
                {user.role === 'admin' ? (
                  <Shield className="w-3.5 h-3.5 text-purple-400" />
                ) : user.role === 'operator' ? (
                  <Shield className="w-3.5 h-3.5 text-cyan-400" />
                ) : (
                  <Sparkles className="w-3.5 h-3.5 text-emerald-400" />
                )}
                <span className="text-slate-200 font-mono font-medium">{user.username}</span>

                <span
                  className={`text-[10px] uppercase font-bold px-1.5 py-0.5 rounded tracking-wider ${
                    user.role === 'admin'
                      ? 'bg-purple-500/15 text-purple-400 border border-purple-500/30'
                      : user.role === 'operator'
                      ? 'bg-cyan-500/15 text-cyan-400 border border-cyan-500/30'
                      : 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                  }`}
                >
                  {user.role}
                </span>

                {user.role === 'user' && userPlan && (
                  <span className="text-[10px] text-slate-400 border-l border-obsidian-700 pl-1.5 font-mono">
                    {userPlan.plan.name} ({userPlan.usage.servers_count}/{userPlan.usage.servers_max})
                  </span>
                )}
              </div>

              <button
                onClick={handleLogout}
                title="Log Out"
                className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-obsidian-800 transition-colors"
              >
                <LogOut className="w-4 h-4" />
              </button>
            </div>

            {/* Mobile Hamburger Button */}
            <div className="flex items-center md:hidden">
              <button
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                className="p-2 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-obsidian-800"
              >
                {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
              </button>
            </div>
          </>
        )}
      </div>

      {/* Mobile Drawer */}
      {user && mobileMenuOpen && (
        <div className="md:hidden border-t border-obsidian-800 bg-obsidian-900 px-4 pt-3 pb-6 space-y-3 font-mono text-sm">
          {/* User profile info */}
          <div className="flex items-center justify-between pb-3 border-b border-obsidian-800">
            <div>
              <span className="text-slate-200 font-bold block">{user.username}</span>
              <span className="text-xs text-emerald-400 uppercase font-semibold">
                Role: {user.role} {user.role === 'user' && userPlan ? `• ${userPlan.plan.name}` : ''}
              </span>
            </div>
            <button
              onClick={handleLogout}
              className="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 border border-rose-800/60 px-2.5 py-1 rounded-lg"
            >
              <LogOut className="w-3.5 h-3.5" />
              <span>Log Out</span>
            </button>
          </div>

          <div className="space-y-1">
            <Link
              to="/"
              className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                location.pathname === '/' ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
              }`}
            >
              <LayoutDashboard className="w-4 h-4" />
              <span>Dashboard</span>
            </Link>

            <Link
              to="/servers"
              className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                location.pathname.startsWith('/servers') ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
              }`}
            >
              <ServerIcon className="w-4 h-4" />
              <span>{user.role === 'user' ? 'My Servers' : 'Servers'}</span>
            </Link>

            {user.role === 'admin' && (
              <>
                <Link
                  to="/plans"
                  className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                    location.pathname.startsWith('/plans') ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
                  }`}
                >
                  <Layers className="w-4 h-4" />
                  <span>Deployment Plans</span>
                </Link>

                <div className="pt-2 text-[10px] text-slate-500 uppercase tracking-widest px-3">Administration</div>

                <Link
                  to="/users"
                  className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                    location.pathname.startsWith('/users') ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
                  }`}
                >
                  <UserCog className="w-4 h-4 text-emerald-400" />
                  <span>Users & Access</span>
                </Link>

                <Link
                  to="/global-players"
                  className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                    location.pathname.startsWith('/global-players') ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
                  }`}
                >
                  <Users className="w-4 h-4 text-cyan-400" />
                  <span>Global Players</span>
                </Link>

                <Link
                  to="/portgate"
                  className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                    location.pathname.startsWith('/portgate') ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
                  }`}
                >
                  <Shield className="w-4 h-4 text-amber-400" />
                  <span>Port Gate Firewall</span>
                </Link>

                <Link
                  to="/tasks"
                  className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                    location.pathname.startsWith('/tasks') ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
                  }`}
                >
                  <Clock className="w-4 h-4 text-indigo-400" />
                  <span>Automated Tasks</span>
                </Link>

                <Link
                  to="/audit"
                  className={`flex items-center space-x-2 px-3 py-2 rounded-lg ${
                    location.pathname.startsWith('/audit') ? 'text-emerald-400 bg-obsidian-850' : 'text-slate-300'
                  }`}
                >
                  <History className="w-4 h-4 text-purple-400" />
                  <span>System Audit</span>
                </Link>
              </>
            )}
          </div>
        </div>
      )}
    </header>
  );
};
