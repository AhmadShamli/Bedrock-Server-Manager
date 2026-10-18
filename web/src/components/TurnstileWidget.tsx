import { useEffect, useRef, useState, useImperativeHandle, forwardRef } from 'react';
import { ShieldAlert, RefreshCw, Loader2 } from 'lucide-react';

export interface TurnstileWidgetRef {
  reset: () => void;
}

export interface TurnstileWidgetProps {
  siteKey: string;
  action?: string;
  cdata?: string;
  theme?: 'dark' | 'light' | 'auto';
  size?: 'normal' | 'compact' | 'flexible';
  onVerify: (token: string) => void;
  onExpire?: () => void;
  onError?: (errorCode?: string) => void;
  className?: string;
}

declare global {
  interface Window {
    turnstile?: {
      render: (
        container: string | HTMLElement,
        options: {
          sitekey: string;
          action?: string;
          cdata?: string;
          theme?: 'dark' | 'light' | 'auto';
          size?: 'normal' | 'compact' | 'flexible';
          callback?: (token: string) => void;
          'error-callback'?: (errorCode: string) => void;
          'expired-callback'?: () => void;
        }
      ) => string;
      reset: (widgetId: string) => void;
      remove: (widgetId: string) => void;
    };
    onloadTurnstileCallback?: () => void;
  }
}

let scriptLoadPromise: Promise<void> | null = null;

function loadTurnstileScript(): Promise<void> {
  if (typeof window === 'undefined') return Promise.resolve();
  if (window.turnstile) return Promise.resolve();

  if (!scriptLoadPromise) {
    scriptLoadPromise = new Promise<void>((resolve, reject) => {
      const existingScript = document.querySelector('script[src*="challenges.cloudflare.com/turnstile"]');
      if (existingScript && window.turnstile) {
        resolve();
        return;
      }

      window.onloadTurnstileCallback = () => {
        resolve();
      };

      const script = document.createElement('script');
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onloadTurnstileCallback&render=explicit';
      script.async = true;
      script.defer = true;
      script.crossOrigin = 'anonymous';

      const timeoutId = setTimeout(() => {
        reject(new Error('Cloudflare Turnstile script loading timed out. Likely blocked by an adblocker.'));
      }, 7000);

      script.onload = () => {
        clearTimeout(timeoutId);
        // If onloadTurnstileCallback already triggered or turnstile object is immediately available
        if (window.turnstile) {
          resolve();
        }
      };

      script.onerror = () => {
        clearTimeout(timeoutId);
        reject(new Error('Failed to load Turnstile script from Cloudflare.'));
      };

      document.head.appendChild(script);
    });
  }

  return scriptLoadPromise;
}

export const TurnstileWidget = forwardRef<TurnstileWidgetRef, TurnstileWidgetProps>(({
  siteKey,
  action,
  cdata,
  theme = 'dark',
  size = 'normal',
  onVerify,
  onExpire,
  onError,
  className = '',
}, ref) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState<number>(0);

  useImperativeHandle(ref, () => ({
    reset: () => {
      if (widgetIdRef.current && window.turnstile) {
        try {
          window.turnstile.reset(widgetIdRef.current);
        } catch {
          // ignore reset error
        }
      }
    },
  }));

  useEffect(() => {
    let isMounted = true;
    setLoading(true);
    setLoadError(null);

    if (!siteKey || siteKey.trim() === '') {
      setLoadError('Turnstile site key is missing.');
      setLoading(false);
      return;
    }

    loadTurnstileScript()
      .then(() => {
        if (!isMounted || !containerRef.current || !window.turnstile) return;

        // Cleanup existing widget if any
        if (widgetIdRef.current) {
          try {
            window.turnstile.remove(widgetIdRef.current);
          } catch {
            // ignore
          }
          widgetIdRef.current = null;
        }

        try {
          const id = window.turnstile.render(containerRef.current, {
            sitekey: siteKey.trim(),
            action: action || undefined,
            cdata: cdata || undefined,
            theme,
            size,
            callback: (token: string) => {
              if (isMounted) {
                onVerify(token);
              }
            },
            'expired-callback': () => {
              if (isMounted) {
                onExpire?.();
              }
            },
            'error-callback': (errorCode: string) => {
              if (isMounted) {
                onError?.(errorCode);
              }
            },
          });
          widgetIdRef.current = id;
          setLoading(false);
        } catch (err: any) {
          if (isMounted) {
            setLoadError(err.message || 'Failed to render Turnstile widget');
            setLoading(false);
            onError?.(err.message);
          }
        }
      })
      .catch((err: any) => {
        if (isMounted) {
          setLoadError(err.message || 'Cloudflare Turnstile failed to load');
          setLoading(false);
          onError?.(err.message);
        }
      });

    return () => {
      isMounted = false;
      if (widgetIdRef.current && window.turnstile) {
        try {
          window.turnstile.remove(widgetIdRef.current);
        } catch {
          // ignore
        }
        widgetIdRef.current = null;
      }
    };
  }, [siteKey, action, cdata, theme, size, reloadKey]);

  const handleRetry = () => {
    scriptLoadPromise = null;
    setReloadKey((prev) => prev + 1);
  };

  return (
    <div className={`turnstile-container flex flex-col items-center justify-center my-3 ${className}`}>
      {loadError ? (
        <div className="w-full p-3 rounded-xl bg-amber-950/40 border border-amber-500/40 text-amber-300 text-xs flex flex-col gap-2">
          <div className="flex items-start gap-2.5">
            <ShieldAlert className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <span className="font-semibold block text-amber-200">Security Challenge Blocked</span>
              <span className="text-amber-300/90 leading-relaxed block mt-0.5">
                Could not load verification challenge. If you are using an adblocker (e.g. uBlock Origin or Brave Shields), please whitelist or allow Cloudflare Turnstile on this domain.
              </span>
            </div>
          </div>
          <button
            type="button"
            onClick={handleRetry}
            className="self-end inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 font-mono text-[11px] font-semibold border border-amber-500/40 transition-colors"
          >
            <RefreshCw className="w-3 h-3" />
            <span>Retry Challenge</span>
          </button>
        </div>
      ) : (
        <>
          {loading && (
            <div className="flex items-center gap-2 py-3 text-slate-400 text-xs font-mono">
              <Loader2 className="w-4 h-4 animate-spin text-emerald-400" />
              <span>Verifying security challenge...</span>
            </div>
          )}
          <div ref={containerRef} className={loading ? 'hidden' : 'min-h-[65px]'} />
        </>
      )}
    </div>
  );
});

TurnstileWidget.displayName = 'TurnstileWidget';
