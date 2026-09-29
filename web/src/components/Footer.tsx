import React, { useEffect, useState } from 'react';
import { ExternalLink, Terminal } from 'lucide-react';
import { api } from '../api/client';

export const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear();
  const [version, setVersion] = useState<string>(
    typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '1.6.1'
  );

  useEffect(() => {
    let isMounted = true;
    api.getVersion()
      .then((data) => {
        if (isMounted && data && data.version) {
          setVersion(data.version);
        }
      })
      .catch(() => {
        // Fall back gracefully to build-time version
      });
    return () => {
      isMounted = false;
    };
  }, []);

  const cleanVersion = version.startsWith('v') ? version.slice(1) : version;

  return (
    <footer className="mt-auto border-t border-obsidian-800/80 bg-obsidian-950/80 backdrop-blur-sm py-4 text-xs font-mono text-slate-500">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-3">
        {/* Left: Branding & Copyright */}
        <div className="flex items-center space-x-2">
          <div className="w-5 h-5 rounded bg-emerald-950 border border-emerald-500/40 flex items-center justify-center text-emerald-400">
            <Terminal className="w-3 h-3" />
          </div>
          <span>
            &copy; {currentYear}{' '}
            <span className="text-slate-300 font-semibold">Bedrock Server Manager</span>
            {' '}by{' '}
            <a
              href="https://github.com/AhmadShamli"
              target="_blank"
              rel="noopener noreferrer"
              className="text-slate-400 hover:text-emerald-400 transition-colors"
            >
              AhmadShamli
            </a>
            . Licensed under MIT.
          </span>
        </div>

        {/* Right: GitHub Project Link & Release Badge */}
        <div className="flex items-center space-x-4">
          <a
            href="https://github.com/AhmadShamli/Bedrock-Server-Manager"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center space-x-1.5 text-slate-400 hover:text-emerald-400 transition-colors group"
          >
            <svg
              className="w-4 h-4 fill-current text-slate-400 group-hover:text-emerald-400 transition-colors"
              viewBox="0 0 24 24"
            >
              <path
                fillRule="evenodd"
                clipRule="evenodd"
                d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
              />
            </svg>
            <span>GitHub Repository</span>
            <ExternalLink className="w-3 h-3 text-slate-500 group-hover:text-emerald-400" />
          </a>

          <a
            href={`https://github.com/AhmadShamli/Bedrock-Server-Manager/releases/tag/v${cleanVersion}`}
            target="_blank"
            rel="noopener noreferrer"
            title={`View release v${cleanVersion} on GitHub`}
            className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[11px] font-bold hover:bg-emerald-500/20 transition-colors"
          >
            v{cleanVersion}
          </a>
        </div>
      </div>
    </footer>
  );
};
